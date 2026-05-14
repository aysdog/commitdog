package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func giteaRequest(method, path, token, host string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, strings.TrimRight(host, "/")+"/api/v1"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "commitdog/"+version)

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
			return nil, fmt.Errorf("gitea: %s", errResp.Message)
		}
		return nil, fmt.Errorf("gitea: HTTP %d", resp.StatusCode)
	}

	return respBody, nil
}

func getGiteaUsername(token, host string) (string, error) {
	body, err := giteaRequest("GET", "/user", token, host, nil)
	if err != nil {
		return "", err
	}
	var u struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	return u.Login, nil
}

func getGiteaOrgs(token, host string) ([]string, error) {
	body, err := giteaRequest("GET", "/user/orgs", token, host, nil)
	if err != nil {
		return nil, err
	}
	var orgs []struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &orgs); err != nil {
		return nil, err
	}
	var names []string
	for _, o := range orgs {
		names = append(names, o.Username)
	}
	return names, nil
}

func createGiteaRepo(token, host, name string, private bool) (repoResponse, error) {
	payload := map[string]interface{}{
		"name":    name,
		"private": private,
	}
	body, err := giteaRequest("POST", "/user/repos", token, host, payload)
	if err != nil {
		return repoResponse{}, err
	}
	var r struct {
		SSHURL  string `json:"ssh_url"`
		HTMLURL string `json:"html_url"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return repoResponse{}, err
	}
	return repoResponse{SSHURL: r.SSHURL, HTMLURL: r.HTMLURL, Name: r.Name}, nil
}

func createGiteaOrgRepo(token, host, org, name string, private bool) (repoResponse, error) {
	if !isSafeGitRef(org) {
		return repoResponse{}, fmt.Errorf("invalid org name")
	}
	payload := map[string]interface{}{
		"name":    name,
		"private": private,
	}
	body, err := giteaRequest("POST", "/orgs/"+org+"/repos", token, host, payload)
	if err != nil {
		return repoResponse{}, err
	}
	var r struct {
		SSHURL  string `json:"ssh_url"`
		HTMLURL string `json:"html_url"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return repoResponse{}, err
	}
	return repoResponse{SSHURL: r.SSHURL, HTMLURL: r.HTMLURL, Name: r.Name}, nil
}

func createGiteaRelease(token, host, owner, repo, tagName, changelog string) (int64, error) {
	payload := map[string]interface{}{
		"tag_name":   "v" + tagName,
		"name":       "v" + tagName,
		"body":       changelog,
		"draft":      false,
		"prerelease": false,
	}
	body, err := giteaRequest("POST", "/repos/"+owner+"/"+repo+"/releases", token, host, payload)
	if err != nil {
		return 0, err
	}
	var r struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, err
	}
	return r.ID, nil
}

func uploadGiteaAsset(token, host, owner, repo string, releaseID int64, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	fileName := filepath.Base(filePath)
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases/%d/assets?name=%s",
		strings.TrimRight(host, "/"), owner, repo, releaseID, fileName)

	req, err := http.NewRequest("POST", url, f)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "commitdog/"+version)
	req.ContentLength = stat.Size()

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gitea asset upload failed: HTTP %d — %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func uploadGiteaAssetsBatched(token, host, owner, repo string, releaseID int64, files []string) error {
	batchSize := 4
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}
		for _, f := range files[i:end] {
			if err := uploadGiteaAsset(token, host, owner, repo, releaseID, f); err != nil {
				return err
			}
		}
		if end < len(files) {
			time.Sleep(2 * time.Second)
		}
	}
	return nil
}
