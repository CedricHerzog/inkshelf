package pocketbook

import "encoding/binary"

// The layouts of the messages on the firmware's two queues, and how to encode and decode them.
// They are C structs of int32s (a long is an int32 too on 32-bit ARM), little-endian as ARM
// is. Encoding them by hand, rather than through unsafe casts of Go structs, keeps the layout
// visible and lets the tests and the emulator build the very bytes the reader sends.

// Event types, as numbered by the firmware (EVT_* in the 6.5 SDK's inkview.h; older headers
// number some differently). Input events come on /hwevent, the others as task events.
const (
	eventExit            = 22
	eventKeyPress        = 25
	eventKeyRelease      = 26
	eventKeyRepeat       = 28
	eventPointerUp       = 29
	eventPointerDown     = 30
	eventPointerMove     = 31
	eventFSChanged       = 73
	eventForeground      = 151
	eventBackground      = 152
	eventSaveState       = 155
	eventUSBStoreIn      = 163
	eventUSBStoreOut     = 164
	eventNetConnected    = 256
	eventNetDisconnected = 257
)

// An input message on the firmware's /hwevent queue is eight int32s: {type, modifier or touch
// id, key code or raw x, raw y, pressure, tool, tv_sec, tv_usec}. The app uses the first four.
const (
	hardwareEventSize = 32

	hardwareEventKindOffset    = 0
	hardwareEventTouchIDOffset = 4
	hardwareEventKeyOrXOffset  = 8
	hardwareEventYOffset       = 12

	// fullPressure and fingerTool are what encodeHardwareEvent writes for a touch.
	fullPressure = 255
	fingerTool   = 1
)

// hardwareEvent is a message from /hwevent.
type hardwareEvent struct {
	// kind is the event type: eventKeyPress, eventPointerDown, …
	kind int32
	// touchID is the finger of a touch: 0 for the first one.
	touchID int32
	// keyOrRawX is the key code of a button, or the raw x of a touch.
	keyOrRawX int32
	// rawY is the raw y of a touch.
	rawY int32
}

// decodeHardwareEvent decodes a message from /hwevent.
func decodeHardwareEvent(message []byte) hardwareEvent {
	return hardwareEvent{
		kind:      int32At(message, hardwareEventKindOffset),
		touchID:   int32At(message, hardwareEventTouchIDOffset),
		keyOrRawX: int32At(message, hardwareEventKeyOrXOffset),
		rawY:      int32At(message, hardwareEventYOffset),
	}
}

// encodeHardwareEvent returns the /hwevent message that decodeHardwareEvent decodes into
// event, with full pressure and a finger as the tool.
func encodeHardwareEvent(event hardwareEvent) []byte {
	message := make([]byte, hardwareEventSize)
	putInt32s(message, event.kind, event.touchID, event.keyOrRawX, event.rawY, fullPressure, fingerTool)
	return message
}

// A task message, on the firmware's System V queue, is {long mtype, int32 id, type, argument,
// length, data}. Every app shares the queue: msgrcv picks the messages of one mtype, so the
// mtype says who a message is for (see firmwareMtype and taskEventMtype).
const (
	mtypeOffset = 0
	mtypeSize   = 4

	taskIDOffset       = 4
	taskKindOffset     = 8
	taskArgumentOffset = 12
	taskLengthOffset   = 16
	// taskMessageHeader is the size of the header, mtype included: the data starts there.
	taskMessageHeader = 20
	// taskSendSize is the size that libinkview gives msgsnd, which doesn't count the mtype:
	// the rest of the header and one empty data word.
	taskSendSize = taskMessageHeader
	// maxTaskData is the most data a received task message may hold.
	maxTaskData = 0x10000
)

// Task message types.
const (
	// taskAsyncEvent is a task event: its data is eventDataSize bytes, and it needs no reply.
	taskAsyncEvent = 0x50d
	// taskSyncEvent is the same as taskAsyncEvent, but the sender waits for a reply.
	taskSyncEvent = 0x513
	// taskBanSuspend is a request whose argument is the number of seconds the reader must not
	// suspend (libinkview's BanSleep).
	taskBanSuspend = 0x123
)

