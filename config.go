package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	Token string
	Email string
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "commitdog", "config.toml")
}

// loadConfig reads config from disk. returns empty config if not found.
func loadConfig() config {
	path := configPath()
	f, err := os.Open(path)
	if err != nil {
		return config{}
	}
	defer f.Close()

	c := config{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "token = ") {
			c.Token = strings.Trim(strings.TrimPrefix(line, "token = "), "\"")
		}
		if strings.HasPrefix(line, "email = ") {
			c.Email = strings.Trim(strings.TrimPrefix(line, "email = "), "\"")
		}
	}
	return c
}

// saveConfig writes config to disk.
func saveConfig(c config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	// sanitize before saving
	c.Token = sanitizeInput(c.Token)
	c.Email = sanitizeInput(c.Email)

	content := fmt.Sprintf("token = \"%s\"\nemail = \"%s\"\n", c.Token, c.Email)
	// 0600 — only owner can read/write, token stays private
	return os.WriteFile(path, []byte(content), 0600)
}

// runSetup walks the user through first-time setup.
func runSetup() {
	fmt.Println()
	fmt.Println("  commitdog setup")
	fmt.Println()

	existing := loadConfig()

	// --- email ---
	currentEmail := getGitEmail()
	if currentEmail != "" {
		fmt.Printf("  git email is set to: %s\n", currentEmail)
		fmt.Printf("  change it? [y/N] › ")
		if strings.ToLower(readLine()) != "y" {
			existing.Email = currentEmail
			goto tokenSetup
		}
	}

	fmt.Println("  enter your GitHub noreply email.")
	fmt.Println("  find it at: github.com/settings/emails")
	fmt.Println("  format: username@users.noreply.github.com")
	fmt.Println()
	fmt.Printf("  email › ")

	for {
		input := sanitizeInput(readLine())
		if input == "" {
			fmt.Printf("  email cannot be empty › ")
			continue
		}
		if !strings.Contains(input, "@") {
			fmt.Printf("  doesn't look like an email › ")
			continue
		}
		existing.Email = input
		break
	}

	// set globally in git
	if err := setGitEmail(existing.Email); err != nil {
		fmt.Printf("  warning: could not set git email: %s\n", err)
	} else {
		fmt.Printf("  ✓ git email set to %s\n", existing.Email)
	}

tokenSetup:
	fmt.Println()

	// --- token ---
	if existing.Token != "" {
		fmt.Println("  github token already saved.")
		fmt.Printf("  replace it? [y/N] › ")
		if strings.ToLower(readLine()) != "y" {
			goto save
		}
	}

	fmt.Println("  enter your GitHub personal access token.")
	fmt.Println("  create one at: github.com/settings/tokens")
	fmt.Println("  required scopes: repo, write:org")
	fmt.Println()
	fmt.Printf("  token › ")

	for {
		input := sanitizeInput(readLine())
		if input == "" {
			fmt.Printf("  token cannot be empty › ")
			continue
		}
		if !strings.HasPrefix(input, "ghp_") && !strings.HasPrefix(input, "github_pat_") {
			fmt.Printf("  doesn't look like a GitHub token (should start with ghp_ or github_pat_) › ")
			continue
		}
		existing.Token = input
		break
	}

save:
	if err := saveConfig(existing); err != nil {
		fatal("could not save config: %v", err)
	}

	fmt.Println()
	fmt.Printf("  ✓ saved to %s\n", configPath())
	fmt.Println("  ✓ commitdog is ready. run 'commitdog init' in any folder to create a repo.")
	fmt.Println()
}

// checkFirstRun checks if email is set, prompts if not.
func checkFirstRun() {
	email := getGitEmail()
	if email != "" {
		return
	}

	c := loadConfig()
	if c.Email != "" {
		_ = setGitEmail(c.Email)
		return
	}

	fmt.Println()
	fmt.Println("  git email is not set — this causes push rejections on GitHub.")
	fmt.Println("  find your noreply email at: github.com/settings/emails")
	fmt.Println()
	fmt.Printf("  email › ")

	for {
		input := sanitizeInput(readLine())
		if input == "" {
			fmt.Printf("  email cannot be empty › ")
			continue
		}
		if !strings.Contains(input, "@") {
			fmt.Printf("  doesn't look like an email › ")
			continue
		}
		c.Email = input
		break
	}

	if err := setGitEmail(c.Email); err != nil {
		fmt.Printf("  warning: could not set git email: %s\n", err)
	} else {
		fmt.Printf("  ✓ git email set globally\n\n")
	}

	_ = saveConfig(c)
}

// sanitizeInput strips dangerous characters from user input.
func sanitizeInput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
