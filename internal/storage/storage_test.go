package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListAndDelete(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	// create structure: a/, a/x.txt, b.txt
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "x.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("bye"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := s.List(".")
	if err != nil {
		t.Fatalf("List root: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}

	// delete file
	if err := s.Delete("b.txt", false); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	// delete dir non-recursive should fail when not empty
	if err := s.Delete("a", false); err == nil {
		t.Fatalf("expected error deleting non-empty dir without recursive")
	}
	// recursive delete works
	if err := s.Delete("a", true); err != nil {
		t.Fatalf("delete dir recursive: %v", err)
	}
}

func TestList_NotFound(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	if _, err := s.List("nope"); err == nil {
		t.Fatalf("expected error for not found path")
	}
}

func TestSafeJoinPreventsEscape(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	// Accessing parent should fail via public API like Delete/List
	if err := s.Delete("..", true); err == nil {
		t.Fatalf("expected error for escaping path")
	}
	if _, err := s.List("../outside"); err == nil {
		t.Fatalf("expected error for escaping list")
	}
}

func TestDownloadZip_EscapesSlashBranch(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()

	branch := "feature/sub"
	dest := filepath.Join(root, "out.zip")

	var seenPath string
	s.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		seenPath = req.URL.EscapedPath()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("zipdata")),
			Header:     make(http.Header),
		}, nil
	})}

	if err := s.downloadZip(ctx, "owner/repo", branch, "", dest); err != nil {
		t.Fatalf("downloadZip: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "zipdata" {
		t.Fatalf("unexpected content: %q", string(data))
	}
	if !strings.Contains(seenPath, "feature%2Fsub") {
		t.Fatalf("expected escaped branch in path, got %q", seenPath)
	}
}

func TestFetchBranchSHA_EscapesSlashBranch(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()

	branch := "feature/sub"
	s.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.EscapedPath(), "feature%2Fsub") {
			return nil, fmt.Errorf("path not escaped: %s", req.URL.EscapedPath())
		}
		body := `{"commit":{"sha":"abc123"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	sha, err := s.fetchBranchSHA(ctx, "owner/repo", branch, "")
	if err != nil {
		t.Fatalf("fetchBranchSHA: %v", err)
	}
	if sha != "abc123" {
		t.Fatalf("unexpected sha: %s", sha)
	}
}

func TestDownloadZip_RetryOnServerError(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	s.RetryMax = 1
	s.RetryBackoff = 0
	ctx := context.Background()

	dest := filepath.Join(root, "out.zip")
	attempts := 0
	s.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("temporary")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("zipdata")),
			Header:     make(http.Header),
		}, nil
	})}

	if err := s.downloadZip(ctx, "owner/repo", "main", "", dest); err != nil {
		t.Fatalf("downloadZip: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "zipdata" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestDownloadFile_RetryOnServerError(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	s.RetryMax = 1
	s.RetryBackoff = 0
	ctx := context.Background()

	dest := filepath.Join(root, "pkg.bin")
	attempts := 0
	s.HTTPClient = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("upstream")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("package")),
			Header:     make(http.Header),
		}, nil
	})}

	if err := s.downloadFile(ctx, "https://example.com/package", dest); err != nil {
		t.Fatalf("downloadFile: %v", err)
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestExportSparseZip_PathValidation(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()

	// Test invalid paths
	testCases := []struct {
		name  string
		paths []string
	}{
		{"empty paths", []string{}},
		{"path with ..", []string{"foo/../bar"}},
		{"absolute path", []string{"/etc/passwd"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.ExportSparseZip(ctx, "owner/repo", "main", tc.paths, filepath.Join(root, "out.zip"))
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestExportSparseDir_PathValidation(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()

	// Test invalid paths
	testCases := []struct {
		name  string
		paths []string
	}{
		{"empty paths", []string{}},
		{"path with ..", []string{"../escape"}},
		{"absolute path", []string{"/root/secret"}},
		{"mixed valid and invalid", []string{"src", "foo/../bar"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.ExportSparseDir(ctx, "owner/repo", "main", tc.paths, filepath.Join(root, "out"))
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestEnsureBareRepo_InvalidRepo(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	ctx := context.Background()

	// Test invalid repo formats
	testCases := []struct {
		name      string
		ownerRepo string
	}{
		{"empty", ""},
		{"no slash", "repo"},
		{"too many slashes", "owner/repo/extra"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.EnsureBareRepo(ctx, tc.ownerRepo, "")
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestGitCachePath(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	testCases := []struct {
		ownerRepo string
		expected  string
	}{
		{"owner/repo", filepath.Join(root, "git-cache", "owner", "repo.git")},
		{"foo/bar", filepath.Join(root, "git-cache", "foo", "bar.git")},
	}

	for _, tc := range testCases {
		t.Run(tc.ownerRepo, func(t *testing.T) {
			result := s.gitCachePath(tc.ownerRepo)
			if result != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}
