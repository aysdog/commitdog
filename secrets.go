package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type secretPattern struct {
	name    string
	pattern *regexp.Regexp
}

var secretPatterns = []secretPattern{
	{
		name:    "AWS access key",
		pattern: regexp.MustCompile(`AKIA[0-9A-Z]{10,20}`),
	},
	{
		name:    "AWS secret key",
		pattern: regexp.MustCompile(`(?i)aws.{0,20}secret.{0,20}['\"][0-9a-zA-Z/+]{20,}`),
	},
	{
		name:    "GitHub token",
		pattern: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{20,}`),
	},
	{
		name:    "GitHub fine-grained token",
		pattern: regexp.MustCompile(`github_pat_[A-Za-z0-9_]{30,}`),
	},
	{
		name:    "private key",
		pattern: regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
	},
	{
		name:    "generic API key",
		pattern: regexp.MustCompile(`(?i)(api_key|apikey|api-key)\s*[:=]\s*['\"]?[A-Za-z0-9_\-]{16,}`),
	},
	{
		name:    "generic secret",
		pattern: regexp.MustCompile(`(?i)(secret|password|passwd|pwd)\s*[:=]\s*['\"][^'"]{8,}['\"]`),
	},
	{
		name:    "Slack token",
		pattern: regexp.MustCompile(`xox[baprs]-[0-9A-Za-z\-]{10,}`),
	},
	{
		name:    "Stripe key",
		pattern: regexp.MustCompile(`(sk|pk)_(live|test)_[0-9a-zA-Z]{16,}`),
	},
	{
		name:    "Heroku API key",
		pattern: regexp.MustCompile(`[hH]eroku.{0,20}[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
	},
}

type secretHit struct {
	name string
	line string
	file string
}

func scanForSecrets(diff string) []secretHit {
	var hits []secretHit
	currentFile := ""

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}

		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}

		content := line[1:]

		if isLikelyTestOrExample(currentFile, content) {
			continue
		}

		for _, sp := range secretPatterns {
			if sp.pattern.MatchString(content) {
				hits = append(hits, secretHit{
					name: sp.name,
					line: strings.TrimSpace(content),
					file: currentFile,
				})
				break
			}
		}
	}
	return hits
}

func isLikelyTestOrExample(file, line string) bool {
	file = strings.ToLower(file)
	line = strings.ToLower(line)

	if strings.Contains(file, "_test") || strings.Contains(file, "test_") ||
		strings.Contains(file, ".test.") || strings.Contains(file, "spec") ||
		strings.Contains(file, "fixture") || strings.Contains(file, "mock") ||
		strings.Contains(file, "fake") {
		return true
	}

	if strings.Contains(line, "placeholder") ||
		strings.Contains(line, "your_") || strings.Contains(line, "<your") ||
		strings.Contains(line, "changeme") {
		return true
	}

	return false
}

func checkSecretsInDiff(diff string) bool {
	hits := scanForSecrets(diff)
	if len(hits) == 0 {
		return true
	}

	fmt.Println()
	fmt.Printf("  %s possible secret%s detected in staged changes:\n", colorRed("✗"), plural(len(hits)))
	fmt.Println()

	seen := map[string]bool{}
	for _, h := range hits {
		key := h.name + h.file
		if seen[key] {
			continue
		}
		seen[key] = true
		fmt.Printf("  · %s", h.name)
		if h.file != "" {
			fmt.Printf("  in %s", h.file)
		}
		fmt.Println()
		if len(h.line) > 72 {
			h.line = h.line[:72] + "..."
		}
		fmt.Printf("    %s\n", colorRed(h.line))
	}

	fmt.Println()
	fmt.Println("  commit anyway? this will push secrets to your remote. [y/N] ›")
	fmt.Printf("  › ")
	input := readLine()
	if input == "y" || input == "yes" {
		fmt.Println()
		return true
	}

	fmt.Println()
	fmt.Println("  commit blocked. remove the secrets from your staged changes.")
	fmt.Println("  to unstage:  git reset HEAD <file>")
	fmt.Println("  to remove from history later: git filter-repo or BFG")
	fmt.Println()
	return false
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func runSecretsHistoryScan() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	fmt.Println()
	fmt.Println("  scanning commit history for secrets...")
	fmt.Println()

	cmd := exec.Command("git", "log", "--all", "--oneline", "--format=%H")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		fatal("could not read git log: %v", err)
	}

	hashes := []string{}
	for _, h := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		h = strings.TrimSpace(h)
		if h != "" {
			hashes = append(hashes, h)
		}
	}

	if len(hashes) == 0 {
		fmt.Println("  no commits found.")
		return
	}

	fmt.Printf("  checking %d commit%s...\n\n", len(hashes), plural(len(hashes)))

	type historyHit struct {
		hash    string
		subject string
		secretHit
	}

	var allHits []historyHit
	seen := map[string]bool{}

	for _, hash := range hashes {
		diffCmd := exec.Command("git", "show", "--no-color", "--format=", hash)
		var diffOut bytes.Buffer
		diffCmd.Stdout = &diffOut
		diffCmd.Env = append(os.Environ(), "GIT_PAGER=cat")
		diffCmd.Run()

		hits := scanForSecrets(diffOut.String())
		if len(hits) == 0 {
			continue
		}

		subjCmd := exec.Command("git", "log", "-1", "--format=%s", hash)
		var subjOut bytes.Buffer
		subjCmd.Stdout = &subjOut
		subjCmd.Run()
		subject := strings.TrimSpace(subjOut.String())

		for _, h := range hits {
			key := hash + h.name + h.file + h.line
			if seen[key] {
				continue
			}
			seen[key] = true
			allHits = append(allHits, historyHit{
				hash:      hash[:7],
				subject:   subject,
				secretHit: h,
			})
		}
	}

	if len(allHits) == 0 {
		fmt.Printf("  %s no secrets found in history\n\n", colorGreen("✓"))
		return
	}

	fmt.Printf("  %s found %d secret%s in history:\n\n", colorRed("✗"), len(allHits), plural(len(allHits)))

	for _, h := range allHits {
		fmt.Printf("  commit %s  %s\n", h.hash, h.subject)
		fmt.Printf("  · %s in %s\n", h.name, h.file)
		line := h.line
		if len(line) > 72 {
			line = line[:72] + "..."
		}
		fmt.Printf("    %s\n\n", colorRed(line))
	}

	fmt.Println("  to remove secrets from history:")
	fmt.Println("  → git filter-repo --path <file> --invert-paths")
	fmt.Println("  → or use BFG Repo Cleaner: https://rtyley.github.io/bfg-repo-cleaner/")
	fmt.Println()
}
