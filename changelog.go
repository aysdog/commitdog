package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func buildChangelog(sinceTag string) string {
	var args []string
	if sinceTag != "" && sinceTag != "0.0.0" {
		args = []string{"log", "v" + sinceTag + "..HEAD", "--pretty=format:%s", "--no-merges"}
	} else {
		args = []string{"log", "--pretty=format:%s", "--no-merges"}
	}
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()

	groups := map[string][]string{
		"feat":     {},
		"fix":      {},
		"chore":    {},
		"docs":     {},
		"refactor": {},
		"other":    {},
	}
	order := []string{"feat", "fix", "refactor", "docs", "chore", "other"}
	labels := map[string]string{
		"feat":     "Features",
		"fix":      "Bug Fixes",
		"refactor": "Refactoring",
		"docs":     "Documentation",
		"chore":    "Chores",
		"other":    "Other",
	}

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matched := false
		for _, key := range order[:len(order)-1] {
			if strings.HasPrefix(line, key+"(") || strings.HasPrefix(line, key+":") {
				groups[key] = append(groups[key], line)
				matched = true
				break
			}
		}
		if !matched {
			groups["other"] = append(groups["other"], line)
		}
	}

	var sb strings.Builder
	for _, key := range order {
		items := groups[key]
		if len(items) == 0 {
			continue
		}
		sb.WriteString("### " + labels[key] + "\n")
		for _, item := range items {
			sb.WriteString("- " + item + "\n")
		}
		sb.WriteString("\n")
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "No changes."
	}
	return result
}

func createGitHubReleaseWithBody(token, owner, repo, ver, body string) (int64, string, error) {
	existing, err := githubRequest("GET",
		fmt.Sprintf("/repos/%s/%s/releases/tags/v%s", owner, repo, ver),
		token, nil,
	)
	if err == nil {
		var result struct {
			ID        int64  `json:"id"`
			UploadURL string `json:"upload_url"`
		}
		if json.Unmarshal(existing, &result) == nil && result.ID != 0 {
			return result.ID, strings.Split(result.UploadURL, "{")[0], nil
		}
	}

	payload := map[string]interface{}{
		"tag_name":               "v" + ver,
		"name":                   "v" + ver,
		"body":                   body,
		"draft":                  false,
		"generate_release_notes": false,
	}
	data, err := githubRequest("POST",
		fmt.Sprintf("/repos/%s/%s/releases", owner, repo),
		token, payload,
	)
	if err != nil {
		return 0, "", err
	}
	var result struct {
		ID        int64  `json:"id"`
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, "", err
	}
	return result.ID, strings.Split(result.UploadURL, "{")[0], nil
}

func fileSHA256(path string) string {
	cmd := exec.Command("sha256sum", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		cmd2 := exec.Command("shasum", "-a", "256", path)
		var out2 bytes.Buffer
		cmd2.Stdout = &out2
		if err2 := cmd2.Run(); err2 != nil {
			return ""
		}
		parts := strings.Fields(strings.TrimSpace(out2.String()))
		if len(parts) > 0 {
			return parts[0]
		}
		return ""
	}
	parts := strings.Fields(strings.TrimSpace(out.String()))
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func runInitCI() {
	proj, _ := detectProject()
	lang := "go"
	if proj != nil {
		lang = strings.ToLower(proj.lang)
	}

	ciDir := ".github/workflows"
	if err := os.MkdirAll(ciDir, 0755); err != nil {
		fatal("could not create .github/workflows: %v", err)
	}

	ciFile := ciDir + "/release.yml"
	if _, err := os.Stat(ciFile); err == nil {
		fmt.Println("  .github/workflows/release.yml already exists, skipping.")
		return
	}

	var content string
	switch lang {
	case "go":
		content = ciYMLGo()
	case "node.js", "node":
		content = ciYMLNode()
	case "rust":
		content = ciYMLRust()
	case "python":
		content = ciYMLPython()
	default:
		content = ciYMLGeneric()
	}

	if err := os.WriteFile(ciFile, []byte(content), 0644); err != nil {
		fatal("could not write release.yml: %v", err)
	}
	fmt.Printf("  \033[32m✓\033[0m created .github/workflows/release.yml for %s\n", lang)
}

func ciYMLGo() string {
	return `name: release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: build binaries
        run: |
          GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o ${{ github.event.repository.name }}-linux-amd64 .
          GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o ${{ github.event.repository.name }}-linux-arm64 .
          GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o ${{ github.event.repository.name }}-darwin-amd64 .
          GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o ${{ github.event.repository.name }}-darwin-arm64 .
          GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ${{ github.event.repository.name }}-windows-amd64.exe .
          sha256sum ${{ github.event.repository.name }}-* > checksums.txt

      - name: create release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            ${{ github.event.repository.name }}-*
            checksums.txt
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`
}

func ciYMLNode() string {
	return `name: release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npm ci
      - run: npm run build --if-present
      - uses: softprops/action-gh-release@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`
}

func ciYMLRust() string {
	return `name: release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@stable
      - run: cargo build --release
      - uses: softprops/action-gh-release@v2
        with:
          files: target/release/${{ github.event.repository.name }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`
}

func ciYMLPython() string {
	return `name: release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'
      - run: pip install build
      - run: python -m build
      - uses: softprops/action-gh-release@v2
        with:
          files: dist/*
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`
}

func ciYMLGeneric() string {
	return `name: release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: softprops/action-gh-release@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`
}
