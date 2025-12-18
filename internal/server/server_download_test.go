package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github-hub/internal/storage"
)

type fakeStore struct {
	ensurePath string
	ensureErr  error
	lastUser   string
	lastRepo   string
	lastBranch string
}

func (f *fakeStore) EnsureRepo(ctx context.Context, user, ownerRepo, branch, token string) (string, error) {
	f.lastUser = user
	f.lastRepo = ownerRepo
	f.lastBranch = branch
	return f.ensurePath, f.ensureErr
}
func (f *fakeStore) List(rel string) ([]storage.Entry, error) { return nil, nil }
func (f *fakeStore) Delete(rel string, recursive bool) error  { return nil }
func (f *fakeStore) Touch(rel string) error                   { return nil }
func (f *fakeStore) CleanupExpired(ttl time.Duration) error   { return nil }

func TestDownloadHandler_UsesStore(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "repo.zip")
	createZip(t, zipPath)
	if err := os.WriteFile(zipPath+".commit.txt", []byte("abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := &fakeStore{ensurePath: zipPath}
	s := NewServerWithStore(fs, "", "default")
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/download?repo=own/repo&branch=main")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("ct=%s", ct)
	}
	if resp.Header.Get("X-GHH-Commit") != "abc123" {
		t.Fatalf("commit header mismatch: %q", resp.Header.Get("X-GHH-Commit"))
	}

	// Validate it's a zip
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) == 0 {
		t.Fatalf("expected files in zip")
	}

	if fs.lastRepo != "own/repo" || fs.lastBranch != "main" || fs.lastUser != "default" {
		t.Fatalf("store called with user=%s repo=%s branch=%s", fs.lastUser, fs.lastRepo, fs.lastBranch)
	}
}

func TestDownloadCommitHandler(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "repo.zip")
	createZip(t, zipPath)
	if err := os.WriteFile(zipPath+".commit.txt", []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := &fakeStore{ensurePath: zipPath}
	s := NewServerWithStore(fs, "", "default")
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/download/commit?repo=own/repo&branch=main")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "deadbeef" {
		t.Fatalf("commit body mismatch: %q", string(body))
	}
}

func TestBranchSwitchHandler_UsesStore(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "repo.zip")
	createZip(t, zipPath)

	fs := &fakeStore{ensurePath: zipPath}
	s := NewServerWithStore(fs, "", "fallback")
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"repo": "own/repo", "branch": "dev"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/branch/switch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GHH-User", "alice")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if fs.lastRepo != "own/repo" || fs.lastBranch != "dev" || fs.lastUser != "alice" {
		t.Fatalf("store called with user=%s repo=%s branch=%s", fs.lastUser, fs.lastRepo, fs.lastBranch)
	}
}

func createZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("sample.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
