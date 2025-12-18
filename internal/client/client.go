package client

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Client is a minimal HTTP API client for the ghh server.
type Client struct {
	BaseURL  string
	Token    string
	User     string
	http     *http.Client
	Endpoint Endpoints
}

// NewClient creates a new API client.
func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, http: httpClient, Endpoint: DefaultEndpoints()}
}

// HTTPError wraps non-2xx responses.
type HTTPError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("http %d: %s", e.StatusCode, e.Message) }

// Download downloads repository code as an archive from the server.
// zipPath: where to save the zip file (always saved)
// extractDir: if non-empty, extract the zip to this directory after download
// Expected server endpoint: GET /api/v1/download?repo=<>&branch=<>
func (c *Client) Download(ctx context.Context, repo, branch, zipPath, extractDir string) error {
	q := url.Values{}
	if !strings.Contains(c.Endpoint.Download, "{repo}") {
		q.Set("repo", repo)
	}
	if strings.TrimSpace(branch) != "" && !strings.Contains(c.Endpoint.Download, "{branch}") {
		q.Set("branch", branch)
	}
	path := replacePlaceholders(c.Endpoint.Download, map[string]string{"repo": repo, "branch": branch, "path": ""})
	endpoint := c.fullURL(path, q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.addAuth(req)
	req.Header.Set("Accept", "application/zip, application/octet-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &HTTPError{StatusCode: resp.StatusCode, Message: "download failed", Body: string(body)}
	}
	commit := strings.TrimSpace(resp.Header.Get("X-GHH-Commit"))

	// Remove existing zip file if it exists
	if err := os.RemoveAll(zipPath); err != nil {
		return fmt.Errorf("remove existing zip: %w", err)
	}

	// Always save the zip file first
	if err := writeStreamToFile(zipPath, resp.Body); err != nil {
		return err
	}
	fmt.Printf("saved archive to %s\n", zipPath)

	// If extractDir is specified, extract the zip
	if extractDir != "" {
		f, err := os.Open(zipPath)
		if err != nil {
			return fmt.Errorf("open zip for extract: %w", err)
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			return fmt.Errorf("stat zip: %w", err)
		}

		if err := extractZip(f, fi.Size(), extractDir); err != nil {
			return fmt.Errorf("extract: %w", err)
		}
		fmt.Printf("extracted to %s\n", extractDir)
	}

	commitPath := ""
	if extractDir != "" {
		commitPath = filepath.Join(extractDir, "commit.txt")
	} else {
		commitPath = zipPath + ".commit.txt"
	}
	if commit != "" {
		if err := os.WriteFile(commitPath, []byte(commit+"\n"), 0o644); err != nil {
			fmt.Printf("warning: failed to save commit info to %s: %v\n", commitPath, err)
		} else {
			fmt.Printf("saved commit to %s\n", commitPath)
		}
	} else {
		commitFetched := c.fetchCommit(ctx, repo, branch)
		if commitFetched != "" {
			if err := os.WriteFile(commitPath, []byte(commitFetched+"\n"), 0o644); err != nil {
				fmt.Printf("warning: failed to save commit info to %s: %v\n", commitPath, err)
			} else {
				fmt.Printf("saved commit to %s\n", commitPath)
			}
		} else {
			fmt.Println("warning: commit info not provided by server")
		}
	}

	return nil
}

func (c *Client) fetchCommit(ctx context.Context, repo, branch string) string {
	q := url.Values{}
	if !strings.Contains(c.Endpoint.DownloadCommit, "{repo}") {
		q.Set("repo", repo)
	}
	if strings.TrimSpace(branch) != "" && !strings.Contains(c.Endpoint.DownloadCommit, "{branch}") {
		q.Set("branch", branch)
	}
	path := replacePlaceholders(c.Endpoint.DownloadCommit, map[string]string{"repo": repo, "branch": branch, "path": ""})
	endpoint := c.fullURL(path, q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ""
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SwitchBranch requests a branch switch on the server for the given repo.
// Expected server endpoint default: POST /api/v1/branch/switch {repo, branch}
func (c *Client) SwitchBranch(ctx context.Context, repo, branch string) error {
	payload := map[string]string{"repo": repo, "branch": branch}
	body, _ := json.Marshal(payload)
	path := replacePlaceholders(c.Endpoint.BranchSwitch, map[string]string{"repo": repo, "branch": branch})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.addAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &HTTPError{StatusCode: resp.StatusCode, Message: "switch branch failed", Body: string(b)}
	}
	fmt.Println("branch switched")
	return nil
}

// ListDir lists a directory on the server.
// Expected server endpoint default: GET /api/v1/dir/list?path=<path>
func (c *Client) ListDir(ctx context.Context, path string, raw bool) error {
	q := url.Values{}
	p := c.Endpoint.DirList
	if strings.Contains(p, "{path}") {
		p = replacePlaceholders(p, map[string]string{"path": path})
	} else {
		q.Set("path", path)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.fullURL(p, q), nil)
	if err != nil {
		return err
	}
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Message: "list failed", Body: string(b)}
	}
	if raw {
		fmt.Println(string(b))
		return nil
	}
	// Try to pretty print into a simple table if JSON is compatible
	var entries []struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
	}
	if err := json.Unmarshal(b, &entries); err != nil {
		// fallback to raw
		fmt.Println(string(b))
		return nil
	}
	for _, e := range entries {
		typ := "file"
		if e.IsDir {
			typ = "dir"
		}
		fmt.Printf("%-4s %10d  %s\n", typ, e.Size, nonEmpty(e.Path, e.Name))
	}
	return nil
}

