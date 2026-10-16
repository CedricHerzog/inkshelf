//go:build linux && arm

package pocketbook

import (
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// readerFirmware is the firmware of a PocketBook running version 6.x, reached through its
// framebuffer, its message queues and its shared memory.
type readerFirmware struct {
	// framebufferFD is the open framebufferPath.
	framebufferFD int
	// framebuffer is the framebuffer's memory, mapped.
	framebuffer       []byte
	framebufferHeight int
	framebufferStride int
	// marker numbers the refreshes: it is the marker of the last refresh sent.
	marker uint32
	// running are the last refreshes sent, which may still be running, oldest first.
	running []runningRefresh
	// global is the firmware's shared state, or nil if it is missing: the app is then always
	// visible.
	global []byte
	// pgid is this process's group. The firmware knows tasks by process group.
	pgid int
	// taskQueue is the id of the task message queue, or -1 if it is missing.
	taskQueue int
	// events is the file descriptor of the /hwevent queue, or -1 if it is missing.
	events int
	// taskBuffer is where task messages are received: an mtype, then up to maxTaskData bytes.
	taskBuffer []byte
	// eventBuffer is where /hwevent messages are received.
	eventBuffer [hardwareEventSize]byte
	// darkStatus is what darkMode last logged, to log it only when it changes.
	darkStatus string
}

// openFirmware opens the framebuffer, the firmware's queues and shared state, and the canvas.
func openFirmware() (firmware, *image.Gray, error) {
	reader := &readerFirmware{
		pgid:       syscall.Getpgrp(),
		taskQueue:  -1,
		events:     -1,
		taskBuffer: make([]byte, mtypeSize+maxTaskData),
	}
	framebufferWidth, err := reader.openFramebuffer()
	if err != nil {
		return nil, nil, fmt.Errorf("framebuffer: %w", err)
	}
	// Without the firmware's state or queues the app still runs, without input or task events.
	if reader.global, err = attachShm(globalShmKey, globalShmSize, true); err != nil {
		log.Printf("firmware state: %v", err)
	}
	if reader.taskQueue, err = msgget(taskQueueKey); err != nil {
		log.Printf("task queue: %v", err)
	}
	if reader.events, err = mqOpen(eventQueue, syscall.O_RDONLY|syscall.O_NONBLOCK); err != nil {
		log.Printf("event queue: %v", err)
	}
	// The screen is portrait and the framebuffer landscape: the framebuffer's rows are the
	// screen's columns.
	size := image.Pt(reader.framebufferHeight, framebufferWidth)
	canvas, err := taskCanvas(reader.pgid, size)
	if err != nil {
		log.Printf("task canvas: %v; drawing into memory", err)
		canvas = image.NewGray(image.Rectangle{Max: size})
	}
	return reader, canvas, nil
}

// The firmware describes every task it launches in a directory named after the task's process
// group. Two of its files, each holding a decimal number, locate the task's canvas.
const (
	// taskDirFormat is the task's directory, given its process group.
	taskDirFormat = "/var/run/task/%d"
	// canvasKeyFile holds the System V key of the canvas's shared memory.
	canvasKeyFile = "fbshmkey"
	// canvasSizeFile holds the size of that shared memory, in bytes.
	canvasSizeFile = "fbshmsize"
)

// taskCanvas returns the canvas that the firmware set up for this task, which its task
// switcher shows as the app's thumbnail.
func taskCanvas(pgid int, size image.Point) (*image.Gray, error) {
	dir := fmt.Sprintf(taskDirFormat, pgid)
	key, err := readInt(filepath.Join(dir, canvasKeyFile))
	if err != nil {
		return nil, err
	}
	shmSize, err := readInt(filepath.Join(dir, canvasSizeFile))
	if err != nil {
		return nil, err
	}
	shm, err := attachShm(key, shmSize, false)
	if err != nil {
		return nil, err
	}
	width, height := int(int32At(shm, canvasWidth)), int(int32At(shm, canvasHeight))
	depth, scanline := int32At(shm, canvasDepth), int(int32At(shm, canvasScanline))
	start := canvasHeader + int(int32At(shm, canvasDataOffset))
	if width != size.X || height != size.Y || depth != 8 || scanline < width || start < canvasHeader || start+scanline*height > len(shm) {
		return nil, fmt.Errorf("%dx%d at %d bpp, scanline %d, data at %d of %d bytes: not a %v canvas",
			width, height, depth, scanline, start, len(shm), size)
	}
	return &image.Gray{Pix: shm[start : start+scanline*height], Stride: scanline, Rect: image.Rectangle{Max: size}}, nil
}

// visible reports whether this task is in the foreground and refreshes are not suppressed.
// Without task switching, the only app is always in the foreground.
func (reader *readerFirmware) visible() bool {
	if reader.global == nil {
		return true
	}
	if reader.global[globalSuppressed] != 0 {
		return false
	}
	return int32At(reader.global, globalMultitask) != multitaskMagic ||
		int(int32At(reader.global, globalForeground)) == reader.pgid
}

// describe returns the firmware state that visible depends on, for the log.
func (reader *readerFirmware) describe() string {
	if reader.global == nil {
		return "no firmware state"
	}
	return fmt.Sprintf("foreground %d, us %d, suppressed %d, multitask %#x",
		int32At(reader.global, globalForeground), reader.pgid,
		reader.global[globalSuppressed], int32At(reader.global, globalMultitask))
}

// nextHardwareEvent returns the next /hwevent message, if any.
func (reader *readerFirmware) nextHardwareEvent() ([]byte, bool) {
	if reader.events < 0 {
		return nil, false
	}
	buffer := reader.eventBuffer[:]
	count, _, errno := syscall.Syscall6(sysMqTimedReceive, uintptr(reader.events), uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)), 0, 0, 0)
	if errno != 0 {
		// EAGAIN means the queue is empty.
		if errno != syscall.EAGAIN {
			log.Printf("event queue: %v", errno)
		}
		return nil, false
	}
	if int(count) < hardwareEventSize {
		return nil, false
	}
	return buffer, true
}

