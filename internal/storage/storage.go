package storage

import (
	"archive/tar"
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrBadPath  = errors.New("bad path")
	ErrNotFound = errors.New("not found")
)

type Storage struct {
	Root            string
	HTTPClient      *http.Client
	DebugSlowReader time.Duration // DEBUG: delay per read chunk to simulate slow network
	RetryMax        int
	RetryBackoff    time.Duration

	mu      sync.Mutex
	lock    map[string]*sync.Mutex
	rwLock  map[string]*sync.RWMutex // for git cache read/write locks
}

func sanitizeName(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, "\\", "-")
	v = strings.ReplaceAll(v, "/", "-")
	return v
}

// PackageHash returns a short hash for a package URL.
func PackageHash(pkgURL string) string {
	hash := sha256.Sum256([]byte(pkgURL))
	hashStr := hex.EncodeToString(hash[:])
	return hashStr[:20] // use first 20 hex chars to reduce collision risk
}

// EnsurePackage caches a package archive downloaded from pkgURL under:
// <root>/users/<user>/packages/<url-hash>/<filename>
func (s *Storage) EnsurePackage(ctx context.Context, user, pkgURL string) (string, error) {
	user = sanitizeName(strings.Trim(user, "/ "))
	if user == "" {
		user = "default"
	}
	if user == "." || strings.Contains(user, "..") {
		return "", fmt.Errorf("invalid user: %w", ErrBadPath)
	}

	u, _ := url.Parse(pkgURL)
	filename := ""
	if u != nil {
		filename = filepath.Base(u.Path)
	}
	if filename == "" || filename == "." || filename == "/" {
		filename = filepath.Base(pkgURL)
	}
	if filename == "" || filename == "." || filename == "/" {
		filename = "package.bin"
	}
	name := filename
	if idx := strings.Index(name, "."); idx > 0 {
		name = name[:idx]
	}
	name = sanitizeName(name)

	hashStr := PackageHash(pkgURL)

	pkgDir := filepath.Join(s.Root, "users", user, "packages", hashStr)
	pkgPath := filepath.Join(pkgDir, filename)

	// If exists, reuse
	if info, err := os.Stat(pkgPath); err == nil && !info.IsDir() {
		_ = s.touch(pkgPath)
		return pkgPath, nil
	}

	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return "", err
	}
	tmpFile, err := os.CreateTemp(pkgDir, ".tmp-package-*.bin")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if err := s.downloadFile(ctx, pkgURL, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	_ = os.Remove(pkgPath)
	if err := os.Rename(tmpPath, pkgPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	_ = s.touch(pkgPath)
	return pkgPath, nil
}

// New creates a Storage with a default HTTP client (no timeout, relies on context).
func New(root string) *Storage {
	return NewWithTimeout(root, 0)
}

// NewWithTimeout creates a Storage with an HTTP client configured with the given timeout.
// If timeout <= 0, no client-level timeout is set (relies on context timeout).
func NewWithTimeout(root string, timeout time.Duration) *Storage {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{Transport: transport}
	if timeout > 0 {
		client.Timeout = timeout
	}
	return &Storage{
		Root:         root,
		HTTPClient:   client,
		RetryMax:     5,
		RetryBackoff: 2 * time.Second,
	}
}

func (s *Storage) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return http.DefaultClient
}

