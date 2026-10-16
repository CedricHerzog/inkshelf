//go:build linux && arm

package pocketbook

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// System calls on 32-bit ARM (EABI) that package syscall lacks.
const (
	sysMqOpen         = 274
	sysMqTimedSend    = 276
	sysMqTimedReceive = 277
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

// mqTimedReceive receives a message of the POSIX queue fd into buffer, waiting until deadline
// if the queue is empty and fd blocks. It returns the message's length. Signals, which the Go
// runtime sends its own threads, interrupt the wait: it then goes on until the same deadline.
func mqTimedReceive(fd int, buffer []byte, deadline time.Time) (int, syscall.Errno) {
	for {
		timeout := syscall.NsecToTimespec(deadline.UnixNano())
		count, _, errno := syscall.Syscall6(sysMqTimedReceive, uintptr(fd), uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)), 0, uintptr(unsafe.Pointer(&timeout)), 0)
		if errno != syscall.EINTR {
			return int(count), errno
		}
	}
}

// mqSend sends message to the POSIX queue fd, which must not block.
func mqSend(fd int, message []byte) syscall.Errno {
	_, _, errno := syscall.Syscall6(sysMqTimedSend, uintptr(fd), uintptr(unsafe.Pointer(&message[0])),
		uintptr(len(message)), 0, 0, 0)
	return errno
}
