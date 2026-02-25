package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var reader = bufio.NewReader(os.Stdin)

// pickSuggestion shows suggestions and returns the chosen message.
// Returns empty string if user aborts.
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

		// check numeric pick
		for i := range suggestions {
			if input == fmt.Sprintf("%d", i+1) {
				return suggestions[i]
			}
		}

		// invalid input — re-prompt on same line
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

// editMessage lets user modify a suggestion before committing.
func editMessage(suggestion string) string {
	fmt.Printf("\n  edit message (enter to keep, ctrl+c to abort):\n")
	fmt.Printf("  > %s\n", suggestion)
	fmt.Printf("  > ")

	input := readLine()
	if input == "" {
		return suggestion
	}

	// sanitize user input
	cleaned := sanitizeMessage(input)
	if cleaned == "" {
		fmt.Println("  empty message, using original.")
		return suggestion
	}
	return cleaned
}

// askPush asks whether to push after committing.
func askPush() {
	remotes := getRemotes()
	if len(remotes) == 0 {
		// no remotes configured, skip push prompt
		return
	}

	branch := getCurrentBranch()
	if branch == "" || branch == "HEAD" {
		return
	}

	remote := remotes[0] // default to first remote (usually "origin")

	fmt.Printf("\n  push to %s/%s? [y/n] › ", remote, branch)

	for {
		input := readLine()
		switch input {
		case "y", "yes":
			fmt.Printf("  pushing...")
			if err := runPush(remote, branch); err != nil {
				fmt.Printf("\n  push failed: %s\n", err)
			} else {
				fmt.Printf("\n  ✓ pushed to %s/%s\n", remote, branch)
			}
			return
		case "n", "no", "":
			fmt.Println("  skipped. push it yourself when ready.")
			return
		default:
			fmt.Printf("  y or n › ")
		}
	}
}

// readLine reads a trimmed line from stdin.
func readLine() string {
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}
