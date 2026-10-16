//go:build linux && arm

package pocketbook

import (
	"fmt"
	"syscall"
	"unsafe"
)

// System calls on 32-bit ARM (EABI) that package syscall lacks.
const (
	sysMqOpen         = 274
	sysMqTimedReceive = 277
	sysMqNotify       = 278
	sysMsgSnd         = 301
	sysMsgRcv         = 302
	sysMsgGet         = 303
	sysShmAt          = 305
	sysShmGet         = 307
)

// Flags of the SysV IPC calls.
const (
	// ipcNoWait (IPC_NOWAIT) makes msgsnd and msgrcv fail instead of waiting.
	ipcNoWait = 0o4000
	// shmReadOnly (SHM_RDONLY) attaches shared memory read-only.
	shmReadOnly = 0o10000
)

// ioctl runs an ioctl request on the file descriptor fd, with arg pointing to its data.
func ioctl(fd int, request uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// attachShm attaches the SysV shared memory with this key and size, and returns its bytes.
func attachShm(key, size int, readOnly bool) ([]byte, error) {
	id, _, errno := syscall.Syscall(sysShmGet, uintptr(key), uintptr(size), 0)
	if errno != 0 {
		return nil, fmt.Errorf("shmget %#x: %w", key, errno)
	}
	flags := uintptr(0)
	if readOnly {
		flags = shmReadOnly
	}
	address, _, errno := syscall.Syscall(sysShmAt, id, 0, flags)
	if errno != 0 {
		return nil, fmt.Errorf("shmat %#x: %w", key, errno)
	}
	// The segment stays attached for the life of the process, outside Go's heap. Going
	// through a pointer to address converts it without go vet's uintptr warning.
	pointer := *(*unsafe.Pointer)(unsafe.Pointer(&address))
	return unsafe.Slice((*byte)(pointer), size), nil
}

// msgget returns the id of the SysV message queue with this key.
func msgget(key int) (int, error) {
	id, _, errno := syscall.Syscall(sysMsgGet, uintptr(key), 0, 0)
	if errno != 0 {
		return -1, fmt.Errorf("msgget %#x: %w", key, errno)
	}
	return int(id), nil
}

// mqOpen opens the POSIX message queue name with flags, and returns its file descriptor.
func mqOpen(name string, flags int) (int, error) {
	namePointer, err := syscall.BytePtrFromString(name)
	if err != nil {
		return -1, err
	}
	fd, _, errno := syscall.Syscall(sysMqOpen, uintptr(unsafe.Pointer(namePointer)), uintptr(flags), 0)
	if errno != 0 {
		return -1, fmt.Errorf("mq_open %s: %w", name, errno)
	}
	return int(fd), nil
}

// A struct sigevent, which tells mq_notify how to notify: sigevSize bytes, of which the signal
// number and the way to notify are the second and third int32s. sigevSignal notifies with a
// signal.
const (
	sigevSize   = 64
	sigevSignal = 0
)

// mqNotify asks the kernel to send this process signal once a message arrives on the empty
// POSIX queue fd. The request lasts for one notification. It fails with EBUSY if a process
// already asked.
func mqNotify(fd int, signal syscall.Signal) error {
	var event [sigevSize]byte
	putInt32s(event[:], 0, int32(signal), sigevSignal)
	_, _, errno := syscall.Syscall(sysMqNotify, uintptr(fd), uintptr(unsafe.Pointer(&event[0])), 0)
	if errno != 0 {
		return fmt.Errorf("mq_notify: %w", errno)
	}
	return nil
}
