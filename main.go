package main

import (
	"fmt"
	"os"
)

const version = "0.1.1"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("commitdog v%s\n", version)
			fmt.Println("zero-bs commits · no AI · no bs")
			fmt.Println("aysdog.pages.dev")
			os.Exit(0)
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		case "setup":
			runSetup()
			os.Exit(0)
		case "init":
			runInit()
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
			fmt.Fprintf(os.Stderr, "run 'commitdog --help' for usage.\n")
			os.Exit(1)
		}
	}

	checkFirstRun()

	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository. run 'commitdog init' to create one.")
	}

	diff, err := getStagedDiff()
	if err != nil {
		fatal("failed to read staged diff: %v", err)
	}

	if diff == "" {
		fatal("nothing staged. run 'git add .' first.")
	}

	analysis := analyzeDiff(diff)

	if analysis.filesChanged == 0 {
		fatal("no changes detected in staged diff.")
	}

	suggestions := generateSuggestions(analysis)

	chosen := pickSuggestion(suggestions)
	if chosen == "" {
		fmt.Println("  aborted.")
		os.Exit(0)
	}

	if err := runCommit(chosen); err != nil {
		fatal("commit failed: %v", err)
	}

	fmt.Printf("\n  ✓ committed: %s\n", chosen)

	askPush()
}

func printHelp() {
	fmt.Println(`commitdog — zero-bs commit message generator

usage:
  commitdog          generate commit message from staged diff
  commitdog init     create a new GitHub repo and do the first push
  commitdog setup    configure email and GitHub token
  commitdog -v       show version
  commitdog -h       show this help

workflow:
  first time:
    commitdog setup  ← set email + GitHub token once
    mkdir my-project && cd my-project
    commitdog init   ← creates repo on GitHub, first commit, push

  daily:
    git add .
    commitdog        ← suggests message, commits, asks to push

no ai. no network (except init). no telemetry. just works.`)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\n  commitdog: "+format+"\n\n", args...)
	os.Exit(1)
}
