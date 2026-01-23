package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

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
