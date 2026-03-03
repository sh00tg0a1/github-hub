package client

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github-hub/internal/storage"
)

func minimalZipBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("dummy.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownloadPackage_Retry(t *testing.T) {
	var attempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download/package", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Length", "7")
		_, _ = w.Write([]byte("package"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "", server.Client())
	c.RetryMax = 1
	c.RetryBackoff = 0
	c.ProgressInterval = 10 * time.Millisecond

	dest := filepath.Join(t.TempDir(), "pkg.bin")
	if err := c.DownloadPackage(context.Background(), "https://example.com/pkg.bin", dest); err != nil {
		t.Fatalf("DownloadPackage: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "package" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestDownloadRepo_Retry(t *testing.T) {
	var attempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		w.Header().Set("X-GHH-Commit", "abc123")
		w.Header().Set("Content-Length", "7")
		_, _ = w.Write([]byte("zipdata"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "", server.Client())
	c.RetryMax = 1
	c.RetryBackoff = 0
	c.ProgressInterval = 10 * time.Millisecond

	dest := filepath.Join(t.TempDir(), "repo.zip")
	if err := c.Download(context.Background(), "owner/repo", "main", dest, ""); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	if string(data) != "zipdata" {
		t.Fatalf("unexpected zip content: %q", string(data))
	}
	commitPath := dest + ".commit.txt"
	commitData, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatalf("read commit: %v", err)
	}
	if strings.TrimSpace(string(commitData)) != "abc123" {
		t.Fatalf("unexpected commit: %q", string(commitData))
	}
}

func TestDownload_WritesInfoJSON(t *testing.T) {
	infoResp := `{"repo":"owner/repo","branch":"main","commit_sha":"abc123","commit_message":"fix","changed_files":["M\tfoo.go"]}`
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-GHH-Commit", "abc123")
		w.Header().Set("Content-Length", "7")
		_, _ = w.Write([]byte("zipdata"))
	})
	mux.HandleFunc("/api/v1/download/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(infoResp))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "", server.Client())
	c.ProgressInterval = 10 * time.Millisecond

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "repo.zip")
	if err := c.Download(context.Background(), "owner/repo", "main", dest, ""); err != nil {
		t.Fatalf("Download: %v", err)
	}
	infoPath := filepath.Join(tmpDir, "repo.info.json")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("read info.json: %v", err)
	}
	var info storage.RepoInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("unmarshal info: %v", err)
	}
	if info.Repo != "owner/repo" || info.Branch != "main" || info.CommitSHA != "abc123" {
		t.Fatalf("info mismatch: %+v", info)
	}
	if len(info.ChangedFiles) != 1 || info.ChangedFiles[0] != "M\tfoo.go" {
		t.Fatalf("changed_files=%v", info.ChangedFiles)
	}
}

func TestDownload_WritesInfoJSON_ExtractDir(t *testing.T) {
	zipData := minimalZipBytes(t)
	infoResp := `{"repo":"owner/repo","branch":"main","commit_sha":"def456","commit_message":"","changed_files":[]}`
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-GHH-Commit", "def456")
		w.Header().Set("Content-Length", strconv.Itoa(len(zipData)))
		_, _ = w.Write(zipData)
	})
	mux.HandleFunc("/api/v1/download/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(infoResp))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "", server.Client())
	c.ProgressInterval = 10 * time.Millisecond

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "repo.zip")
	if err := c.Download(context.Background(), "owner/repo", "main", zipPath, tmpDir); err != nil {
		t.Fatalf("Download: %v", err)
	}
	infoPath := filepath.Join(tmpDir, "info.json")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("read info.json: %v", err)
	}
	var info storage.RepoInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("unmarshal info: %v", err)
	}
	if info.Repo != "owner/repo" || info.CommitSHA != "def456" {
		t.Fatalf("info mismatch: %+v", info)
	}
}

func TestDownloadSparse_Success(t *testing.T) {
	var gotPaths string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download/sparse", func(w http.ResponseWriter, r *http.Request) {
		gotPaths = r.URL.Query().Get("paths")
		w.Header().Set("X-GHH-Commit", "def456")
		w.Header().Set("Content-Length", "11")
		_, _ = w.Write([]byte("sparsedata!"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "", server.Client())
	c.ProgressInterval = 10 * time.Millisecond

	dest := filepath.Join(t.TempDir(), "sparse.zip")
	paths := []string{"src", "docs"}
	if err := c.DownloadSparse(context.Background(), "owner/repo", "main", paths, dest, ""); err != nil {
		t.Fatalf("DownloadSparse: %v", err)
	}
	if gotPaths != "src,docs" {
		t.Fatalf("expected paths=src,docs, got %q", gotPaths)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	if string(data) != "sparsedata!" {
		t.Fatalf("unexpected content: %q", string(data))
	}
	commitPath := dest + ".commit.txt"
	commitData, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatalf("read commit: %v", err)
	}
	if strings.TrimSpace(string(commitData)) != "def456" {
		t.Fatalf("unexpected commit: %q", string(commitData))
	}
}

func TestDownloadSparse_Retry(t *testing.T) {
	var attempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download/sparse", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-GHH-Commit", "ghi789")
		w.Header().Set("Content-Length", "6")
		_, _ = w.Write([]byte("sparse"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := NewClient(server.URL, "", server.Client())
	c.RetryMax = 1
	c.RetryBackoff = 0
	c.ProgressInterval = 10 * time.Millisecond

	dest := filepath.Join(t.TempDir(), "sparse.zip")
	if err := c.DownloadSparse(context.Background(), "owner/repo", "main", []string{"src"}, dest, ""); err != nil {
		t.Fatalf("DownloadSparse: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
