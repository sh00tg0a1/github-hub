package client

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client is a minimal HTTP API client for the ghh server.
type Client struct {
	BaseURL          string
	Token            string
	User             string
	DebugDelay       string // DEBUG: request server to add artificial delay (e.g., "90s", "2m")
	DebugStreamDelay string // DEBUG: request server to slow streaming (e.g., "90s", "2m")
	RetryMax         int
	RetryBackoff     time.Duration
	ProgressInterval time.Duration
	http             *http.Client
	Endpoint         Endpoints
}

// NewClient creates a new API client.
func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		BaseURL:          strings.TrimRight(baseURL, "/"),
		Token:            token,
		http:             httpClient,
		Endpoint:         DefaultEndpoints(),
		RetryMax:         5,
		RetryBackoff:     2 * time.Second,
		ProgressInterval: time.Second,
	}
}

// HTTPError wraps non-2xx responses.
type HTTPError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("http %d: %s", e.StatusCode, e.Message) }

// DownloadPackage downloads a release/package file by URL with server-side caching keyed by URL hash.
func (c *Client) DownloadPackage(ctx context.Context, pkgURL, destPath string) error {
	q := url.Values{}
	if !strings.Contains(c.Endpoint.DownloadPackage, "{url}") {
		q.Set("url", pkgURL)
	}
	if strings.TrimSpace(c.DebugStreamDelay) != "" {
		q.Set("debug_stream_delay", c.DebugStreamDelay)
	}
	path := replacePlaceholders(c.Endpoint.DownloadPackage, map[string]string{"url": pkgURL, "path": ""})
	endpoint := c.fullURL(path, q)
	reqBuilder := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.addAuth(req)
		req.Header.Set("Accept", "application/octet-stream")
		return req, nil
	}
	label := fmt.Sprintf("package %s", filepath.Base(destPath))
	fmt.Printf("downloading %s ...\n", label)
	if _, err := c.downloadToFileWithRetry(ctx, destPath, label, reqBuilder); err != nil {
		return err
	}
	fmt.Printf("saved package to %s\n", destPath)
	return nil
}

// Download downloads repository code as an archive from the server.
// zipPath: where to save the zip file (always saved)
// extractDir: if non-empty, extract the zip to this directory after download
// Expected server endpoint: GET /api/v1/download?repo=<>&branch=<>
func (c *Client) Download(ctx context.Context, repo, branch, zipPath, extractDir string) error {
	startTime := time.Now()

	q := url.Values{}
	if !strings.Contains(c.Endpoint.Download, "{repo}") {
		q.Set("repo", repo)
	}
	if strings.TrimSpace(branch) != "" && !strings.Contains(c.Endpoint.Download, "{branch}") {
		q.Set("branch", branch)
	}
	if strings.TrimSpace(c.DebugDelay) != "" {
		q.Set("debug_delay", c.DebugDelay)
	}
	if strings.TrimSpace(c.DebugStreamDelay) != "" {
		q.Set("debug_stream_delay", c.DebugStreamDelay)
	}
	path := replacePlaceholders(c.Endpoint.Download, map[string]string{"repo": repo, "branch": branch, "path": ""})
	endpoint := c.fullURL(path, q)
	reqBuilder := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.addAuth(req)
		req.Header.Set("Accept", "application/zip, application/octet-stream")
		return req, nil
	}
	fmt.Printf("downloading %s ...\n", repo)
	headers, err := c.downloadToFileWithRetry(ctx, zipPath, "repo "+repo, reqBuilder)
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(headers.Get("X-GHH-Commit"))
	elapsed := time.Since(startTime)
	fi, _ := os.Stat(zipPath)
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	fmt.Printf("saved archive to %s (%.2f MB, %s)\n", zipPath, float64(size)/(1024*1024), elapsed.Round(time.Millisecond))

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

