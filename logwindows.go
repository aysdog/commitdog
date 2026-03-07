//go:build windows
// +build windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

func enableRawMode()  {}
func disableRawMode() {}

func terminalHeight() int {
	cmd := exec.Command("cmd", "/C", "mode", "con")
	out, err := cmd.Output()
	if err != nil {
		return 24
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(line, "lines:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
					return n
				}
			}
		}
	}
	return 24
}
