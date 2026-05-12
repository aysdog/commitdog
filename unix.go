//go:build !windows
// +build !windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

type termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Cc     [20]uint8
	Ispeed uint32
	Ospeed uint32
}

var origTermios termios

func enableRawMode() {
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stdin.Fd()), 0x5401, uintptr(unsafe.Pointer(&origTermios)))
	raw := origTermios
	raw.Lflag &^= syscall.ECHO | syscall.ICANON
	raw.Cc[6] = 1
	raw.Cc[5] = 0
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stdin.Fd()), 0x5402, uintptr(unsafe.Pointer(&raw)))
}

func disableRawMode() {
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stdin.Fd()), 0x5402, uintptr(unsafe.Pointer(&origTermios)))
}

func terminalHeight() int {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	ws := &winsize{}
	ret, _, _ := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(int(os.Stdout.Fd())),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if int(ret) == -1 || ws.Row == 0 {
		return 24
	}
	return int(ws.Row)
}
