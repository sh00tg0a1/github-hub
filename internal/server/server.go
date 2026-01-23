package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github-hub/internal/storage"
)

const defaultDownloadTimeout = 30 * time.Minute

//go:embed static/*
var uiFS embed.FS

// Store is the abstraction for workspace/cache storage used by the server.
type Store interface {
	EnsureRepo(ctx context.Context, user, ownerRepo, branch, token string, force bool) (string, error)
	EnsurePackage(ctx context.Context, user, pkgURL string) (string, error)
	EnsureBareRepo(ctx context.Context, ownerRepo, token string) (string, error)
	ExportSparseZip(ctx context.Context, ownerRepo, branch string, paths []string, destZip string) (string, error)
	ExportSparseDir(ctx context.Context, ownerRepo, branch string, paths []string, destDir string) (string, error)
	List(rel string) ([]storage.Entry, error)
	Delete(rel string, recursive bool) error
	Touch(rel string) error
	CleanupExpired(ttl time.Duration) error
}

type Server struct {
	store       Store
	token       string
	defaultUser string
	downloadTO  time.Duration

	cleanupInterval time.Duration
	ttl             time.Duration

	janitorCtx    context.Context
	janitorCancel context.CancelFunc
}

func NewServer(root, defaultUser, githubToken string, downloadTimeout time.Duration) (*Server, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if downloadTimeout <= 0 {
		downloadTimeout = defaultDownloadTimeout
	}
	// Pass download timeout to storage HTTP client
	st := storage.NewWithTimeout(root, downloadTimeout)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		store:           st,
		token:           githubToken,
		defaultUser:     defaultUser,
		downloadTO:      downloadTimeout,
		cleanupInterval: time.Minute,
		ttl:             24 * time.Hour,
		janitorCtx:      ctx,
		janitorCancel:   cancel,
	}
	go s.startJanitor()
	return s, nil
}

// NewServerWithStore allows tests to inject a fake store.
func NewServerWithStore(store Store, githubToken, defaultUser string) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		store:           store,
		token:           githubToken,
		defaultUser:     defaultUser,
		downloadTO:      defaultDownloadTimeout,
		cleanupInterval: time.Minute,
		ttl:             24 * time.Hour,
		janitorCtx:      ctx,
		janitorCancel:   cancel,
	}
	go s.startJanitor()
	return s
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/download", s.handleDownload)
	mux.HandleFunc("/api/v1/download/commit", s.handleDownloadCommit)
	mux.HandleFunc("/api/v1/download/package", s.handleDownloadPackage)
	mux.HandleFunc("/api/v1/download/sparse", s.handleDownloadSparse)
	mux.HandleFunc("/api/v1/branch/switch", s.handleBranchSwitch)
	mux.HandleFunc("/api/v1/dir/list", s.handleDirList)
	mux.HandleFunc("/api/v1/dir", s.handleDir)
	// Static UI for browsing cached workspace
	sub, _ := fs.Sub(uiFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := s.resolveUser(r)
	token := tokenFromRequest(r, s.token)
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))
	force, _ := strconv.ParseBool(r.URL.Query().Get("force"))
	debugDelayStr := strings.TrimSpace(r.URL.Query().Get("debug_delay"))
	debugStreamDelayStr := strings.TrimSpace(r.URL.Query().Get("debug_stream_delay"))
	if repo == "" {
		http.Error(w, "missing repo", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.downloadTO)
	defer cancel()

	// DEBUG: simulate slow network by adding delay per read chunk during download
	if debugDelayStr != "" {
		debugDelay, err := time.ParseDuration(debugDelayStr)
		if err == nil && debugDelay > 0 {
			fmt.Printf("DEBUG: client requested slow network simulation (%s per chunk) for repo=%s\n", debugDelay, repo)
			if st, ok := s.store.(*storage.Storage); ok {
				st.DebugSlowReader = debugDelay
				defer func() { st.DebugSlowReader = 0 }() // cleanup after request
			}
			force = true // ensure we actually download from GitHub (bypass cache)
		}
	}
	var streamDelay time.Duration
	if debugStreamDelayStr != "" {
		if d, err := time.ParseDuration(debugStreamDelayStr); err == nil && d > 0 {
			streamDelay = d
			fmt.Printf("DEBUG: client requested slow stream (%s) for repo=%s\n", d, repo)
		}
	}

	// Ensure cached copy exists (download if missing), and then stream a zip.
	// If branch is empty, EnsureRepo will fetch the default branch from GitHub.
	// If force is true, bypass cache validation and always download fresh.
	zipPath, err := s.store.EnsureRepo(ctx, user, repo, branch, token, force)
	if err != nil {
		fmt.Printf("download error user=%s repo=%s branch=%s err=%v\n", user, repo, branch, err)
		httpError(w, "ensure repo", err)
		return
	}
	// Extract actual branch name from zipPath (e.g., "main.zip" -> "main")
	actualBranch := strings.TrimSuffix(filepath.Base(zipPath), ".zip")
	commitPath := strings.TrimSuffix(zipPath, ".zip") + ".commit.txt"
	if commit := readCommitFile(commitPath); commit != "" {
		w.Header().Set("X-GHH-Commit", commit)
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", safeName(repo, actualBranch)))
	// Update access time for the zip file itself
	zipRelPath := s.userPath(user, filepath.Join("repos", repo, actualBranch+".zip"))
	_ = s.store.Touch(zipRelPath)
	f, err := os.Open(zipPath)
	if err != nil {
		fmt.Printf("zip open error user=%s repo=%s branch=%s err=%v\n", user, repo, actualBranch, err)
		httpError(w, "open zip", err)
		return
	}
	defer f.Close()
	var reader io.Reader = f
	if fi, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		if streamDelay > 0 {
			reader = newSlowReader(f, r.Context(), streamDelay, fi.Size())
		}
	} else if streamDelay > 0 {
		reader = newSlowReader(f, r.Context(), streamDelay, -1)
	}
	if _, err := io.Copy(w, reader); err != nil {
		fmt.Printf("zip stream error user=%s repo=%s branch=%s err=%v\n", user, repo, actualBranch, err)
		return
	}
	fmt.Printf("download ok user=%s repo=%s branch=%s zip=%s\n", user, repo, actualBranch, zipPath)
}

