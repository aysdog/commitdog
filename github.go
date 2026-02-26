package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const githubAPI = "https://api.github.com"

type createRepoRequest struct {
	Name        string `json:"name"`
	Private     bool   `json:"private"`
	AutoInit    bool   `json:"auto_init"`
	Description string `json:"description"`
}

type repoResponse struct {
	SSHURL  string `json:"ssh_url"`
	HTMLURL string `json:"html_url"`
	Name    string `json:"name"`
}

type githubUser struct {
	Login string `json:"login"`
}

// getGitHubUsername fetches the authenticated user's login.
func getGitHubUsername(token string) (string, error) {
	body, err := githubRequest("GET", "/user", token, nil)
	if err != nil {
		return "", err
	}
	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	return u.Login, nil
}

// createPersonalRepo creates a repo under the authenticated user.
func createPersonalRepo(token, name string, private bool) (repoResponse, error) {
	payload := createRepoRequest{
		Name:     name,
		Private:  private,
		AutoInit: false,
	}
	body, err := githubRequest("POST", "/user/repos", token, payload)
	if err != nil {
		return repoResponse{}, err
	}
	var r repoResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return repoResponse{}, err
	}
	return r, nil
}

// createOrgRepo creates a repo under a GitHub org.
func createOrgRepo(token, org, name string, private bool) (repoResponse, error) {
	// validate org name
	if !isSafeGitRef(org) {
		return repoResponse{}, fmt.Errorf("invalid org name")
	}
	payload := createRepoRequest{
		Name:     name,
		Private:  private,
		AutoInit: false,
	}
	body, err := githubRequest("POST", "/orgs/"+org+"/repos", token, payload)
	if err != nil {
		return repoResponse{}, err
	}
	var r repoResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return repoResponse{}, err
	}
	return r, nil
}

// githubRequest makes an authenticated GitHub API request.
func githubRequest(method, path string, token string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, githubAPI+path, body)
	if err != nil {
		return nil, err
	}

	// never log or expose the token
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "commitdog/0.1.1")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		if errResp.Message != "" {
			return nil, fmt.Errorf("github: %s", errResp.Message)
		}
		return nil, fmt.Errorf("github: HTTP %d", resp.StatusCode)
	}

	return respBody, nil
}

func getGitEmail() string {
	cmd := exec.Command("git", "config", "--global", "user.email")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func setGitEmail(email string) error {
	email = sanitizeInput(email)
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("invalid email")
	}
	return exec.Command("git", "config", "--global", "user.email", email).Run()
}

func hasSSHKey() bool {
	cmd := exec.Command("ssh", "-T", "git@github.com", "-o", "StrictHostKeyChecking=no", "-o", "BatchMode=yes")
	err := cmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 1 {
			return true
		}
	}
	return false
}
