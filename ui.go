package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var reader = bufio.NewReader(os.Stdin)

func pickSuggestion(suggestions []string) string {
	fmt.Println()
	fmt.Println("  suggestions:")
	fmt.Println()

	for i, s := range suggestions {
		fmt.Printf("  %d  %s\n", i+1, s)
	}

	fmt.Println()
	fmt.Printf("  [")
	for i := range suggestions {
		if i > 0 {
			fmt.Printf("/")
		}
		fmt.Printf("%d", i+1)
	}
	fmt.Printf("] pick, [e] edit, [q] quit › ")

	for {
		input := readLine()

		switch input {
		case "q", "quit", "exit":
			return ""
		case "e", "edit":
			return editMessage(suggestions[0])
		}

		for i := range suggestions {
			if input == fmt.Sprintf("%d", i+1) {
				return suggestions[i]
			}
		}

		fmt.Printf("  enter ")
		for i := range suggestions {
			if i > 0 {
				fmt.Printf("/")
			}
			fmt.Printf("%d", i+1)
		}
		fmt.Printf(", e, or q › ")
	}
}

func editMessage(suggestion string) string {
	fmt.Printf("\n  edit message (enter to keep, ctrl+c to abort):\n")
	fmt.Printf("  > %s\n", suggestion)
	fmt.Printf("  > ")

	input := readLine()
	if input == "" {
		return suggestion
	}

	cleaned := sanitizeMessage(input)
	if cleaned == "" {
		fmt.Println("  empty message, using original.")
		return suggestion
	}
	return cleaned
}

func askPush() {
	remotes := getRemotes()
	if len(remotes) == 0 {
		return
	}

	branch := getCurrentBranch()
	if branch == "" || branch == "HEAD" {
		return
	}

	remote := remotes[0]

	fmt.Printf("\n  push to %s/%s? [Y/n] › ", remote, branch)

	for {
		input := readLine()
		switch input {
		case "y", "yes", "":
			fmt.Printf("  pushing...")
			var err error
			if !hasUpstream(branch) {
				err = runPushUpstream(remote, branch)
			} else {
				err = runPush(remote, branch)
			}
			if err != nil {
				fmt.Printf("\n  push failed: %s\n", err)
			} else {
				fmt.Printf("\n  ✓ pushed to %s/%s\n", remote, branch)
			}
			return
		case "n", "no":
			fmt.Println("  skipped. push it yourself when ready.")
			return
		default:
			fmt.Printf("  Y or n › ")
		}
	}
}

func hasUpstream(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", branch+"@{upstream}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func readLine() string {
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}
