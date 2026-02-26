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
			fmt.Println(`
		/ ^ ^ \
		/  o o  \
		( =  Y  = )  commitdog v` + version + `
		)       (   zero-bs commits
		(_|___|_|_)  aysdog.pages.dev
		`)
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

	// check email on every run — silent if already set
	checkFirstRun()

	// verify we're inside a git repo
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository. run 'commitdog init' to create one.")
	}

	// get staged diff
	diff, err := getStagedDiff()
	if err != nil {
		fatal("failed to read staged diff: %v", err)
	}

	if diff == "" {
		fatal("nothing staged. run 'git add .' first.")
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
		fmt.Println("  aborted.")
		os.Exit(0)
	}

	// commit
	if err := runCommit(chosen); err != nil {
		fatal("commit failed: %v", err)
	}

	fmt.Printf("\n  ✓ committed: %s\n", chosen)

	// ask to push
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
