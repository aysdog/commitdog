//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var asciiLines = []string{
	`00000000            00000000  `,
	`0000000000000000000000000000  `,
	`000  000000000000000000  000  `,
	`000000000          000000000  `,
	`0000000              0000000  `,
	` 0000   00       0000  0000   `,
	`0000   0000      00000  0000  `,
	`0000   0000      0000   0000  `,
	` 000                    0000  `,
	` 0000      000000      0000   `,
	` 0000     00000000     0000   `,
	`  0000     000000     0000    `,
	`   0000     0000     0000     `,
	`    00000000000000000000      `,
	`      0000000000000000        `,
	`                              `,
}

const artWidth = 30

func terminalWidth() int {
	cmd := exec.Command("cmd", "/C", "mode", "con")
	out, err := cmd.Output()
	if err != nil {
		return 80
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(line, "columns:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
					return n
				}
			}
		}
	}
	return 80
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