// DeleteDir deletes a directory on the server.
// Expected server endpoint default: DELETE /api/v1/dir?path=<path>&recursive=true
func (c *Client) DeleteDir(ctx context.Context, path string, recursive bool) error {
	q := url.Values{}
	p := c.Endpoint.DirDelete
	if strings.Contains(p, "{path}") {
		p = replacePlaceholders(p, map[string]string{"path": path})
	} else {
		q.Set("path", path)
	}
	if recursive {
		q.Set("recursive", "true")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.fullURL(p, q), nil)
	if err != nil {
		return err
	}
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &HTTPError{StatusCode: resp.StatusCode, Message: "delete failed", Body: string(b)}
	}
	fmt.Println("deleted")
	return nil
}

func (c *Client) addAuth(req *http.Request) {
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if strings.TrimSpace(c.User) != "" {
		req.Header.Set("X-GHH-User", c.User)
	}
}

// Endpoints provides API path templates.
type Endpoints struct {
	Download       string
	DownloadCommit string
	BranchSwitch   string
	DirList        string
	DirDelete      string
}

func DefaultEndpoints() Endpoints {
	return Endpoints{
		Download:       "/api/v1/download",
		DownloadCommit: "/api/v1/download/commit",
		BranchSwitch:   "/api/v1/branch/switch",
		DirList:        "/api/v1/dir/list",
		DirDelete:      "/api/v1/dir",
	}
}

func replacePlaceholders(tpl string, values map[string]string) string {
	out := tpl
	for k, v := range values {
		if v == "" {
			continue
		}
		out = strings.ReplaceAll(out, "{"+k+"}", url.PathEscape(v))
	}
	return out
}

func (c *Client) fullURL(path string, q url.Values) string {
	if len(q) == 0 {
		return c.BaseURL + path
	}
	return c.BaseURL + path + "?" + q.Encode()
}

func writeStreamToFile(dest string, r io.Reader) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return nil
}

func extractZip(r io.ReaderAt, size int64, dest string) error {
	if dest == "" {
		return errors.New("dest required for extract")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil { // create target dir
		return err
	}
	// Use absolute path for ZipSlip check
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve dest path: %w", err)
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		fp := filepath.Join(dest, f.Name)
		// Prevent ZipSlip using absolute paths
		absFp, err := filepath.Abs(fp)
		if err != nil {
			return fmt.Errorf("resolve file path: %w", err)
		}
		if !strings.HasPrefix(absFp, absDest+string(os.PathSeparator)) && absFp != absDest {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

func nonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
