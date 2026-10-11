package pocketbook

import (
	"slices"
	"testing"
)

func TestParseTaskMessage(t *testing.T) {
	// A foreground event received on an InkPad 3: {mtype, id, 0x50d, 0, 16, {151, 0, pgid, 0}}.
	const pgid = 10946
	mtype := int32(taskEventMtype | pgid)
	message := int32s(mtype, mtype, taskAsyncEvent, 0, eventDataSize, eventForeground, 0, pgid, 0)
	parsed, ok := parseTaskMessage(message)
	if !ok || parsed.kind != taskAsyncEvent || parsed.event != eventForeground || parsed.eventCode != pgid {
		t.Errorf("parseTaskMessage = %+v, %v", parsed, ok)
	}
	if _, ok := parseTaskMessage(message[:taskArgumentOffset]); ok {
		t.Error("parseTaskMessage accepted a truncated message")
	}
}

func TestTaskReply(t *testing.T) {
	const id = 0x1234005
	reply := taskReply(id)
	if got := int32At(reply, mtypeOffset); got != id {
		t.Errorf("reply mtype = %#x, want the request id", got)
	}
	if got := int32At(reply, taskArgumentOffset); got != 1 {
		t.Errorf("reply argument = %d, want 1", got)
	}
}

// Requests are laid out as libinkview's hw_ipcrequest sends them (libinkview's BanSleep, as
// PreventSleepFor, sends hw_ipcrequest(0x123, seconds, NULL, 0)).
func TestTaskRequest(t *testing.T) {
	id := requestID(0x12345, 0x1001)
	if id != 0x2345001 {
		t.Errorf("requestID = %#x, want the pid's low 16 bits, then the serial's low 12", id)
	}
	request := taskRequest(id, taskBanSuspend, 60)
	if want := int32s(firmwareMtype, id, taskBanSuspend, 60, 0); !slices.Equal(request[:len(want)], want) {
		t.Errorf("request = %x, want %x", request, want)
	}
}

func TestHardwareEventRoundTrip(t *testing.T) {
	event := hardwareEvent{kind: eventPointerDown, touchID: 1, keyOrRawX: 1760, rawY: 97}
	if got := decodeHardwareEvent(encodeHardwareEvent(event)); got != event {
		t.Errorf("decodeHardwareEvent(encodeHardwareEvent(%+v)) = %+v", event, got)
	}
}
