package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ic "github-hub/internal/client"
	cfgpkg "github-hub/internal/config"
	"github-hub/internal/version"
)

const (
	defaultTimeout      = 30 * time.Second
	defaultRetryMax     = 5
	defaultRetryBackoff = 2 * time.Second
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	// Global flags
	server := getenvDefault("GHH_BASE_URL", "")
	token := os.Getenv("GHH_TOKEN")
	timeout := defaultTimeout
	retryMax := defaultRetryMax
	retryBackoff := defaultRetryBackoff
	insecure := false
	configPath := os.Getenv("GHH_CONFIG")
	user := strings.TrimSpace(os.Getenv("GHH_USER"))
	showVersion := false

	if v := strings.TrimSpace(os.Getenv("GHH_RETRY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			retryMax = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("GHH_RETRY_BACKOFF")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			retryBackoff = d
		}
	}

	global := flag.NewFlagSet("ghh", flag.ContinueOnError)
	global.Usage = func() { printUsage() }
	global.StringVar(&server, "server", server, "server base URL (env: GHH_BASE_URL or config.base_url)")
	global.StringVar(&token, "token", token, "auth token (env: GHH_TOKEN)")
	global.StringVar(&user, "user", user, "user name (env: GHH_USER or config.user)")
	global.DurationVar(&timeout, "timeout", timeout, "HTTP timeout")
	global.IntVar(&retryMax, "retry", retryMax, "retry times for failed downloads (env: GHH_RETRY)")
	global.DurationVar(&retryBackoff, "retry-backoff", retryBackoff, "wait before retrying a failed download (env: GHH_RETRY_BACKOFF)")
	global.BoolVar(&insecure, "insecure", insecure, "skip TLS verification")
	global.StringVar(&configPath, "config", configPath, "path to YAML config (env: GHH_CONFIG); JSON compatible")
	global.BoolVar(&showVersion, "version", showVersion, "print version and exit")

	// Parse global flags followed by subcommands.
	// Example: ghh --server http://... download --repo foo --branch main --dest out.zip
	// We parse only up to the first non-flag arg (the subcommand), then reparse per-subcommand.
	if err := global.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage()
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if showVersion {
		fmt.Println(version.String())
		return
	}

	args := global.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(2)
	}

	// Load config and merge with flags
	cfg, err := cfgpkg.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}
	if server == "" {
		server = cfg.BaseURL
	}
	if token == "" && cfg.Token != "" {
		token = cfg.Token
	}
	if strings.TrimSpace(user) == "" && strings.TrimSpace(cfg.User) != "" {
		user = cfg.User
	}
	eps := ic.DefaultEndpoints()

	if server == "" {
		server = "http://localhost:8080"
	}

	// Build HTTP client
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 optional
	}
	httpClient := &http.Client{Timeout: timeout, Transport: transport}
	defer transport.CloseIdleConnections()
	client := ic.NewClient(server, token, httpClient)
	client.Endpoint = eps
	client.User = strings.TrimSpace(user)
	client.RetryMax = retryMax
	client.RetryBackoff = retryBackoff
	client.ProgressInterval = time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch args[0] {
	case "download":
		cmd := flag.NewFlagSet("download", flag.ExitOnError)
		pkgURLFlag := cmd.String("package", "", "package download URL")
		repo := cmd.String("repo", "", "repository identifier (e.g. owner/name or name)")
		branch := cmd.String("branch", "", "branch name (default: server default)")
		dest := cmd.String("dest", "", "destination path (default: current directory)")
		extract := cmd.Bool("extract", false, "extract zip archive into dest directory")
		debugDelay := cmd.String("debug-delay", "", "DEBUG: request server to add artificial delay (e.g., 90s, 2m)")
		debugStreamDelay := cmd.String("debug-stream-delay", "", "DEBUG: slow down server streaming to client (e.g., 90s, 2m)")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if *debugDelay != "" {
			client.DebugDelay = *debugDelay
		}
		if *debugStreamDelay != "" {
			client.DebugStreamDelay = *debugStreamDelay
		}
		pkgURL := strings.TrimSpace(*pkgURLFlag)
		if pkgURL != "" {
			destPath := resolvePackageDest(pkgURL, *dest)
			if err := client.DownloadPackage(ctx, pkgURL, destPath); err != nil {
				exitErr(err)
			}
			return
		}
		if *repo == "" {
			fmt.Fprintln(os.Stderr, "download requires --repo")
			os.Exit(2)
		}
		zipPath, extractDir := resolveDest(*repo, *dest, *extract)
		if err := client.Download(ctx, *repo, *branch, zipPath, extractDir); err != nil {
			exitErr(err)
		}

	case "download-sparse":
		cmd := flag.NewFlagSet("download-sparse", flag.ExitOnError)
		repo := cmd.String("repo", "", "repository identifier (e.g. owner/name)")
		branch := cmd.String("branch", "", "branch name (default: main)")
		var pathsFlag multiFlag
		cmd.Var(&pathsFlag, "path", "directory/file path to include (can be specified multiple times or comma-separated)")
		dest := cmd.String("dest", "", "destination path (default: current directory)")
		extract := cmd.Bool("extract", false, "extract zip archive into dest directory")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if *repo == "" {
			fmt.Fprintln(os.Stderr, "download-sparse requires --repo")
			os.Exit(2)
		}
		// Parse paths from flag
		var paths []string
		for _, p := range pathsFlag {
			for _, part := range strings.Split(p, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					paths = append(paths, part)
				}
			}
		}
		if len(paths) == 0 {
			fmt.Fprintln(os.Stderr, "download-sparse requires at least one --path")
			os.Exit(2)
		}
		// Build default name: repo-branch (sanitize branch: replace / with -)
		defaultName := *repo
		branchName := strings.TrimSpace(*branch)
		if branchName == "" {
			branchName = "main"
		}
		// Sanitize branch name for filesystem (e.g., release/0.2.0 -> release-0.2.0)
		safeBranch := strings.ReplaceAll(branchName, "/", "-")
		defaultName = defaultName + "-" + safeBranch
		zipPath, extractDir := resolveDest(defaultName, *dest, *extract)
		if err := client.DownloadSparse(ctx, *repo, *branch, paths, zipPath, extractDir); err != nil {
			exitErr(err)
		}

	case "switch":
		cmd := flag.NewFlagSet("switch", flag.ExitOnError)
		repo := cmd.String("repo", "", "repository identifier")
		branch := cmd.String("branch", "", "branch to switch to")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if *repo == "" || *branch == "" {
			fmt.Fprintln(os.Stderr, "switch requires --repo and --branch")
			os.Exit(2)
		}
		if err := client.SwitchBranch(ctx, *repo, *branch); err != nil {
			exitErr(err)
		}

	case "ls":
		cmd := flag.NewFlagSet("ls", flag.ExitOnError)
		path := cmd.String("path", ".", "remote path to list (relative to user root, e.g. repos/owner/repo)")
		raw := cmd.Bool("raw", false, "print raw JSON returned by server")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		// Allow positional path: ghh ls <path>
		if cmd.NArg() > 0 && *path == "." {
			*path = cmd.Arg(0)
		}
		if err := client.ListDir(ctx, *path, *raw); err != nil {
			exitErr(err)
		}

	case "rm":
		cmd := flag.NewFlagSet("rm", flag.ExitOnError)
		path := cmd.String("path", "", "remote path to delete")
		recursive := cmd.Bool("r", false, "recursive delete")
		if err := cmd.Parse(args[1:]); err != nil {
			exitErr(err)
		}
		if *path == "" {
			fmt.Fprintln(os.Stderr, "rm requires --path")
			os.Exit(2)
		}
		if err := client.DeleteDir(ctx, *path, *recursive); err != nil {
			exitErr(err)
		}

	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		os.Exit(2)
	}
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func exitErr(err error) {
	if err == nil {
		return
	}
	var he *ic.HTTPError
	if errors.As(err, &he) {
		fmt.Fprintf(os.Stderr, "error: %s (status=%d)\n", he.Message, he.StatusCode)
		if he.Body != "" {
			fmt.Fprintln(os.Stderr, he.Body)
		}
	} else {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(1)
}

// multiFlag allows a flag to be specified multiple times.
type multiFlag []string

func (f *multiFlag) String() string { return strings.Join(*f, ",") }
func (f *multiFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func printUsage() {
	fmt.Print(`ghh - GitHub Hub client (offline-friendly)

Usage:
  ghh [--server URL] [--token TOKEN] [--config PATH] <command> [flags]
  Note: paths in ls/rm are relative to user root (users/<user>, default user=default). Omitting --path lists the user root.

Commands:
  download         Download repository code as archive (optionally extract) or release package (--package URL)
  download-sparse  Download selected directories from a repository using sparse checkout
  switch           Switch repository branch on server
  ls               List remote directory contents (path is relative to user root; no leading "users/")
  rm               Delete remote directory (use -r for recursive)
  help             Show this help message

Global Flags:
  --server     Server base URL (env: GHH_BASE_URL) (default: http://localhost:8080)
  --token      Auth token (env: GHH_TOKEN)
  --user       User name for grouping cache (env: GHH_USER)
  --config     Path to YAML config (env: GHH_CONFIG); JSON compatible
  --timeout    HTTP timeout (default: 30s)
  --retry      Retry times for failed downloads (env: GHH_RETRY)
  --retry-backoff  Wait before retrying a failed download (env: GHH_RETRY_BACKOFF)
  --insecure   Skip TLS verification
  --version    Print version and exit

Download Flags:
  --repo         Repository identifier (e.g. owner/name)
  --branch       Branch name (default: server default)
  --dest         Destination path (default: current directory)
  --extract      Extract zip archive into dest directory
  --package      Package download URL (alternative to --repo)
  --debug-delay  DEBUG: request server to add artificial delay (e.g., 90s, 2m)
  --debug-stream-delay  DEBUG: slow down server streaming to client (e.g., 90s, 2m)

Download-Sparse Flags:
  --repo       Repository identifier (e.g. owner/name)
  --branch     Branch name (default: main)
  --path       Directory/file path to include (repeatable or comma-separated)
  --dest       Destination path (default: current directory)
  --extract    Extract zip archive into dest directory

Examples:
  ghh --server http://localhost:8080 download --repo foo/bar --branch main
  ghh --server http://localhost:8080 download --repo foo/bar --dest out.zip
  ghh --server http://localhost:8080 download --repo foo --extract
  ghh --server http://localhost:8080 download --package https://example.com/pkg.tar.gz --dest ./pkg.tar.gz
  ghh --server http://localhost:8080 download-sparse --repo foo/bar --path src --path docs
  ghh --server http://localhost:8080 download-sparse --repo foo/bar --path src,docs --extract
  ghh --server http://localhost:8080 switch --repo foo/bar --branch dev
  ghh --server http://localhost:8080 ls --path repos/foo/bar
  ghh --server http://localhost:8080 rm --path repos/foo/bar --r
  ghh --timeout 3m download --repo foo/bar --debug-delay 90s
`)
}

// resolveDest determines the zip file path and extract directory based on repo and dest flag.
// Returns (zipPath, extractDir):
// - zipPath: where to save the zip file
// - extractDir: where to extract (empty if extract=false, or same as zip's parent dir)
func resolveDest(repo, dest string, extract bool) (zipPath, extractDir string) {
	// Extract repo name from owner/repo
	repoName := repo
	if idx := strings.LastIndex(repoName, "/"); idx >= 0 {
		repoName = repoName[idx+1:]
	}

	// Determine the base directory and zip file path
	if dest == "" {
		// Default to current directory
		zipPath = "./" + repoName + ".zip"
		if extract {
			extractDir = "."
		}
		return
	}

	// If dest is an existing directory, save zip inside it
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		zipPath = filepath.Join(dest, repoName+".zip")
		if extract {
			extractDir = dest
		}
		return
	}

	// dest is a file path (or non-existent path)
	if extract {
		// When extract is true and dest doesn't end with .zip, treat dest as directory
		if !strings.HasSuffix(strings.ToLower(dest), ".zip") {
			// Create the directory and extract there
			extractDir = dest
			zipPath = filepath.Join(dest, repoName+".zip")
			return
		}
		// dest is a .zip file, extract to its parent directory
		zipPath = dest
		extractDir = filepath.Dir(dest)
		if extractDir == "" {
			extractDir = "."
		}
		return
	}

	// No extract, just save to dest
	zipPath = dest
	return
}

// resolvePackageDest determines package save path. If dest is empty, use basename of URL in cwd.
// If dest is a directory, place the file inside that directory with basename of URL.
// Otherwise, dest is treated as the file path.
func resolvePackageDest(pkgURL, dest string) string {
	base := filepath.Base(pkgURL)
	if base == "" || base == "/" || base == "." {
		base = "package.bin"
	}
	if dest == "" {
		return "./" + base
	}
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return filepath.Join(dest, base)
	}
	return dest
}
