//go:build windows
// +build windows

package main

import (
	"fmt"
)

var asciiLines = []string{
	`                              ▄▄             `,
	`                              ▀▀  ██      ██             `,
	`▄████ ▄███▄ ███▄███▄ ███▄███▄ ██ ▀██▀▀ ▄████ ▄███▄ ▄████ `,
	`██    ██ ██ ██ ██ ██ ██ ██ ██ ██  ██   ██ ██ ██ ██ ██ ██ `,
	`▀████ ▀███▀ ██ ██ ██ ██ ██ ██ ██▄ ██   ▀████ ▀███▀ ▀████ `,
	`                                                      ██ `,
	`                                                    ▀▀▀  `,
}

func printAsciiArt() {
	yellow := "\033[33m"
	bold := "\033[1m"
	dim := "\033[90m"
	reset := "\033[0m"

	margin := "  "
	for _, line := range asciiLines {
		fmt.Printf("%s%s%s\n", yellow, margin, line+reset)
	}

	fmt.Println()
	fmt.Println("  " + bold + yellow + "commitdog" + reset + dim + " v" + version + reset)
	fmt.Println("  " + dim + "─────────────────────────────────" + reset)
	fmt.Println("  zero-bs commits · no AI · no telemetry")
	fmt.Println()
	fmt.Println(dim + "  git, but make it not painful." + reset)
	fmt.Println()

	cmds := [][]string{
		{"commitdog", "stage · commit · push"},
		{"commitdog init", "create repo · first push"},
		{"commitdog log", "colored branch graph"},
		{"commitdog branch", "branch management"},
		{"commitdog switch", "switch branches fast"},
		{"commitdog merge", "merge with diff preview"},
		{"commitdog pr", "create · review · merge PRs"},
		{"commitdog sync", "pull · rebase · push"},
		{"commitdog stash", "save work in progress"},
		{"commitdog revert", "undo a commit"},
		{"commitdog release", "version bump · build · publish"},
		{"commitdog setup", "configure github token"},
	}

	for _, c := range cmds {
		fmt.Printf("  %s%-22s%s  %s\n", yellow, c[0], reset, c[1])
	}

	fmt.Println()
	fmt.Println(dim + "  https://commitdog.aysdog.com" + reset)
	fmt.Println()
}