// DownloadSparse downloads selected paths from a repository using sparse checkout.
// paths: list of directory/file prefixes to include
// zipPath: where to save the zip file
// extractDir: if non-empty, extract the zip to this directory after download
func (c *Client) DownloadSparse(ctx context.Context, repo, branch string, paths []string, zipPath, extractDir string) error {
	startTime := time.Now()

	q := url.Values{}
	q.Set("repo", repo)
	if strings.TrimSpace(branch) != "" {
		q.Set("branch", branch)
	}
	q.Set("paths", strings.Join(paths, ","))

	endpoint := c.fullURL(c.Endpoint.DownloadSparse, q)
	reqBuilder := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.addAuth(req)
		req.Header.Set("Accept", "application/zip, application/octet-stream")
		return req, nil
	}
	label := fmt.Sprintf("sparse %s [%s]", repo, strings.Join(paths, ","))
	fmt.Printf("downloading %s ...\n", label)
	headers, err := c.downloadToFileWithRetry(ctx, zipPath, label, reqBuilder)
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(headers.Get("X-GHH-Commit"))
	elapsed := time.Since(startTime)
	fi, _ := os.Stat(zipPath)
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	fmt.Printf("saved sparse archive to %s (%.2f MB, %s)\n", zipPath, float64(size)/(1024*1024), elapsed.Round(time.Millisecond))

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

	// Write commit.txt
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
	Download        string
	DownloadCommit  string
	DownloadSparse  string
	BranchSwitch    string
	DirList         string
	DirDelete       string
	ServerVersion   string
	DownloadPackage string
}

func DefaultEndpoints() Endpoints {
	return Endpoints{
		Download:        "/api/v1/download",
		DownloadCommit:  "/api/v1/download/commit",
		DownloadSparse:  "/api/v1/download/sparse",
		BranchSwitch:    "/api/v1/branch/switch",
		DirList:         "/api/v1/dir/list",
		DirDelete:       "/api/v1/dir",
		ServerVersion:   "/api/v1/version",
		DownloadPackage: "/api/v1/download/package",
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

func (c *Client) downloadToFileWithRetry(ctx context.Context, destPath, label string, reqBuilder func(context.Context) (*http.Request, error)) (http.Header, error) {
	attempts := c.retryAttempts()
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := sleepWithBackoff(ctx, c.retryBackoff(), attempt); err != nil {
				return nil, err
			}
		}
		req, err := reqBuilder(ctx)
		if err != nil {
			return nil, err
		}
		waitStop := make(chan struct{})
		var waitPrinted int32
		started := time.Now()
		go func() {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			for {
				select {
				case <-waitStop:
					return
				case <-timer.C:
					atomic.StoreInt32(&waitPrinted, 1)
					printInline(fmt.Sprintf("waiting for server... %s", time.Since(started).Round(time.Second)), false)
					timer.Reset(time.Second)
				}
			}
		}()
		resp, err := c.http.Do(req)
		close(waitStop)
		if atomic.LoadInt32(&waitPrinted) == 1 {
			clearInline()
		}
		if err != nil {
			lastErr = err
			if attempt == attempts-1 || !isRetryableError(err) {
				return nil, err
			}
			printRetry(attempt, attempts, err)
			continue
		}
		headers := resp.Header.Clone()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			err := &HTTPError{StatusCode: resp.StatusCode, Message: "download failed", Body: string(body)}
			lastErr = err
			if attempt == attempts-1 || !isRetryableStatus(resp.StatusCode) {
				return nil, err
			}
			printRetry(attempt, attempts, err)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			resp.Body.Close()
			return nil, err
		}
		tmpFile, err := os.CreateTemp(filepath.Dir(destPath), ".tmp-download-*")
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		err = c.copyWithProgress(ctx, tmpPath, resp.Body, resp.ContentLength, label)
		resp.Body.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
			lastErr = err
			if attempt == attempts-1 || !isRetryableError(err) {
				return nil, err
			}
			printRetry(attempt, attempts, err)
			continue
		}
		_ = os.Remove(destPath)
		if err := os.Rename(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			return nil, err
		}
		return headers, nil
	}
	return nil, lastErr
}

