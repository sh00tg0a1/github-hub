package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDest(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		repo       string
		dest       string
		extract    bool
		wantZip    string
		wantExtDir string
	}{
		{
			name:       "empty dest, no extract",
			repo:       "owner/myrepo",
			dest:       "",
			extract:    false,
			wantZip:    "./myrepo.zip",
			wantExtDir: "",
		},
		{
			name:       "empty dest, with extract",
			repo:       "owner/myrepo",
			dest:       "",
			extract:    true,
			wantZip:    "./myrepo.zip",
			wantExtDir: ".",
		},
		{
			name:       "repo without owner, no extract",
			repo:       "myrepo",
			dest:       "",
			extract:    false,
			wantZip:    "./myrepo.zip",
			wantExtDir: "",
		},
		{
			name:       "repo without owner, with extract",
			repo:       "myrepo",
			dest:       "",
			extract:    true,
			wantZip:    "./myrepo.zip",
			wantExtDir: ".",
		},
		{
			name:       "explicit file path, no extract",
			repo:       "owner/myrepo",
			dest:       "output.zip",
			extract:    false,
			wantZip:    "output.zip",
			wantExtDir: "",
		},
		{
			name:       "explicit file path, with extract",
			repo:       "owner/myrepo",
			dest:       "output.zip",
			extract:    true,
			wantZip:    "output.zip",
			wantExtDir: ".",
		},
		{
			name:       "explicit directory path (non-existent), no extract",
			repo:       "owner/myrepo",
			dest:       "mydir/subdir",
			extract:    false,
			wantZip:    "mydir/subdir",
			wantExtDir: "",
		},
		{
			name:       "explicit directory path (non-existent), with extract",
			repo:       "owner/myrepo",
			dest:       "mydir/subdir.zip",
			extract:    true,
			wantZip:    "mydir/subdir.zip",
			wantExtDir: "mydir",
		},
		{
			name:       "existing directory, no extract",
			repo:       "owner/myrepo",
			dest:       tmpDir,
			extract:    false,
			wantZip:    filepath.Join(tmpDir, "myrepo.zip"),
			wantExtDir: "",
		},
		{
			name:       "existing directory, with extract",
			repo:       "owner/myrepo",
			dest:       tmpDir,
			extract:    true,
			wantZip:    filepath.Join(tmpDir, "myrepo.zip"),
			wantExtDir: tmpDir,
		},
		{
			name:       "dot as dest (current dir), no extract",
			repo:       "owner/myrepo",
			dest:       ".",
			extract:    false,
			wantZip:    "myrepo.zip", // filepath.Join normalizes
			wantExtDir: "",
		},
		{
			name:       "dot as dest (current dir), with extract",
			repo:       "owner/myrepo",
			dest:       ".",
			extract:    true,
			wantZip:    "myrepo.zip",
			wantExtDir: ".",
		},
		{
			name:       "nested repo path",
			repo:       "org/team/project",
			dest:       "",
			extract:    false,
			wantZip:    "./project.zip",
			wantExtDir: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotZip, gotExtDir := resolveDest(tt.repo, tt.dest, tt.extract)
			if gotZip != tt.wantZip {
				t.Errorf("resolveDest(%q, %q, %v) zipPath = %q, want %q",
					tt.repo, tt.dest, tt.extract, gotZip, tt.wantZip)
			}
			if gotExtDir != tt.wantExtDir {
				t.Errorf("resolveDest(%q, %q, %v) extractDir = %q, want %q",
					tt.repo, tt.dest, tt.extract, gotExtDir, tt.wantExtDir)
			}
		})
	}
}

func TestResolveDest_CurrentDirExists(t *testing.T) {
	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Create and change to temp directory
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Test with empty dest (uses default naming)
	gotZip, gotExtDir := resolveDest("owner/myrepo", "", false)
	if gotZip != "./myrepo.zip" || gotExtDir != "" {
		t.Errorf("resolveDest empty dest = (%q, %q), want (\"./myrepo.zip\", \"\")", gotZip, gotExtDir)
	}

	gotZip, gotExtDir = resolveDest("owner/myrepo", "", true)
	if gotZip != "./myrepo.zip" || gotExtDir != "." {
		t.Errorf("resolveDest empty dest (extract) = (%q, %q), want (\"./myrepo.zip\", \".\")", gotZip, gotExtDir)
	}

	// Test with "." as dest (existing directory)
	gotZip, gotExtDir = resolveDest("owner/myrepo", ".", false)
	if gotZip != "myrepo.zip" || gotExtDir != "" {
		t.Errorf("resolveDest dot dest = (%q, %q), want (\"myrepo.zip\", \"\")", gotZip, gotExtDir)
	}

	gotZip, gotExtDir = resolveDest("owner/myrepo", ".", true)
	if gotZip != "myrepo.zip" || gotExtDir != "." {
		t.Errorf("resolveDest dot dest (extract) = (%q, %q), want (\"myrepo.zip\", \".\")", gotZip, gotExtDir)
	}
}
