package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("commitdog v%s\n", version)
			os.Exit(0)
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", os.Args[1])
			os.Exit(1)
		}
	}

	// verify we're inside a git repo
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository (or no git installed)")
	}

	// get staged diff
	diff, err := getStagedDiff()
	if err != nil {
		fatal("failed to read staged diff: %v", err)
	}

	if diff == "" {
		fatal("nothing staged. run 'git add' first.")
	}

	// analyze the diff
	analysis := analyzeDiff(diff)

	if analysis.filesChanged == 0 {
		fatal("no changes detected in staged diff.")
	}

	// generate suggestions
	suggestions := generateSuggestions(analysis)

	// show picker
	chosen := pickSuggestion(suggestions)
	if chosen == "" {
		fmt.Println("aborted.")
		os.Exit(0)
	}

	// commit
	if err := runCommit(chosen); err != nil {
		fatal("commit failed: %v", err)
	}

	fmt.Printf("\n✓ committed: %s\n", chosen)

	// ask to push
	askPush()
}

func printHelp() {
	fmt.Println(`commitdog — zero-bs commit message generator

usage:
  commitdog          generate commit message from staged diff
  commitdog -v       show version
  commitdog -h       show this help

how it works:
  1. reads your staged git diff
  2. analyzes changed files and patterns
  3. suggests 2-3 conventional commit messages
  4. you pick one, it commits
  5. optionally pushes for you

no ai. no network. no telemetry. just works.`)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "commitdog: "+format+"\n", args...)
	os.Exit(1)
}
