//go:build windows
// +build windows

package main

import (
	"fmt"
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

func printAsciiArt() {
	yellow := "\033[33m"
	cyan := "\033[36m"
	bold := "\033[1m"
	dim := "\033[90m"
	reset := "\033[0m"

	info := []string{
		bold + cyan + "commitdog" + reset + dim + " v" + version + reset,
		dim + "─────────────────────────────────" + reset,
		"zero-bs commits · no AI · no telemetry",
		"",
		cyan + "commitdog" + reset + "              stage · commit · push",
		cyan + "commitdog init" + reset + "         create repo · first push",
		cyan + "commitdog log" + reset + "          colored branch graph",
		cyan + "commitdog branch" + reset + "       branch management",
		cyan + "commitdog merge" + reset + "        merge with preview",
		cyan + "commitdog sync" + reset + "         pull · rebase · push",
		cyan + "commitdog stash" + reset + "        save work in progress",
		cyan + "commitdog revert" + reset + "       undo a commit",
		cyan + "commitdog setup" + reset + "        configure github token",
		"",
		dim + "aysdog.com" + reset,
		"",
	}

	gap := "    "
	for i, line := range asciiLines {
		art := yellow + line + reset
		if i < len(info) {
			fmt.Printf("%s%s%s\n", art, gap, info[i])
		} else {
			fmt.Printf("%s\n", art)
		}
	}

	if len(info) > len(asciiLines) {
		padding := strings.Repeat(" ", artWidth)
		for i := len(asciiLines); i < len(info); i++ {
			fmt.Printf("%s%s%s\n", padding, gap, info[i])
		}
	}
}