// nextTaskMessage returns the next message for this task, if any.
func (reader *readerFirmware) nextTaskMessage() ([]byte, bool) {
	return reader.receive(taskEventMtype | reader.pgid)
}

// nextReply returns the firmware's reply to the request with this id, if it came.
func (reader *readerFirmware) nextReply(id int32) ([]byte, bool) {
	return reader.receive(int(id))
}

// receive returns the next task message with this mtype, if any, without waiting. The message
// is in a buffer that the next call reuses.
func (reader *readerFirmware) receive(mtype int) ([]byte, bool) {
	if reader.taskQueue < 0 {
		return nil, false
	}
	buffer := reader.taskBuffer
	// msgrcv's size is that of the data after the mtype, and it returns the size received.
	count, _, errno := syscall.Syscall6(sysMsgRcv, uintptr(reader.taskQueue), uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)-mtypeSize), uintptr(mtype), ipcNoWait, 0)
	if errno != 0 {
		// ENOMSG means there is no such message.
		if errno != syscall.ENOMSG {
			log.Printf("task queue: %v", errno)
		}
		return nil, false
	}
	return buffer[:mtypeSize+int(count)], true
}

// send sends a task message with no data, and reports whether it was sent.
func (reader *readerFirmware) send(message []byte) bool {
	if reader.taskQueue < 0 {
		return false
	}
	_, _, errno := syscall.Syscall6(sysMsgSnd, uintptr(reader.taskQueue), uintptr(unsafe.Pointer(&message[0])),
		taskSendSize, ipcNoWait, 0, 0)
	if errno != 0 {
		log.Printf("task message: %v", errno)
		return false
	}
	return true
}

// battery returns the battery's charge and whether it is charging, from the firmware's state.
func (reader *readerFirmware) battery() (percent int, charging, ok bool) {
	if reader.global == nil {
		return 0, false, false
	}
	percent, charging = batteryState(reader.global)
	return percent, charging, true
}

// firmwareSettingsPath is the firmware's main settings file (see firmware_settings.go).
const firmwareSettingsPath = "/mnt/ext1/system/config/global.cfg"

// darkMode reads the firmware's settings and reports whether they turn dark mode on. ok is
// false when they cannot be read or lack the setting: an older firmware, or a file read while
// the firmware rewrites it. It logs what it found when that changes.
func (reader *readerFirmware) darkMode() (dark, ok bool) {
	data, err := os.ReadFile(firmwareSettingsPath)
	if err == nil {
		dark, ok = darkModeOf(parseSettings(data))
	}
	status := fmt.Sprintf("dark mode %v", dark)
	switch {
	case err != nil:
		status = fmt.Sprintf("dark mode unknown: %v", err)
	case !ok:
		status = fmt.Sprintf("dark mode unknown: no %s in %s", darkModeSetting, firmwareSettingsPath)
	}
	if status != reader.darkStatus {
		reader.darkStatus = status
		log.Print(status)
	}
	return dark, ok
}

// readInt reads a file that holds a decimal number.
func readInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	return value, nil
}
