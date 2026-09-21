//go:build darwin || linux

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminalFile reports whether f is a real terminal. A character-device check
// alone is not enough: /dev/null is a character device too, so a process
// started with its stdin closed would look interactive. A terminal is the thing
// that answers the termios ioctl.
func isTerminalFile(f *os.File) bool {
	var termios [256]byte
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		f.Fd(),
		uintptr(ioctlReadTermios),
		uintptr(unsafe.Pointer(&termios[0])),
		0, 0, 0,
	)
	return errno == 0
}
