package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

func gitInit() error {
	cmd := exec.Command("git", "init", "-b", "main")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cmd2 := exec.Command("git", "init")
		cmd2.Stderr = &stderr
		if err2 := cmd2.Run(); err2 != nil {
			return fmt.Errorf("%s", stderr.String())
		}
	}
	return nil
}

func gitSetRemote(name, url string) error {
	if url == "" {
		return fmt.Errorf("empty remote url")
	}
	cmd := exec.Command("git", "remote", "add", name, url)
	if err := cmd.Run(); err != nil {
		cmd2 := exec.Command("git", "remote", "set-url", name, url)
		return cmd2.Run()
	}
	return nil
}

func gitAddAll() error {
	cmd := exec.Command("git", "add", ".")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", stderr.String())
	}
	return nil
}