func (s *Server) handleDownloadCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := s.resolveUser(r)
	token := tokenFromRequest(r, s.token)
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))
	force, _ := strconv.ParseBool(r.URL.Query().Get("force"))
	if repo == "" {
		http.Error(w, "missing repo", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.downloadTO)
	defer cancel()

	zipPath, err := s.store.EnsureRepo(ctx, user, repo, branch, token, force)
	if err != nil {
		fmt.Printf("download commit error user=%s repo=%s branch=%s err=%v\n", user, repo, branch, err)
		httpError(w, "ensure repo", err)
		return
	}
	commitPath := strings.TrimSuffix(zipPath, ".zip") + ".commit.txt"
	commit := readCommitFile(commitPath)
	if commit == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(commit + "\n"))
}

func (s *Server) handleDownloadPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := s.resolveUser(r)
	pkgURL := strings.TrimSpace(r.URL.Query().Get("url"))
	debugStreamDelayStr := strings.TrimSpace(r.URL.Query().Get("debug_stream_delay"))
	if pkgURL == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.downloadTO)
	defer cancel()
	var streamDelay time.Duration
	if debugStreamDelayStr != "" {
		if d, err := time.ParseDuration(debugStreamDelayStr); err == nil && d > 0 {
			streamDelay = d
			fmt.Printf("DEBUG: client requested slow stream (%s) for url=%s\n", d, pkgURL)
		}
	}

	filePath, err := s.store.EnsurePackage(ctx, user, pkgURL)
	if err != nil {
		fmt.Printf("download package error user=%s url=%s err=%v\n", user, pkgURL, err)
		httpError(w, "ensure package", err)
		return
	}
	name := filepath.Base(filePath)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", name))
	hashStr := storage.PackageHash(pkgURL)
	_ = s.store.Touch(s.userPath(user, filepath.Join("packages", hashStr, name)))
	f, err := os.Open(filePath)
	if err != nil {
		httpError(w, "open package", err)
		return
	}
	defer f.Close()
	var reader io.Reader = f
	if fi, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		if streamDelay > 0 {
			reader = newSlowReader(f, r.Context(), streamDelay, fi.Size())
		}
	} else if streamDelay > 0 {
		reader = newSlowReader(f, r.Context(), streamDelay, -1)
	}
	if _, err := io.Copy(w, reader); err != nil {
		fmt.Printf("package stream error user=%s url=%s err=%v\n", user, pkgURL, err)
		return
	}
	fmt.Printf("package download ok user=%s url=%s path=%s\n", user, pkgURL, filePath)
}