// EnsureRepo ensures a cached repo (owner/repo) at branch exists under workspace.
// If missing, it downloads from GitHub zipball and extracts into
//
//	<root>/users/<user>/repos/<owner>/<repo>/<branch>.zip
//
// If branch is empty, fetches the default branch from GitHub API.
// If force is true, bypasses cache validation and always downloads fresh.
func (s *Storage) EnsureRepo(ctx context.Context, user, ownerRepo, branch, token string, force bool) (string, error) {
	user = strings.Trim(user, "/ ")
	if user == "" {
		user = "default"
	}
	if strings.ContainsRune(user, '/') || strings.ContainsRune(user, '\\') {
		return "", fmt.Errorf("invalid user: %w", ErrBadPath)
	}
	user = sanitizeName(user)
	ownerRepo = strings.Trim(ownerRepo, "/")
	if ownerRepo == "" || strings.Count(ownerRepo, "/") != 1 {
		return "", fmt.Errorf("owner/repo expected: %w", ErrBadPath)
	}
	// If branch not specified, fetch the default branch from GitHub
	if branch == "" {
		defaultBranch, err := s.fetchDefaultBranch(ctx, ownerRepo, token)
		if err != nil {
			return "", fmt.Errorf("fetch default branch: %w", err)
		}
		fmt.Printf("resolved default branch for %s: %s\n", ownerRepo, defaultBranch)
		branch = defaultBranch
	}
	zipPath := filepath.Join(s.Root, "users", user, "repos", ownerRepo, branch+".zip")
	metaPath := zipPath + ".meta"
	unlock := s.acquire(user, ownerRepo, branch)
	defer unlock()

	remoteSHA, fetchErr := s.fetchBranchSHA(ctx, ownerRepo, branch, token)

	parent := filepath.Dir(zipPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}

	// If we have cache and sha matches, reuse (unless force refresh requested).
	if !force {
		if info, err := os.Stat(zipPath); err == nil && !info.IsDir() {
			if remoteSHA != "" {
				if cachedSHA, err := readSHA(metaPath); err == nil && cachedSHA == remoteSHA {
					_ = s.touch(zipPath)
					return zipPath, nil
				}
			} else if fetchErr != nil {
				// Cannot verify, force refresh
			}
		}
	}

	// Download fresh zip (to temp then replace).
	tmpFile, err := os.CreateTemp(parent, ".tmp-download-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if err := s.downloadZip(ctx, ownerRepo, branch, token, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	_ = os.Remove(zipPath)
	if err := os.Rename(tmpPath, zipPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	commitPath := strings.TrimSuffix(zipPath, ".zip") + ".commit.txt"
	if remoteSHA != "" {
		_ = writeSHA(metaPath, remoteSHA)
		short := remoteSHA
		if len(short) > 7 {
			short = short[:7]
		}
		_ = writeSHA(commitPath, short)
	} else {
		_ = os.Remove(metaPath)
		// 若无法获取远端 SHA，则保持已有 commit 文件（如果存在），不强删
	}
	_ = s.touch(zipPath)
	return zipPath, nil
}

// List lists entries under the given relative path.
func (s *Storage) List(rel string) ([]Entry, error) {
	abs, err := s.safeJoin(rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".meta") {
			continue
		}
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		p := filepath.ToSlash(filepath.Join(rel, e.Name()))
		result = append(result, Entry{
			Name:  e.Name(),
			Path:  p,
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	return result, nil
}

// Delete removes the relative path. If recursive is false and path is a directory, it must be empty.
func (s *Storage) Delete(rel string, recursive bool) error {
	abs, err := s.safeJoin(rel)
	if err != nil {
		return err
	}
	if recursive {
		return os.RemoveAll(abs)
	}
	return os.Remove(abs)
}

// Helpers
func (s *Storage) safeJoin(rel string) (string, error) {
	if rel == "" {
		rel = "."
	}
	cleaned := filepath.Clean(rel)
	abs := filepath.Join(s.Root, cleaned)
	abs = filepath.Clean(abs)
	rootClean := filepath.Clean(s.Root)
	if !strings.HasPrefix(abs+string(os.PathSeparator), rootClean+string(os.PathSeparator)) && abs != rootClean {
		return "", ErrBadPath
	}
	return abs, nil
}

// acquire returns an unlock func for a per-repo/branch key.
func (s *Storage) acquire(user, repo, branch string) func() {
	key := fmt.Sprintf("%s|%s|%s", user, repo, branch)
	s.mu.Lock()
	if s.lock == nil {
		s.lock = make(map[string]*sync.Mutex)
	}
	m, ok := s.lock[key]
	if !ok {
		m = &sync.Mutex{}
		s.lock[key] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// downloadZip downloads archive into the given path.
func (s *Storage) downloadZip(ctx context.Context, ownerRepo, branch, token, dest string) error {
	downloadURL := fmt.Sprintf("https://codeload.github.com/%s/zip/%s", ownerRepo, url.PathEscape(branch))
	reqBuilder := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(token) != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Accept", "application/zip")
		return req, nil
	}
	readerFn := func(resp *http.Response) io.Reader {
		if s.DebugSlowReader > 0 {
			fmt.Printf("DEBUG: simulating slow network, target download time %s for repo=%s (size=%d bytes)\n",
				s.DebugSlowReader, ownerRepo, resp.ContentLength)
			return newSlowReader(resp.Body, ctx, s.DebugSlowReader, resp.ContentLength)
		}
		return resp.Body
	}
	label := fmt.Sprintf("repo %s@%s", ownerRepo, branch)
	return s.downloadWithRetry(ctx, dest, label, reqBuilder, readerFn)
}

func (s *Storage) downloadFile(ctx context.Context, fileURL, dest string) error {
	reqBuilder := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
		if err != nil {
			return nil, err
		}
		return req, nil
	}
	label := fmt.Sprintf("package %s", filepath.Base(fileURL))
	return s.downloadWithRetry(ctx, dest, label, reqBuilder, func(resp *http.Response) io.Reader {
		return resp.Body
	})
}

func (s *Storage) downloadWithRetry(ctx context.Context, dest string, label string, reqBuilder func(context.Context) (*http.Request, error), readerFn func(*http.Response) io.Reader) error {
	attempts := s.retryAttempts()
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := sleepWithBackoff(ctx, s.retryBackoff(), attempt); err != nil {
				return err
			}
		}
		req, err := reqBuilder(ctx)
		if err != nil {
			return err
		}
		resp, err := s.httpClient().Do(req)
		if err != nil {
			lastErr = err
			if attempt == attempts-1 || !isRetryableError(err) {
				return err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			err := fmt.Errorf("download failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
			lastErr = err
			if attempt == attempts-1 || !isRetryableStatus(resp.StatusCode) {
				return err
			}
			continue
		}

		tmpFile, err := os.CreateTemp(filepath.Dir(dest), ".tmp-download-*")
		if err != nil {
			resp.Body.Close()
			return err
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()

		reader := readerFn(resp)
		out, err := os.Create(tmpPath)
		if err != nil {
			resp.Body.Close()
			_ = os.Remove(tmpPath)
			return err
		}
		var written int64
		start := time.Now()
		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func(total int64) {
			defer wg.Done()
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					printServerProgress(label, atomic.LoadInt64(&written), total, start, false)
				case <-done:
					printServerProgress(label, atomic.LoadInt64(&written), total, start, true)
					return
				}
			}
		}(resp.ContentLength)

		cr := &countingReader{r: reader, ctx: ctx, written: &written}
		_, err = io.Copy(out, cr)
		out.Close()
		resp.Body.Close()
		close(done)
		wg.Wait()
		if err != nil {
			_ = os.Remove(tmpPath)
			lastErr = err
			if attempt == attempts-1 || !isRetryableError(err) {
				return err
			}
			continue
		}
		_ = os.Remove(dest)
		if err := os.Rename(tmpPath, dest); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		return nil
	}
	return lastErr
}

func (s *Storage) retryAttempts() int {
	if s.RetryMax < 0 {
		return 1
	}
	return s.RetryMax + 1
}

func (s *Storage) retryBackoff() time.Duration {
	if s.RetryBackoff <= 0 {
		return 2 * time.Second
	}
	return s.RetryBackoff
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

func printServerProgress(label string, written, total int64, start time.Time, final bool) {
	if label == "" {
		label = "download"
	}
	elapsed := time.Since(start)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	speed := float64(written) / elapsed.Seconds()
	if total > 0 {
		percent := float64(written) / float64(total) * 100
		if percent > 100 {
			percent = 100
		}
		fmt.Printf("github download %s: %s/%s (%.1f%%) %s/s\n",
			label, formatBytes(written), formatBytes(total), percent, formatBytes(int64(speed)))
	} else {
		fmt.Printf("github download %s: %s %s/s\n",
			label, formatBytes(written), formatBytes(int64(speed)))
	}
	if final {
		fmt.Printf("github download %s: done\n", label)
	}
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

// Touch records access time for a relative path (file or directory). It ignores missing paths.
func (s *Storage) Touch(rel string) error {
	abs, err := s.safeJoin(rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err == nil {
		// Update timestamp for both files and directories
		return s.touch(abs)
	}
	return nil
}

func (s *Storage) touch(abs string) error {
	now := time.Now()
	return os.Chtimes(abs, now, now)
}

// CleanupExpired removes cached items unused beyond ttl.
// - Repos: users/<user>/repos/<owner>/<repo>/<branch>.zip (+.meta, commit)
// - Packages: users/<user>/packages/** (any file)
func (s *Storage) CleanupExpired(ttl time.Duration) error {
	cutoff := time.Now().Add(-ttl)
	root := filepath.Join(s.Root, "users")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore inaccessible
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(s.Root, path)
		parts := splitPath(rel)
		if len(parts) < 3 || parts[0] != "users" {
			return nil
		}

		switch parts[2] {
		case "repos":
			// expect users/<user>/repos/<owner>/<repo>/<branch>.zip
			if filepath.Ext(path) != ".zip" || len(parts) < 6 {
				return nil
			}
			if expired(path, cutoff) {
				_ = os.Remove(path)
				_ = os.Remove(path + ".meta")
				_ = os.Remove(strings.TrimSuffix(path, ".zip") + ".commit.txt")
				trimEmpty(filepath.Dir(path), filepath.Join(s.Root, "users"))
			}
		case "packages":
			// any package file under users/<user>/packages/**
			if expired(path, cutoff) {
				_ = os.Remove(path)
				trimEmpty(filepath.Dir(path), filepath.Join(s.Root, "users"))
			}
		default:
			return nil
		}
		return nil
	})
}

func expired(path string, cutoff time.Time) bool {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime().Before(cutoff)
	}
	return false
}

func trimEmpty(dir string, stop string) {
	for {
		if dir == stop || dir == "." || dir == string(filepath.Separator) {
			return
		}
		_ = os.Remove(dir)
		next := filepath.Dir(dir)
		if next == dir {
			return
		}
		dir = next
	}
}

func splitPath(p string) []string {
	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// fetchDefaultBranch retrieves the default branch name from GitHub API.
func (s *Storage) fetchDefaultBranch(ctx context.Context, ownerRepo, token string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s", ownerRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", fmt.Errorf("fetch repo info failed: %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if strings.TrimSpace(data.DefaultBranch) == "" {
		return "", fmt.Errorf("empty default branch")
	}
	return data.DefaultBranch, nil
}

func (s *Storage) fetchBranchSHA(ctx context.Context, ownerRepo, branch, token string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("branch unspecified")
	}
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid owner/repo")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/branches/%s", ownerRepo, url.PathEscape(branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", fmt.Errorf("branch sha failed: status=%d body=%s", resp.StatusCode, string(b))
	}
	var data struct {
		Commit struct {
			Sha string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if strings.TrimSpace(data.Commit.Sha) == "" {
		return "", fmt.Errorf("empty sha")
	}
	return data.Commit.Sha, nil
}

func readSHA(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func writeSHA(path, sha string) error {
	return os.WriteFile(path, []byte(strings.TrimSpace(sha)), 0o644)
}

type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// slowReader wraps an io.Reader to simulate slow network by stretching download to target duration.
type slowReader struct {
	r             io.Reader
	ctx           context.Context
	totalDuration time.Duration // target total download time
	contentLength int64         // expected total bytes (-1 if unknown)
	startTime     time.Time
	bytesRead     int64
	readCount     int
}

func newSlowReader(r io.Reader, ctx context.Context, totalDuration time.Duration, contentLength int64) *slowReader {
	return &slowReader{
		r:             r,
		ctx:           ctx,
		totalDuration: totalDuration,
		contentLength: contentLength,
		startTime:     time.Now(),
	}
}

func (sr *slowReader) Read(p []byte) (n int, err error) {
	// Check context before reading
	select {
	case <-sr.ctx.Done():
		return 0, sr.ctx.Err()
	default:
	}

	n, err = sr.r.Read(p)
	if n > 0 {
		sr.bytesRead += int64(n)
		sr.readCount++
	}
	if err != nil {
		return n, err
	}

	if sr.totalDuration > 0 && n > 0 {
		var sleepTime time.Duration

		if sr.contentLength > 0 {
			// Content-Length known: calculate based on progress
			progress := float64(sr.bytesRead) / float64(sr.contentLength)
			expectedElapsed := time.Duration(float64(sr.totalDuration) * progress)
			actualElapsed := time.Since(sr.startTime)
			if expectedElapsed > actualElapsed {
				sleepTime = expectedElapsed - actualElapsed
			}
		} else {
			// Content-Length unknown: use fixed delay per chunk
			// Assume ~2000 reads for a typical repo (~70MB), spread totalDuration across reads
			delayPerRead := sr.totalDuration / 2000
			if delayPerRead < time.Millisecond {
				delayPerRead = time.Millisecond
			}
			sleepTime = delayPerRead
		}

		if sleepTime > 0 {
			select {
			case <-time.After(sleepTime):
			case <-sr.ctx.Done():
				return n, sr.ctx.Err()
			}
		}
	}
	return n, err
}

// ========== Sparse Checkout Support ==========

// acquireGitCacheWrite returns an unlock func for a per-repo git cache write lock.
// Use this for operations that modify the bare repo (clone, fetch).
func (s *Storage) acquireGitCacheWrite(ownerRepo string) func() {
	key := "git-cache|" + ownerRepo
	s.mu.Lock()
	if s.rwLock == nil {
		s.rwLock = make(map[string]*sync.RWMutex)
	}
	m, ok := s.rwLock[key]
	if !ok {
		m = &sync.RWMutex{}
		s.rwLock[key] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// acquireGitCacheRead returns an unlock func for a per-repo git cache read lock.
// Use this for operations that only read from the bare repo (export).
// Multiple readers can hold the lock simultaneously.
func (s *Storage) acquireGitCacheRead(ownerRepo string) func() {
	key := "git-cache|" + ownerRepo
	s.mu.Lock()
	if s.rwLock == nil {
		s.rwLock = make(map[string]*sync.RWMutex)
	}
	m, ok := s.rwLock[key]
	if !ok {
		m = &sync.RWMutex{}
		s.rwLock[key] = m
	}
	s.mu.Unlock()
	m.RLock()
	return m.RUnlock
}

// gitCachePath returns the path to the bare repo cache for a given owner/repo.
// Layout: <root>/git-cache/<owner>/<repo>.git
func (s *Storage) gitCachePath(ownerRepo string) string {
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 {
		return filepath.Join(s.Root, "git-cache", sanitizeName(ownerRepo)+".git")
	}
	return filepath.Join(s.Root, "git-cache", parts[0], parts[1]+".git")
}

// EnsureBareRepo ensures a bare repo cache exists and is up-to-date.
// If missing, clones from GitHub. Otherwise, fetches updates.
// Returns the path to the bare repo.
func (s *Storage) EnsureBareRepo(ctx context.Context, ownerRepo, token string) (string, error) {
	ownerRepo = strings.Trim(ownerRepo, "/")
	if ownerRepo == "" || strings.Count(ownerRepo, "/") != 1 {
		return "", fmt.Errorf("owner/repo expected: %w", ErrBadPath)
	}

	unlock := s.acquireGitCacheWrite(ownerRepo)
	defer unlock()

	barePath := s.gitCachePath(ownerRepo)

	// Build the remote URL with optional token
	remoteURL := fmt.Sprintf("https://github.com/%s.git", ownerRepo)
	if strings.TrimSpace(token) != "" {
		remoteURL = fmt.Sprintf("https://%s@github.com/%s.git", token, ownerRepo)
	}

	// Check if bare repo exists
	if _, err := os.Stat(filepath.Join(barePath, "HEAD")); err == nil {
		// Bare repo exists, fetch updates
		fmt.Printf("fetching updates for %s...\n", ownerRepo)
		cmd := exec.CommandContext(ctx, "git", "-C", barePath, "fetch", "--prune", "origin")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git fetch failed: %w", err)
		}
	} else {
		// Clone bare repo
		fmt.Printf("cloning bare repo for %s...\n", ownerRepo)
		if err := os.MkdirAll(filepath.Dir(barePath), 0o755); err != nil {
			return "", err
		}
		cmd := exec.CommandContext(ctx, "git", "clone", "--bare", remoteURL, barePath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git clone --bare failed: %w", err)
		}
	}

	return barePath, nil
}

// ExportSparseZip exports selected paths from a branch to a zip file using git archive.
// paths: list of directory/file prefixes to include
// Returns the commit SHA.
func (s *Storage) ExportSparseZip(ctx context.Context, ownerRepo, branch string, paths []string, destZip string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("paths cannot be empty")
	}
	for _, p := range paths {
		if strings.Contains(p, "..") || filepath.IsAbs(p) {
			return "", fmt.Errorf("invalid path %q: %w", p, ErrBadPath)
		}
	}

	// Acquire read lock to prevent concurrent fetch from modifying the bare repo
	unlock := s.acquireGitCacheRead(ownerRepo)
	defer unlock()

	barePath := s.gitCachePath(ownerRepo)
	if _, err := os.Stat(filepath.Join(barePath, "HEAD")); err != nil {
		return "", fmt.Errorf("bare repo not found, call EnsureBareRepo first")
	}

	// Resolve branch to ref
	refName := "origin/" + branch
	commitSHA, err := s.gitRevParse(ctx, barePath, refName)
	if err != nil {
		return "", fmt.Errorf("resolve branch %q: %w", branch, err)
	}

	fmt.Printf("exporting %s@%s paths %v via git archive...\n", ownerRepo, branch, paths)

	// Use git archive to directly create zip - much faster than worktree+sparse-checkout
	// git archive --format=zip --output=<dest> <commit> -- path1 path2 ...
	args := []string{"-C", barePath, "archive", "--format=zip", "--output=" + destZip, commitSHA, "--"}
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git archive failed: %w", err)
	}

	// Return short commit SHA
	shortSHA := commitSHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	return shortSHA, nil
}

// ExportSparseDir exports selected paths from a branch to a directory using git archive.
// Returns the commit SHA.
func (s *Storage) ExportSparseDir(ctx context.Context, ownerRepo, branch string, paths []string, destDir string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("paths cannot be empty")
	}
	for _, p := range paths {
		if strings.Contains(p, "..") || filepath.IsAbs(p) {
			return "", fmt.Errorf("invalid path %q: %w", p, ErrBadPath)
		}
	}

	// Acquire read lock to prevent concurrent fetch from modifying the bare repo
	unlock := s.acquireGitCacheRead(ownerRepo)
	defer unlock()

	barePath := s.gitCachePath(ownerRepo)
	if _, err := os.Stat(filepath.Join(barePath, "HEAD")); err != nil {
		return "", fmt.Errorf("bare repo not found, call EnsureBareRepo first")
	}

	// Resolve branch to ref
	refName := "origin/" + branch
	commitSHA, err := s.gitRevParse(ctx, barePath, refName)
	if err != nil {
		return "", fmt.Errorf("resolve branch %q: %w", branch, err)
	}

	fmt.Printf("exporting %s@%s paths %v via git archive...\n", ownerRepo, branch, paths)

	// Use git archive to export to tar and extract directly
	// git archive --format=tar <commit> -- path1 path2 ... | tar -x -C <destDir>
	args := []string{"-C", barePath, "archive", "--format=tar", commitSHA, "--"}
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("git archive pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("git archive start: %w", err)
	}

	// Extract tar to destDir
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	if err := extractTar(stdout, destDir); err != nil {
		_ = cmd.Wait()
		return "", fmt.Errorf("extract tar: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("git archive failed: %w", err)
	}

	shortSHA := commitSHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	return shortSHA, nil
}

// gitRevParse runs git rev-parse to resolve a ref to a commit SHA.
// For bare repos, it tries multiple ref formats since refs may be stored differently.
func (s *Storage) gitRevParse(ctx context.Context, repoPath, ref string) (string, error) {
	// Try the ref as-is first
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", ref)
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// For bare repos, refs might be under refs/heads/ directly
	// Try without origin/ prefix if it was provided
	if strings.HasPrefix(ref, "origin/") {
		branchName := strings.TrimPrefix(ref, "origin/")
		cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", branchName)
		out, err = cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}

		// Also try refs/heads/branch format
		cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "refs/heads/"+branchName)
		out, err = cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}

	return "", fmt.Errorf("cannot resolve ref %q", ref)
}

// createZipFromDir creates a zip archive from a directory, excluding .git.
func (s *Storage) createZipFromDir(srcDir, destZip string) error {
	if err := os.MkdirAll(filepath.Dir(destZip), 0o755); err != nil {
		return err
	}
	f, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		// Skip .git directory
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
}

// copyDirContents copies contents of src to dst, excluding .git.
// extractTar extracts a tar archive from reader to destDir.
func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanitize path to prevent directory traversal
		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar path: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// Remove existing symlink if any
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDirContents(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Skip .git directory
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}
		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
