package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

func New(root string) *Storage { return &Storage{Root: root} }

// EnsureRepo ensures a cached repo (owner/repo) at branch exists under workspace.
// If missing, it downloads from GitHub zipball and extracts into
//
//	<root>/users/<user>/repos/<owner>/<repo>/<branch>
//
// If branch is empty, uses "default" path name.
func (s *Storage) EnsureRepo(ctx context.Context, user, ownerRepo, branch, token string) (string, error) {
	user = strings.Trim(user, "/ ")
	if user == "" {
		user = "default"
	}
	if strings.ContainsRune(user, '/') || strings.ContainsRune(user, '\\') {
		return "", fmt.Errorf("invalid user: %w", ErrBadPath)
	}
	ownerRepo = strings.Trim(ownerRepo, "/")
	if ownerRepo == "" || strings.Count(ownerRepo, "/") != 1 {
		return "", fmt.Errorf("owner/repo expected: %w", ErrBadPath)
	}
	if branch == "" {
		branch = "default"
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

	// If we have cache and sha matches (or fetch failed), reuse.
	if info, err := os.Stat(zipPath); err == nil && !info.IsDir() {
		if remoteSHA != "" {
			if cachedSHA, err := readSHA(metaPath); err == nil && cachedSHA == remoteSHA {
				_ = s.touch(zipPath)
				return zipPath, nil
			}
		} else if fetchErr != nil {
			_ = s.touch(zipPath)
			return zipPath, nil
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
	if err := os.Rename(tmpPath, zipPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	if remoteSHA != "" {
		_ = writeSHA(metaPath, remoteSHA)
	} else {
		_ = os.Remove(metaPath)
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
		if e.Name() == ".last_access" {
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
	url := fmt.Sprintf("https://codeload.github.com/%s/zip/%s", ownerRepo, branch)
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

// CleanupExpired removes repos unused beyond ttl under users/*/repos/*/*/<branch>.
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
		if filepath.Ext(path) != ".zip" {
			return nil
		}
		rel, _ := filepath.Rel(s.Root, path)
		parts := splitPath(rel)
		// expect users/<user>/repos/<owner>/<repo>/<branch>.zip
		if len(parts) < 6 || parts[0] != "users" || parts[2] != "repos" {
			return nil
		}
		if expired(path, cutoff) {
			_ = os.Remove(path)
			trimEmpty(filepath.Dir(path), filepath.Join(s.Root, "users"))
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

func (s *Storage) fetchBranchSHA(ctx context.Context, ownerRepo, branch, token string) (string, error) {
	if branch == "" || branch == "default" {
		return "", fmt.Errorf("branch unspecified")
	}
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid owner/repo")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/branches/%s", ownerRepo, branch)
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