func (s *Server) handleDownloadSparse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := tokenFromRequest(r, s.token)
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))
	pathsParam := strings.TrimSpace(r.URL.Query().Get("paths"))
	if repo == "" {
		http.Error(w, "missing repo", http.StatusBadRequest)
		return
	}
	if pathsParam == "" {
		http.Error(w, "missing paths", http.StatusBadRequest)
		return
	}
	// Parse paths (comma-separated)
	var paths []string
	for _, p := range strings.Split(pathsParam, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Validate path
		if strings.Contains(p, "..") || filepath.IsAbs(p) {
			http.Error(w, fmt.Sprintf("invalid path: %s", p), http.StatusBadRequest)
			return
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		http.Error(w, "no valid paths provided", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.downloadTO)
	defer cancel()

	// Ensure bare repo is up-to-date
	if _, err := s.store.EnsureBareRepo(ctx, repo, token); err != nil {
		fmt.Printf("sparse download error repo=%s err=%v\n", repo, err)
		httpError(w, "ensure bare repo", err)
		return
	}

	// If branch not specified, use "main" as default (could also fetch default branch)
	if branch == "" {
		branch = "main"
	}

	// Create temp zip file
	tmpFile, err := os.CreateTemp("", "sparse-*.zip")
	if err != nil {
		httpError(w, "create temp file", err)
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Export sparse zip
	commit, err := s.store.ExportSparseZip(ctx, repo, branch, paths, tmpPath)
	if err != nil {
		fmt.Printf("sparse export error repo=%s branch=%s paths=%v err=%v\n", repo, branch, paths, err)
		httpError(w, "export sparse", err)
		return
	}

	// Set headers
	w.Header().Set("X-GHH-Commit", commit)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-sparse.zip\"", safeName(repo, branch)))

	f, err := os.Open(tmpPath)
	if err != nil {
		httpError(w, "open zip", err)
		return
	}
	defer f.Close()

	if fi, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	}

	if _, err := io.Copy(w, f); err != nil {
		fmt.Printf("sparse stream error repo=%s branch=%s err=%v\n", repo, branch, err)
		return
	}
	fmt.Printf("sparse download ok repo=%s branch=%s paths=%v commit=%s\n", repo, branch, paths, commit)
}

