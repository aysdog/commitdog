//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var asciiLines = []string{
	` 0000    0000   00   00  00   00  0000 000000  00000    0000    00000`,
	`00      00  00  0000000  0000000   00    00    00  00  00  00  00    `,
	`00      00  00  00 0 00  00 0 00   00    00    00  00  00  00  00 000`,
	`00      00  00  00   00  00   00   00    00    00  00  00  00  00  00`,
	` 0000    0000   00   00  00   00  0000   00    00000    0000    00000`,
}

const artWidth = 69

func terminalWidth() int {
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
	if int(ret) == -1 || ws.Col == 0 {
		return 80
	}
	return int(ws.Col)
}

func printAsciiArt() {
	tw := terminalWidth() - 2
	if tw < 20 {
		tw = 20
	}
	targetWidth := tw
	if targetWidth > artWidth {
		targetWidth = artWidth
	}
	ratio := float64(artWidth) / float64(targetWidth)
	yellow := "\033[33m"
	reset := "\033[0m"
	for _, line := range asciiLines {
		padded := line
		for len(padded) < artWidth {
			padded += " "
		}
		var sb strings.Builder
		for i := 0; i < targetWidth; i++ {
			srcIdx := int(float64(i) * ratio)
			if srcIdx >= len(padded) {
				srcIdx = len(padded) - 1
			}
			sb.WriteByte(padded[srcIdx])
		}
		fmt.Printf("%s%s%s\n", yellow, strings.TrimRight(sb.String(), " "), reset)
	}
}
