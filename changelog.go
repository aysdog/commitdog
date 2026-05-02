package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
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
			githubRequest("PATCH",
				fmt.Sprintf("/repos/%s/%s/releases/%d", owner, repo, result.ID),
				token, map[string]interface{}{"body": body},
			)
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
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
