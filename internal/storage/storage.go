package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrBadPath  = errors.New("bad path")
	ErrNotFound = errors.New("not found")
)

type Storage struct {
	Root string

	mu   sync.Mutex
	lock map[string]*sync.Mutex
}

func sanitizeName(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, "\\", "-")
	v = strings.ReplaceAll(v, "/", "-")
	return v
}

// EnsurePackage caches a package archive downloaded from pkgURL under:
// <root>/users/<user>/packages/<name>/<url-hash>/<filename>
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

	hash := sha256.Sum256([]byte(pkgURL))
	hashStr := hex.EncodeToString(hash[:])
	if len(hashStr) > 12 {
		hashStr = hashStr[:12]
	}

	pkgDir := filepath.Join(s.Root, "users", user, "packages", name, hashStr)
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

func New(root string) *Storage { return &Storage{Root: root} }

// EnsureRepo ensures a cached repo (owner/repo) at branch exists under workspace.
// If missing, it downloads from GitHub zipball and extracts into
//
//	<root>/users/<user>/repos/<owner>/<repo>/<branch>.zip
//
// If branch is empty, fetches the default branch from GitHub API.
func (s *Storage) EnsureRepo(ctx context.Context, user, ownerRepo, branch, token string) (string, error) {
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

	// If we have cache and sha matches (or fetch failed but no cache), reuse.
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
	url := fmt.Sprintf("https://codeload.github.com/%s/zip/%s", ownerRepo, url.PathEscape(branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("download archive failed: %s", string(b))
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func (s *Storage) downloadFile(ctx context.Context, fileURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("download failed: %s", string(b))
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
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
	resp, err := http.DefaultClient.Do(req)
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
	resp, err := http.DefaultClient.Do(req)
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
