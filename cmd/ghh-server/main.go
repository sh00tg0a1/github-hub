package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	srv "github-hub/internal/server"
	"github-hub/internal/version"
)

func main() {
	configPath := findConfigPath(os.Args[1:])
	cfg, err := srv.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Env overrides config defaults for compatibility.
	if envToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); envToken != "" {
		cfg.Token = envToken
	}

	addr := cfg.Addr
	root := cfg.Root
	token := cfg.Token
	defaultUser := cfg.DefaultUser
	downloadTO := cfg.DownloadTimeout
	showVersion := false

	flag.StringVar(&configPath, "config", configPath, "path to server config (yaml or json)")
	flag.StringVar(&addr, "addr", addr, "listen address (e.g., :8080)")
	flag.StringVar(&root, "root", root, "workspace root to store caches")
	flag.StringVar(&token, "github-token", token, "GitHub token for higher rate limits (env: GITHUB_TOKEN)")
	flag.StringVar(&defaultUser, "default-user", defaultUser, "default user grouping when client user is empty")
	flag.BoolVar(&showVersion, "version", showVersion, "print version and exit")
	flag.StringVar(&downloadTO, "download-timeout", downloadTO, "timeout for download/package handlers (e.g., 10m, 5m)")
	flag.Parse()

	if showVersion {
		fmt.Println(version.String())
		return
	}

	dlTimeout, err := time.ParseDuration(strings.TrimSpace(downloadTO))
	if err != nil || dlTimeout <= 0 {
		log.Fatalf("invalid download-timeout: %v", err)
	}

	s, err := srv.NewServer(root, defaultUser, token, dlTimeout)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           logging(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("ghh-server listening on %s, root=%s, default_user=%s\n", addr, root, defaultUser)
	log.Fatal(httpSrv.ListenAndServe())
}

type statusRecorder struct {
	http.ResponseWriter
	status     int
	size       int
	headerSent bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.headerSent {
		r.status = code
		r.headerSent = true
		r.ResponseWriter.WriteHeader(code)
	}
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
		r.headerSent = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		user := r.Header.Get("X-GHH-User")
		if user == "" {
			user = r.URL.Query().Get("user")
		}
		fmt.Printf("%s %s status=%d bytes=%d dur=%s user=%s\n",
			r.Method, r.URL.Path, rec.status, rec.size, time.Since(start), strings.TrimSpace(user))
	})
}

// findConfigPath scans args for --config or -config to allow loading defaults before flag parsing.
func findConfigPath(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--config="))
		}
		if arg == "--config" || arg == "-config" {
			if i+1 < len(args) {
				return strings.TrimSpace(args[i+1])
			}
		}
	}
	return ""
}
