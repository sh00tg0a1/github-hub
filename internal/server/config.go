package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds server defaults for root path, auth token, and default user grouping.
type Config struct {
	Addr            string `json:"addr"`
	Root            string `json:"root"`
	Token           string `json:"token"`
	DefaultUser     string `json:"default_user"`
	DownloadTimeout string `json:"download_timeout"` // e.g. "10m", "5m"
}

func DefaultConfig() Config {
	return Config{
		Addr:            ":8080",
		Root:            "data",
		DefaultUser:     "default",
		DownloadTimeout: "30m",
	}
}

// LoadConfig loads YAML or JSON config. If path is empty or file missing, returns default config.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if isYAML(path) || looksLikeYAML(string(b)) {
		c, err := parseYAMLConfig(string(b))
		if err != nil {
			return Config{}, fmt.Errorf("yaml parse error: %w", err)
		}
		return c, nil
	}

	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func isYAML(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

func looksLikeYAML(s string) bool {
	trim := strings.TrimSpace(s)
	return strings.HasPrefix(trim, "addr:") || strings.HasPrefix(trim, "root:") || strings.HasPrefix(trim, "token:") || strings.Contains(trim, "default_user")
}

// Minimal YAML parser for the limited schema of Config.
func parseYAMLConfig(s string) (Config, error) {
	cfg := DefaultConfig()
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimRight(raw, "\r")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		kv := strings.SplitN(t, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.Trim(strings.TrimSpace(kv[1]), "\"'")
		switch k {
		case "addr":
			if v != "" {
				cfg.Addr = v
			}
		case "root":
			if v != "" {
				cfg.Root = v
			}
		case "token":
			if v != "" {
				cfg.Token = v
			}
		case "default_user":
			if v != "" {
				cfg.DefaultUser = v
			}
		case "download_timeout":
			if v != "" {
				cfg.DownloadTimeout = v
			}
		}
	}
	return cfg, nil
}
