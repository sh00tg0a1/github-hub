package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirListAndDeleteHandlers(t *testing.T) {
	root := t.TempDir()
	user := "tester"
	userRoot := filepath.Join(root, "users", user)
	if err := os.MkdirAll(filepath.Join(userRoot, "alpha", "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userRoot, "alpha", "beta", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(root, user, "", defaultDownloadTimeout)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// list root
	resp, err := http.Get(ts.URL + "/api/v1/dir/list?path=.")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", resp.StatusCode)
	}
	var entries []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry at root, got %d", len(entries))
	}

	// delete recursively
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/dir?path=alpha&recursive=true", nil)
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d", delResp.StatusCode)
	}

	// list root again -> empty
	resp2, err := http.Get(ts.URL + "/api/v1/dir/list?path=.")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var entries2 []map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&entries2); err != nil {
		t.Fatalf("decode2: %v", err)
	}
	if len(entries2) != 0 {
		t.Fatalf("want 0 entry at root, got %d", len(entries2))
	}
}

func TestStaticIndexServed(t *testing.T) {
	root := t.TempDir()
	s, err := NewServer(root, "default", "", defaultDownloadTimeout)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct == "" {
		t.Fatalf("expected content type for index.html")
	}
}

func TestBadRelPathsAreRejected(t *testing.T) {
	root := t.TempDir()
	s, err := NewServer(root, "default", "", defaultDownloadTimeout)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test GET endpoint /api/v1/dir/list
	listTests := []string{
		"/api/v1/dir/list?path=..",
		"/api/v1/dir/list?path=../foo",
		"/api/v1/dir/list?path=/absolute",
		"/api/v1/dir/list?path=./dot",
		"/api/v1/dir/list?path=foo/../bar",
	}
	for _, u := range listTests {
		resp, err := http.Get(ts.URL + u)
		if err != nil {
			t.Fatalf("get %s: %v", u, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("path %s expected 400, got %d", u, resp.StatusCode)
		}
	}

	// Test DELETE endpoint /api/v1/dir
	dirTests := []string{
		"/api/v1/dir?path=..",
		"/api/v1/dir?path=../foo",
		"/api/v1/dir?path=/absolute",
		"/api/v1/dir?path=./dot",
		"/api/v1/dir?path=foo/../bar",
	}
	for _, u := range dirTests {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete %s: %v", u, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("path %s expected 400, got %d", u, resp.StatusCode)
		}
	}
}

func TestShutdownStopsJanitor(t *testing.T) {
	root := t.TempDir()
	s, err := NewServer(root, "default", "", defaultDownloadTimeout)
	if err != nil {
		t.Fatal(err)
	}

	// Shutdown should stop the janitor and not block
	done := make(chan bool)
	go func() {
		s.Shutdown()
		done <- true
	}()

	select {
	case <-done:
		// Good, shutdown completed quickly
	case <-time.After(1 * time.Second):
		t.Fatal("shutdown should complete quickly")
	}

	// Multiple shutdown calls should be safe and not panic
	s.Shutdown()
	s.Shutdown()
}

func TestDownloadSparseHandler_Validation(t *testing.T) {
	root := t.TempDir()
	s, err := NewServer(root, "default", "", defaultDownloadTimeout)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test missing repo
	resp, err := http.Get(ts.URL + "/api/v1/download/sparse?paths=src")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing repo, got %d", resp.StatusCode)
	}

	// Test missing paths
	resp, err = http.Get(ts.URL + "/api/v1/download/sparse?repo=owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing paths, got %d", resp.StatusCode)
	}

	// Test invalid path with ..
	resp, err = http.Get(ts.URL + "/api/v1/download/sparse?repo=owner/repo&paths=../etc")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid path, got %d", resp.StatusCode)
	}
}
