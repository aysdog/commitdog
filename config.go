package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type platformConfig struct {
	Token string
	Host  string
	Email string
}

type config struct {
	Email   string
	GitHub  platformConfig
	GitLab  platformConfig
	Gitea   platformConfig
	Forgejo platformConfig
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "commitdog", "config.toml")
}

func loadConfig() config {
	f, err := os.Open(configPath())
	if err != nil {
		return config{}
	}
	defer f.Close()

	c := config{}
	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line[1 : len(line)-1])
			continue
		}
		key, val, ok := parseKV(line)
		if !ok {
			continue
		}
		switch section {
		case "":
			if key == "email" {
				c.Email = val
			}
		case "github":
			switch key {
			case "token":
				c.GitHub.Token = val
			case "email":
				c.GitHub.Email = val
			}
		case "gitlab":
			switch key {
			case "token":
				c.GitLab.Token = val
			case "host":
				c.GitLab.Host = val
			case "email":
				c.GitLab.Email = val
			}
		case "gitea":
			switch key {
			case "token":
				c.Gitea.Token = val
			case "host":
				c.Gitea.Host = val
			case "email":
				c.Gitea.Email = val
			}
		case "forgejo":
			switch key {
			case "token":
				c.Forgejo.Token = val
			case "host":
				c.Forgejo.Host = val
			case "email":
				c.Forgejo.Email = val
			}
		}
	}
	return c
}

func parseKV(line string) (string, string, bool) {
	parts := strings.SplitN(line, " = ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), "\""), true
}

func saveConfig(c config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	c.Email = sanitizeInput(c.Email)
	c.GitHub.Token = sanitizeInput(c.GitHub.Token)
	c.GitHub.Email = sanitizeInput(c.GitHub.Email)
	c.GitLab.Token = sanitizeInput(c.GitLab.Token)
	c.GitLab.Host = sanitizeInput(c.GitLab.Host)
	c.GitLab.Email = sanitizeInput(c.GitLab.Email)
	c.Gitea.Token = sanitizeInput(c.Gitea.Token)
	c.Gitea.Host = sanitizeInput(c.Gitea.Host)
	c.Gitea.Email = sanitizeInput(c.Gitea.Email)
	c.Forgejo.Token = sanitizeInput(c.Forgejo.Token)
	c.Forgejo.Host = sanitizeInput(c.Forgejo.Host)
	c.Forgejo.Email = sanitizeInput(c.Forgejo.Email)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("email = \"%s\"\n", c.Email))

	if c.GitHub.Token != "" {
		sb.WriteString("\n[github]\n")
		sb.WriteString(fmt.Sprintf("token = \"%s\"\n", c.GitHub.Token))
		if c.GitHub.Email != "" {
			sb.WriteString(fmt.Sprintf("email = \"%s\"\n", c.GitHub.Email))
		}
	}
	if c.GitLab.Token != "" {
		sb.WriteString("\n[gitlab]\n")
		sb.WriteString(fmt.Sprintf("token = \"%s\"\n", c.GitLab.Token))
		if c.GitLab.Host != "" {
			sb.WriteString(fmt.Sprintf("host = \"%s\"\n", c.GitLab.Host))
		}
		if c.GitLab.Email != "" {
			sb.WriteString(fmt.Sprintf("email = \"%s\"\n", c.GitLab.Email))
		}
	}
	if c.Gitea.Token != "" {
		sb.WriteString("\n[gitea]\n")
		sb.WriteString(fmt.Sprintf("token = \"%s\"\n", c.Gitea.Token))
		if c.Gitea.Host != "" {
			sb.WriteString(fmt.Sprintf("host = \"%s\"\n", c.Gitea.Host))
		}
		if c.Gitea.Email != "" {
			sb.WriteString(fmt.Sprintf("email = \"%s\"\n", c.Gitea.Email))
		}
	}
	if c.Forgejo.Token != "" {
		sb.WriteString("\n[forgejo]\n")
		sb.WriteString(fmt.Sprintf("token = \"%s\"\n", c.Forgejo.Token))
		if c.Forgejo.Host != "" {
			sb.WriteString(fmt.Sprintf("host = \"%s\"\n", c.Forgejo.Host))
		}
		if c.Forgejo.Email != "" {
			sb.WriteString(fmt.Sprintf("email = \"%s\"\n", c.Forgejo.Email))
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0600)
}

func emailForPlatform(c config, platform string) string {
	switch platform {
	case "gitlab":
		if c.GitLab.Email != "" {
			return c.GitLab.Email
		}
	case "gitea":
		if c.Gitea.Email != "" {
			return c.Gitea.Email
		}
	case "forgejo":
		if c.Forgejo.Email != "" {
			return c.Forgejo.Email
		}
	default:
		if c.GitHub.Email != "" {
			return c.GitHub.Email
		}
	}
	return c.Email
}

func tokenForPlatform(c config, platform string) string {
	switch platform {
	case "gitlab":
		return c.GitLab.Token
	case "gitea":
		return c.Gitea.Token
	case "forgejo":
		return c.Forgejo.Token
	default:
		return c.GitHub.Token
	}
}