// The mtypes of task messages, which tell who they are for.
const (
	// firmwareMtype marks requests to the firmware. It replies with the request's id as mtype.
	firmwareMtype = 0x7fffffff
	// taskEventMtype, combined with a process group, marks the messages to that task.
	taskEventMtype = 0x7a000000
)

// The data of a task event is four int32s: {event, 0, code, value}.
const (
	eventDataSize     = 16
	eventNumberOffset = 0
	eventCodeOffset   = 8
	eventValueOffset  = 12
)

// A request id holds the pid in bits 12 to 27 and a serial number in bits 0 to 11.
const (
	requestPIDShift   = 12
	requestPIDMask    = 0x0ffff000
	requestSerialMask = 0xfff
)

// taskMessage is a parsed task message.
type taskMessage struct {
	id       int32
	kind     int32
	argument int32
	// event, eventCode and eventValue are set for task events only. The meaning of the code
	// and the value depends on the event: the foreground event's code is a process group.
	event      int32
	eventCode  int32
	eventValue int32
}

// parseTaskMessage parses a received message, mtype included. It reports false if the
// message is too short to hold a header.
func parseTaskMessage(message []byte) (taskMessage, bool) {
	if len(message) < taskMessageHeader {
		return taskMessage{}, false
	}
	parsed := taskMessage{
		id:       int32At(message, taskIDOffset),
		kind:     int32At(message, taskKindOffset),
		argument: int32At(message, taskArgumentOffset),
	}
	isEvent := parsed.kind == taskAsyncEvent || parsed.kind == taskSyncEvent
	if isEvent && len(message) >= taskMessageHeader+eventDataSize {
		parsed.event = int32At(message, taskMessageHeader+eventNumberOffset)
		parsed.eventCode = int32At(message, taskMessageHeader+eventCodeOffset)
		parsed.eventValue = int32At(message, taskMessageHeader+eventValueOffset)
	}
	return parsed, true
}

// taskEvent returns a task event as the firmware sends it to the task with process group
// pgid: {mtype, id, taskAsyncEvent, 0, eventDataSize, {event, 0, code, value}}.
func taskEvent(pgid, id, event, code, value int32) []byte {
	message := make([]byte, taskMessageHeader+eventDataSize)
	putInt32s(message, taskEventMtype|pgid, id, taskAsyncEvent, 0, eventDataSize, event, 0, code, value)
	return message
}

// libinkview answers every sync event with hw_ipcreply(id, 0, 1, NULL, 0): a reply of kind 0
// and argument 1, without data. The firmware only waits for a reply; what the two numbers mean
// to it is unknown.
const (
	syncReplyKind     = 0
	syncReplyArgument = 1
)

// taskReply returns the reply to the sync event id, as libinkview sends it:
// {mtype id, id, syncReplyKind, syncReplyArgument, length 0}.
func taskReply(id int32) []byte {
	reply := make([]byte, mtypeSize+taskSendSize)
	putInt32s(reply, id, id, syncReplyKind, syncReplyArgument)
	return reply
}

// requestID numbers a request as libinkview's hw_ipcrequest does. The reply comes back with
// it as its mtype.
func requestID(pid, serial int) int32 {
	return int32(pid<<requestPIDShift&requestPIDMask | serial&requestSerialMask)
}

// taskRequest returns a request to the firmware without data, as hw_ipcrequest(kind,
// argument, NULL, 0) sends it: {mtype firmwareMtype, id, kind, argument, length 0}.
func taskRequest(id, kind, argument int32) []byte {
	request := make([]byte, mtypeSize+taskSendSize)
	putInt32s(request, firmwareMtype, id, kind, argument)
	return request
}

// int32At reads the little-endian int32 at offset in buffer.
func int32At(buffer []byte, offset int) int32 {
	return int32(binary.LittleEndian.Uint32(buffer[offset:]))
}

// putInt32s writes values as consecutive little-endian int32s at the start of buffer.
func putInt32s(buffer []byte, values ...int32) {
	for index, value := range values {
		binary.LittleEndian.PutUint32(buffer[index*4:], uint32(value))
	}
}