func (c *Client) copyWithProgress(ctx context.Context, dest string, r io.Reader, total int64, label string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	var written int64
	start := time.Now()
	interval := c.progressInterval()
	if label == "" {
		label = "download"
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ticker.C:
				printProgress(label, atomic.LoadInt64(&written), total, start, false)
			case <-done:
				printProgress(label, atomic.LoadInt64(&written), total, start, true)
				return
			}
		}
	}()

	cr := &countingReader{r: r, ctx: ctx, written: &written}
	_, err = io.Copy(f, cr)
	close(done)
	wg.Wait()
	return err
}

func (c *Client) retryAttempts() int {
	if c.RetryMax < 0 {
		return 1
	}
	return c.RetryMax + 1
}

func (c *Client) retryBackoff() time.Duration {
	if c.RetryBackoff <= 0 {
		return 2 * time.Second
	}
	return c.RetryBackoff
}

func (c *Client) progressInterval() time.Duration {
	if c.ProgressInterval <= 0 {
		return time.Second
	}
	return c.ProgressInterval
}

type countingReader struct {
	r       io.Reader
	ctx     context.Context
	written *int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	if cr.ctx != nil {
		select {
		case <-cr.ctx.Done():
			return 0, cr.ctx.Err()
		default:
		}
	}
	n, err := cr.r.Read(p)
	if n > 0 {
		atomic.AddInt64(cr.written, int64(n))
	}
	return n, err
}

func printRetry(attempt, attempts int, err error) {
	next := attempt + 1
	if next < attempts {
		fmt.Printf("download failed: %v, retrying (%d/%d)\n", err, next, attempts-1)
	}
}

func printProgress(label string, written, total int64, start time.Time, final bool) {
	elapsed := time.Since(start)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	speed := float64(written) / elapsed.Seconds()
	msg := ""
	if total > 0 {
		percent := float64(written) / float64(total) * 100
		if percent > 100 {
			percent = 100
		}
		msg = fmt.Sprintf("%s  %s %s  %3.0f%%  %s/s", label, progressBar(percent, 28), formatBytes(written), percent, formatBytes(int64(speed)))
	} else {
		msg = fmt.Sprintf("%s  %s  %s/s", label, formatBytes(written), formatBytes(int64(speed)))
	}
	printInline(msg, final)
}

var progressLastLen int32

func printInline(msg string, final bool) {
	prev := int(atomic.LoadInt32(&progressLastLen))
	if len(msg) < prev {
		msg += strings.Repeat(" ", prev-len(msg))
	}
	fmt.Printf("\r%s", msg)
	atomic.StoreInt32(&progressLastLen, int32(len(msg)))
	if final {
		fmt.Print("\n")
		atomic.StoreInt32(&progressLastLen, 0)
	}
}

func clearInline() {
	prev := int(atomic.LoadInt32(&progressLastLen))
	if prev > 0 {
		fmt.Printf("\r%s\r", strings.Repeat(" ", prev))
		atomic.StoreInt32(&progressLastLen, 0)
	}
}

func progressBar(percent float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(math.Round(percent / 100 * float64(width)))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
	return "[" + bar + "]"
}

func formatBytes(n int64) string {
	const unit = 1024.0
	if n < int64(unit) {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	suffixes := []string{"KB", "MB", "GB", "TB"}
	exp := 0
	for value >= unit && exp < len(suffixes) {
		value /= unit
		exp++
	}
	if exp == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", value, suffixes[exp-1])
}

func sleepWithBackoff(ctx context.Context, base time.Duration, attempt int) error {
	backoff := base * time.Duration(attempt)
	if backoff <= 0 {
		return nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryableStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusTooManyRequests ||
		status >= 500
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var nerr net.Error
	if errors.As(err, &nerr) {
		if nerr.Timeout() || nerr.Temporary() {
			return true
		}
	}
	return true
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