func (s *Server) handleBranchSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := s.resolveUser(r)
	token := tokenFromRequest(r, s.token)
	var req struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
		Force  bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Repo) == "" || strings.TrimSpace(req.Branch) == "" {
		http.Error(w, "missing repo/branch", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if _, err := s.store.EnsureRepo(ctx, user, req.Repo, req.Branch, token, req.Force); err != nil {
		fmt.Printf("branch switch error user=%s repo=%s branch=%s err=%v\n", user, req.Repo, req.Branch, err)
		httpError(w, "ensure branch", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, "ok"); err != nil {
		fmt.Printf("branch switch write error user=%s repo=%s branch=%s err=%v\n", user, req.Repo, req.Branch, err)
		return
	}
	fmt.Printf("branch switch ok user=%s repo=%s branch=%s\n", user, req.Repo, req.Branch)
}

func (s *Server) handleDirList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := s.resolveUser(r)
	rel := r.URL.Query().Get("path")
	if badRel(rel) {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	_ = s.store.Touch(s.userPath(user, rel))
	list, err := s.store.List(s.userPath(user, rel))
	if err != nil {
		// Return empty list for not found paths (e.g., new user with no cached repos)
		if errors.Is(err, storage.ErrNotFound) {
			list = []storage.Entry{}
		} else {
			fmt.Printf("dir list error user=%s path=%s err=%v\n", user, rel, err)
			httpError(w, "list", err)
			return
		}
	}
	// Rewrite paths to be relative to user root (no users/<user> prefix), so UI can delete correctly.
	cleanRel := strings.TrimLeft(filepath.ToSlash(rel), "./")
	for i := range list {
		name := list[i].Name
		if cleanRel == "" || cleanRel == "." {
			list[i].Path = name
		} else {
			list[i].Path = filepath.ToSlash(filepath.Join(cleanRel, name))
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(list); err != nil {
		fmt.Printf("dir list write error user=%s path=%s err=%v\n", user, rel, err)
		return
	}
	fmt.Printf("dir list ok user=%s path=%s entries=%d\n", user, rel, len(list))
}

func (s *Server) handleDir(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		user := s.resolveUser(r)
		rel := r.URL.Query().Get("path")
		if badRel(rel) {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		// Path coming from UI is relative to user root. Normalize to absolute user path here.
		if strings.HasPrefix(rel, "users/") || strings.HasPrefix(rel, "users\\") {
			// already absolute-ish, keep as-is
		} else {
			rel = s.userPath(user, rel)
		}
		recursive, _ := strconv.ParseBool(r.URL.Query().Get("recursive"))
		if err := s.store.Delete(rel, recursive); err != nil {
			fmt.Printf("delete error user=%s path=%s recursive=%t err=%v\n", user, rel, recursive, err)
			httpError(w, "delete", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, "deleted"); err != nil {
			fmt.Printf("delete write error user=%s path=%s recursive=%t err=%v\n", user, rel, recursive, err)
			return
		}
		fmt.Printf("delete ok user=%s path=%s recursive=%t\n", user, rel, recursive)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func httpError(w http.ResponseWriter, op string, err error) {
	code := http.StatusInternalServerError
	if errors.Is(err, storage.ErrBadPath) || errors.Is(err, storage.ErrNotFound) {
		code = http.StatusBadRequest
	}
	http.Error(w, op+": "+err.Error(), code)
}

func safeName(repo, branch string) string {
	name := strings.ReplaceAll(repo, "/", "-")
	if strings.TrimSpace(branch) != "" {
		name += "-" + branch
	}
	return name
}

func (s *Server) resolveUser(r *http.Request) string {
	user := r.Header.Get("X-GHH-User")
	if user == "" {
		user = r.URL.Query().Get("user")
	}
	user = strings.TrimSpace(user)
	if user == "" {
		user = s.defaultUser
	}
	return sanitizeUser(user)
}

func (s *Server) userPath(user, rel string) string {
	base := filepath.ToSlash(filepath.Join("users", sanitizeUser(user)))
	rel = strings.TrimLeft(rel, "./")
	if rel == "" || rel == "." || rel == "/" {
		return base
	}
	return filepath.ToSlash(filepath.Join(base, rel))
}

func sanitizeUser(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return "default"
	}
	u = strings.ReplaceAll(u, "\\", "-")
	u = strings.ReplaceAll(u, "/", "-")
	return u
}

func badRel(rel string) bool {
	// Check for suspicious patterns before filepath.Clean normalizes them away
	// (e.g., filepath.Clean("./dot") -> "dot", filepath.Clean("foo/../bar") -> "bar")
	if strings.HasPrefix(rel, "./") || strings.HasPrefix(rel, ".\\") {
		return true
	}
	if strings.Contains(rel, "..") {
		return true
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "" || rel == "." || rel == "/" {
		return false
	}
	// Disallow any absolute or relative traversal/hidden segments
	if strings.HasPrefix(rel, "/") {
		return true
	}
	if strings.HasPrefix(rel, ".") {
		return true
	}
	if strings.Contains(rel, "/.") {
		return true
	}
	return false
}

func readCommitFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	commit := strings.TrimSpace(string(b))
	return commit
}

func tokenFromRequest(r *http.Request, fallback string) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		t := strings.TrimSpace(h[len("bearer "):])
		if t != "" {
			return t
		}
	}
	return fallback
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

func (s *Server) startJanitor() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.janitorCtx.Done():
			return
		case <-ticker.C:
			_ = s.store.CleanupExpired(s.ttl)
		}
	}
}

// Shutdown stops the janitor goroutine and releases associated resources.
func (s *Server) Shutdown() {
	if s.janitorCancel != nil {
		s.janitorCancel()
	}
}