func runSetup() {
	fmt.Println()
	fmt.Println("  commitdog setup")
	fmt.Println()

	existing := loadConfig()

	fmt.Println("  which platform token do you want to configure?")
	fmt.Println()
	fmt.Println("  1  github")
	fmt.Println("  2  gitlab")
	fmt.Println("  3  gitea")
	fmt.Println("  4  forgejo")
	fmt.Println()
	fmt.Printf("  [1/2/3/4] pick › ")

	platform := ""
	for {
		switch strings.TrimSpace(readLine()) {
		case "1":
			platform = "github"
		case "2":
			platform = "gitlab"
		case "3":
			platform = "gitea"
		case "4":
			platform = "forgejo"
		default:
			fmt.Printf("  [1/2/3/4] pick › ")
			continue
		}
		break
	}

	fmt.Println()
	setupEmail(&existing, platform)
	setupToken(&existing, platform)

	if err := saveConfig(existing); err != nil {
		fatal("could not save config: %v", err)
	}

	fmt.Println()
	fmt.Printf("  ✓ saved to %s\n", configPath())
	fmt.Println()
}

func setupEmail(c *config, platform string) {
	var current string
	switch platform {
	case "gitlab":
		current = c.GitLab.Email
	case "gitea":
		current = c.Gitea.Email
	case "forgejo":
		current = c.Forgejo.Email
	default:
		current = c.GitHub.Email
	}
	if current == "" {
		current = c.Email
	}

	if current != "" {
		fmt.Printf("  email for %s: %s\n", platform, current)
		fmt.Printf("  change it? [y/N] › ")
		if strings.ToLower(readLine()) != "y" {
			fmt.Println()
			return
		}
	}

	fmt.Println()
	fmt.Printf("  email for %s › ", platform)

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
		switch platform {
		case "gitlab":
			c.GitLab.Email = input
		case "gitea":
			c.Gitea.Email = input
		case "forgejo":
			c.Forgejo.Email = input
		default:
			c.GitHub.Email = input
			c.Email = input
		}
		break
	}

	fmt.Println()
}

func setupToken(c *config, platform string) {
	var current string
	switch platform {
	case "gitlab":
		current = c.GitLab.Token
	case "gitea":
		current = c.Gitea.Token
	default:
		current = c.GitHub.Token
	}

	if current != "" {
		fmt.Printf("  %s token already saved.\n", platform)
		fmt.Printf("  replace it? [y/N] › ")
		if strings.ToLower(readLine()) != "y" {
			return
		}
		fmt.Println()
	}

	switch platform {
	case "github":
		fmt.Println("  enter your GitHub personal access token.")
		fmt.Println("  create one at: github.com/settings/tokens")
		fmt.Println("  required scopes: repo, write:org")
	case "gitlab":
		fmt.Println("  enter your GitLab personal access token.")
		fmt.Println("  create one at: gitlab.com/-/profile/personal_access_tokens")
		fmt.Println("  required scopes: api")
	case "gitea":
		fmt.Println("  enter your Gitea personal access token.")
		fmt.Println("  create one under: settings → applications")
	case "forgejo":
		fmt.Println("  enter your Forgejo personal access token.")
		fmt.Println("  create one under: settings → applications")
	}

	fmt.Println()
	fmt.Printf("  token › ")

	for {
		input := sanitizeInput(readLine())
		if input == "" {
			fmt.Printf("  token cannot be empty › ")
			continue
		}
		switch platform {
		case "github":
			if !strings.HasPrefix(input, "ghp_") && !strings.HasPrefix(input, "github_pat_") {
				fmt.Printf("  doesn't look like a GitHub token (ghp_ or github_pat_) › ")
				continue
			}
			c.GitHub.Token = input
		case "gitlab":
			if !strings.HasPrefix(input, "glpat-") {
				fmt.Printf("  doesn't look like a GitLab token (glpat-) › ")
				continue
			}
			c.GitLab.Token = input
			fmt.Println()
			setupHost(&c.GitLab, "https://gitlab.com")
		case "gitea":
			c.Gitea.Token = input
			fmt.Println()
			setupHost(&c.Gitea, "")
		case "forgejo":
			c.Forgejo.Token = input
			fmt.Println()
			setupHost(&c.Forgejo, "")
		}
		break
	}

	fmt.Println()
}

func setupHost(pc *platformConfig, defaultHost string) {
	if defaultHost != "" {
		fmt.Printf("  self-hosted instance? [enter] for %s or paste URL › ", defaultHost)
		input := sanitizeInput(readLine())
		if input == "" {
			pc.Host = defaultHost
		} else {
			pc.Host = strings.TrimRight(input, "/")
		}
	} else {
		fmt.Printf("  instance URL (e.g. https://git.yourcompany.com) › ")
		pc.Host = strings.TrimRight(sanitizeInput(readLine()), "/")
	}
}

func checkFirstRun() {
	c := loadConfig()
	proj := loadProjectConfig()
	platform := proj.platform
	if platform == "" {
		platform = "github"
	}

	email := emailForPlatform(c, platform)
	if email != "" {
		_ = setGitEmailLocal(email)
		return
	}

	if getGitEmail() != "" {
		return
	}

	fmt.Println()
	fmt.Println("  git email is not set.")
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
		fmt.Printf("  ✓ git email set\n\n")
	}

	_ = saveConfig(c)
}

func sanitizeInput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
