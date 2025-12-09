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

//go:embed static/*
var uiFS embed.FS

// Store is the abstraction for workspace/cache storage used by the server.
type Store interface {
	EnsureRepo(ctx context.Context, user, ownerRepo, branch, token string) (string, error)
	List(rel string) ([]storage.Entry, error)
	Delete(rel string, recursive bool) error
	Touch(rel string) error
	CleanupExpired(ttl time.Duration) error
}

type Server struct {
	store       Store
	token       string
	defaultUser string

	cleanupInterval time.Duration
	ttl             time.Duration

	janitorCtx    context.Context
	janitorCancel context.CancelFunc
}

func NewServer(root, defaultUser, githubToken string) (*Server, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	st := storage.New(root)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		store:           st,
		token:           githubToken,
		defaultUser:     defaultUser,
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
	if repo == "" {
		http.Error(w, "missing repo", http.StatusBadRequest)
		return
	}
	if branch == "" {
		branch = "default"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	// Ensure cached copy exists (download if missing), and then stream a zip.
	zipPath, err := s.store.EnsureRepo(ctx, user, repo, branch, token)
	if err != nil {
		fmt.Printf("download error user=%s repo=%s branch=%s err=%v\n", user, repo, branch, err)
		httpError(w, "ensure repo", err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", safeName(repo, branch)))
	// Update access time for the zip file itself, not just the user directory
	zipRelPath := s.userPath(user, filepath.Join("repos", repo, branch+".zip"))
	_ = s.store.Touch(zipRelPath)
	f, err := os.Open(zipPath)
	if err != nil {
		fmt.Printf("zip open error user=%s repo=%s branch=%s err=%v\n", user, repo, branch, err)
		httpError(w, "open zip", err)
		return
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		fmt.Printf("zip stream error user=%s repo=%s branch=%s err=%v\n", user, repo, branch, err)
		return
	}
	fmt.Printf("download ok user=%s repo=%s branch=%s zip=%s\n", user, repo, branch, zipPath)
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
	if _, err := s.store.EnsureRepo(ctx, user, req.Repo, req.Branch, token); err != nil {
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
		recursive, _ := strconv.ParseBool(r.URL.Query().Get("recursive"))
		if err := s.store.Delete(s.userPath(user, rel), recursive); err != nil {
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
