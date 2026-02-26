package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

// gitInit runs git init in the current directory.
func gitInit() error {
	cmd := exec.Command("git", "init", "-b", "main")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// fallback for older git without -b flag
		cmd2 := exec.Command("git", "init")
		cmd2.Stderr = &stderr
		if err2 := cmd2.Run(); err2 != nil {
			return fmt.Errorf("%s", stderr.String())
		}
	}
	return nil
}

// gitSetRemote adds or updates the origin remote.
func gitSetRemote(name, url string) error {
	// validate url is not empty
	if url == "" {
		return fmt.Errorf("empty remote url")
	}
	// try add first, if it exists update it
	cmd := exec.Command("git", "remote", "add", name, url)
	if err := cmd.Run(); err != nil {
		// remote already exists, update it
		cmd2 := exec.Command("git", "remote", "set-url", name, url)
		return cmd2.Run()
	}
	return nil
}

// gitAddAll stages all files.
func gitAddAll() error {
	cmd := exec.Command("git", "add", ".")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", stderr.String())
	}
	return nil
}
