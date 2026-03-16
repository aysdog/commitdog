package main

import (
	"fmt"
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
		strings.Contains(file, "example") || strings.Contains(file, "fixture") ||
		strings.Contains(file, "mock") || strings.Contains(file, "fake") {
		return true
	}

	if strings.Contains(line, "example") || strings.Contains(line, "placeholder") ||
		strings.Contains(line, "your_") || strings.Contains(line, "<your") ||
		strings.Contains(line, "xxx") || strings.Contains(line, "dummy") ||
		strings.Contains(line, "changeme") || strings.Contains(line, "todo") {
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
