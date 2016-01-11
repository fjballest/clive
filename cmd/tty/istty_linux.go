// +build linux

package tty

import (
	"os"
	"syscall"
	"unsafe"
)

const ioctlReadTermios = syscall.TCGETS

// Return true if f refers to a tty
func IsTTY(f *os.File) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, f.Fd(),
		uintptr(ioctlReadTermios),
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0)
	return err == 0
}
