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

func gitlabHost(c config) string {
	if c.GitLab.Host != "" {
		return c.GitLab.Host
	}
	return "https://gitlab.com"
}

func gitlabRequest(method, path, token, host string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, strings.TrimRight(host, "/")+"/api/v4"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
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
			return nil, fmt.Errorf("gitlab: %s", errResp.Message)
		}
		return nil, fmt.Errorf("gitlab: HTTP %d", resp.StatusCode)
	}

	return respBody, nil
}

func getGitLabUsername(token, host string) (string, error) {
	body, err := gitlabRequest("GET", "/user", token, host, nil)
	if err != nil {
		return "", err
	}
	var u struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	return u.Username, nil
}

func getGitLabGroups(token, host string) ([]string, error) {
	body, err := gitlabRequest("GET", "/groups", token, host, nil)
	if err != nil {
		return nil, err
	}
	var groups []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		return nil, err
	}
	var names []string
	for _, g := range groups {
		names = append(names, g.Path)
	}
	return names, nil
}

func createGitLabRepo(token, host, name string, private bool) (repoResponse, error) {
	visibility := "public"
	if private {
		visibility = "private"
	}
	payload := map[string]interface{}{
		"name":       name,
		"visibility": visibility,
	}
	body, err := gitlabRequest("POST", "/projects", token, host, payload)
	if err != nil {
		return repoResponse{}, err
	}
	var r struct {
		SSHURLToRepo  string `json:"ssh_url_to_repo"`
		HTTPURLToRepo string `json:"http_url_to_repo"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return repoResponse{}, err
	}
	return repoResponse{SSHURL: r.SSHURLToRepo, HTMLURL: r.HTTPURLToRepo, Name: r.Name}, nil
}

func getGitLabGroupID(token, host, group string) (int64, error) {
	body, err := gitlabRequest("GET", "/groups/"+group, token, host, nil)
	if err != nil {
		return 0, err
	}
	var g struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &g); err != nil {
		return 0, err
	}
	return g.ID, nil
}

func createGitLabGroupRepo(token, host, group, name string, private bool) (repoResponse, error) {
	if !isSafeGitRef(group) {
		return repoResponse{}, fmt.Errorf("invalid group name")
	}
	namespaceID, err := getGitLabGroupID(token, host, group)
	if err != nil {
		return repoResponse{}, fmt.Errorf("could not find group %s: %v", group, err)
	}
	visibility := "public"
	if private {
		visibility = "private"
	}
	payload := map[string]interface{}{
		"name":         name,
		"visibility":   visibility,
		"namespace_id": namespaceID,
	}
	body, err := gitlabRequest("POST", "/projects", token, host, payload)
	if err != nil {
		return repoResponse{}, err
	}
	var r struct {
		SSHURLToRepo  string `json:"ssh_url_to_repo"`
		HTTPURLToRepo string `json:"http_url_to_repo"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return repoResponse{}, err
	}
	return repoResponse{SSHURL: r.SSHURLToRepo, HTMLURL: r.HTTPURLToRepo, Name: r.Name}, nil
}

func getGitLabProjectID(token, host, owner, repo string) (string, error) {
	path := strings.ReplaceAll(owner+"/"+repo, "/", "%2F")
	body, err := gitlabRequest("GET", "/projects/"+path, token, host, nil)
	if err != nil {
		return "", err
	}
	var p struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", p.ID), nil
}

func uploadGitLabPackage(token, host, projectID, pkgName, version, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", err
	}

	fileName := filepath.Base(filePath)
	apiPath := fmt.Sprintf("/projects/%s/packages/generic/%s/%s/%s", projectID, pkgName, version, fileName)
	url := strings.TrimRight(host, "/") + "/api/v4" + apiPath

	req, err := http.NewRequest("PUT", url, f)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "commitdog/"+version)
	req.ContentLength = stat.Size()

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gitlab package upload failed: HTTP %d — %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	packageURL := strings.TrimRight(host, "/") + "/api/v4" + apiPath
	return packageURL, nil
}

func createGitLabRelease(token, host, projectID, tagName, changelog string) error {
	payload := map[string]interface{}{
		"tag_name":    "v" + tagName,
		"name":        "v" + tagName,
		"description": changelog,
	}
	_, err := gitlabRequest("POST", "/projects/"+projectID+"/releases", token, host, payload)
	return err
}

func addGitLabReleaseLink(token, host, projectID, tagName, name, url string) error {
	payload := map[string]interface{}{
		"name":      name,
		"url":       url,
		"link_type": "package",
	}
	_, err := gitlabRequest("POST",
		fmt.Sprintf("/projects/%s/releases/v%s/assets/links", projectID, tagName),
		token, host, payload,
	)
	return err
}

type gitlabMREntry struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	WebURL       string `json:"web_url"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Author       struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (m gitlabMREntry) toPREntry() prEntry {
	p := prEntry{
		Number:  m.IID,
		Title:   m.Title,
		HTMLURL: m.WebURL,
	}
	p.Head.Ref = m.SourceBranch
	p.Base.Ref = m.TargetBranch
	p.User.Login = m.Author.Username
	return p
}

func listGitLabMRs(token, host, projectID string) ([]prEntry, error) {
	body, err := gitlabRequest("GET", "/projects/"+projectID+"/merge_requests?state=opened&per_page=20", token, host, nil)
	if err != nil {
		return nil, err
	}
	var mrs []gitlabMREntry
	if err := json.Unmarshal(body, &mrs); err != nil {
		return nil, err
	}
	prs := make([]prEntry, len(mrs))
	for i, m := range mrs {
		prs[i] = m.toPREntry()
	}
	return prs, nil
}

func createGitLabMR(token, host, projectID, title, desc, source, target string) (prEntry, error) {
	payload := map[string]string{
		"source_branch": source,
		"target_branch": target,
		"title":         title,
		"description":   desc,
	}
	resp, err := gitlabRequest("POST", "/projects/"+projectID+"/merge_requests", token, host, payload)
	if err != nil {
		return prEntry{}, err
	}
	var mr gitlabMREntry
	if err := json.Unmarshal(resp, &mr); err != nil {
		return prEntry{}, err
	}
	return mr.toPREntry(), nil
}

func mergeGitLabMR(token, host, projectID string, iid int, method string) error {
	iidStr := fmt.Sprintf("%d", iid)
	if method == "rebase" {
		_, err := gitlabRequest("PUT", "/projects/"+projectID+"/merge_requests/"+iidStr+"/rebase", token, host, nil)
		return err
	}
	payload := map[string]interface{}{}
	if method == "squash" {
		payload["squash"] = true
	}
	_, err := gitlabRequest("PUT", "/projects/"+projectID+"/merge_requests/"+iidStr+"/merge", token, host, payload)
	return err
}

func deleteGitLabBranch(token, host, projectID, branch string) error {
	_, err := gitlabRequest("DELETE", "/projects/"+projectID+"/repository/branches/"+branch, token, host, nil)
	return err
}
