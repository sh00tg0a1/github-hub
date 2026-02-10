// Load testing tool for quality-server webhook endpoint
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Statistics for load testing
type Stats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalBytes      int64
	MinLatency      time.Duration
	MaxLatency      time.Duration
	latencies       []time.Duration
	mu              sync.Mutex
}

// Load test configuration
type Config struct {
	ServerURL      string
	EventType      string
	Concurrent     int
	TotalRequests  int
	Timeout        time.Duration
	QPS            int // Queries per second (0 = unlimited)
}

// Webhook payloads
var pushPayload = []byte(`{
    "ref": "refs/heads/main",
    "repository": {
        "id": 123456789,
        "node_id": "MDEwOlJlcG9zaXRvcnkxMjM0NTY3ODk=",
        "name": "TestRepo",
        "full_name": "testuser/TestRepo",
        "private": false,
        "owner": {
            "login": "testuser",
            "id": 1234567,
            "type": "User"
        },
        "html_url": "https://github.com/testuser/TestRepo",
        "description": "测试仓库",
        "url": "https://api.github.com/repos/testuser/TestRepo",
        "default_branch": "main"
    },
    "sender": {
        "login": "testuser",
        "id": 1234567,
        "type": "User"
    },
    "pusher": {
        "name": "testuser",
        "email": "testuser@example.com"
    },
    "head_commit": {
        "id": "abc123def4567890abcdef1234567890abcdef12",
        "tree_id": "def1234567890abcdef1234567890abcdef1234",
        "distinct": true,
        "message": "Load test commit",
        "timestamp": "2026-02-07T10:00:00Z",
        "url": "https://github.com/testuser/TestRepo/commit/abc123d",
        "author": {
            "name": "Test User",
            "email": "testuser@example.com",
            "username": "testuser"
        },
        "committer": {
            "name": "Test User",
            "email": "testuser@example.com",
            "username": "testuser"
        },
        "added": ["src/file.go"],
        "removed": [],
        "modified": ["README.md"]
    },
    "commits": []
}`)

var prPayload = []byte(`{
    "action": "opened",
    "number": 42,
    "pull_request": {
        "id": 987654321,
        "node_id": "MDExOlB1bGxSZXF1ZXN0OTg3NjU0MzIx",
        "html_url": "https://github.com/testuser/TestRepo/pull/42",
        "number": 42,
        "state": "open",
        "title": "feat: Load test PR",
        "body": "Load testing PR",
        "user": {
            "login": "contributor",
            "id": 7654321,
            "type": "User"
        },
        "base": {
            "label": "testuser:main",
            "ref": "main",
            "sha": "1234567890abcdef1234567890abcdef12345678",
            "repo": {
                "id": 123456789,
                "url": "https://api.github.com/repos/testuser/TestRepo",
                "name": "TestRepo",
                "full_name": "testuser/TestRepo"
            }
        },
        "head": {
            "label": "contributor:feature/load-test",
            "ref": "feature/load-test",
            "sha": "abcdef1234567890abcdef1234567890abcdef12",
            "repo": {
                "id": 123456789,
                "url": "https://api.github.com/repos/testuser/TestRepo",
                "name": "TestRepo",
                "full_name": "testuser/TestRepo"
            },
            "user": {
                "login": "contributor",
                "id": 7654321
            }
        },
        "merged": false,
        "mergeable": true,
        "mergeable_state": "clean"
    },
    "repository": {
        "id": 123456789,
        "node_id": "MDEwOlJlcG9zaXRvcnkxMjM0NTY3ODk=",
        "name": "TestRepo",
        "full_name": "testuser/TestRepo",
        "private": false,
        "owner": {
            "login": "testuser",
            "id": 1234567,
            "type": "User"
        },
        "html_url": "https://github.com/testuser/TestRepo",
        "description": "测试仓库",
        "url": "https://api.github.com/repos/testuser/TestRepo",
        "default_branch": "main"
    },
    "sender": {
        "login": "contributor",
        "id": 7654321,
        "type": "User"
    }
}`)

func getPayload(eventType string) []byte {
	switch eventType {
	case "push":
		return pushPayload
	case "pr":
		return prPayload
	default:
		return pushPayload
	}
}

func sendRequest(client *http.Client, url string, eventType string, stats *Stats) {
	payload := getPayload(eventType)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		atomic.AddInt64(&stats.FailedRequests, 1)
		atomic.AddInt64(&stats.TotalRequests, 1)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("X-GitHub-Delivery", fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid()))

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		atomic.AddInt64(&stats.FailedRequests, 1)
		atomic.AddInt64(&stats.TotalRequests, 1)
		return
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	atomic.AddInt64(&stats.TotalBytes, int64(len(body)))

	stats.mu.Lock()
	stats.latencies = append(stats.latencies, latency)
	if stats.MinLatency == 0 || latency < stats.MinLatency {
		stats.MinLatency = latency
	}
	if latency > stats.MaxLatency {
		stats.MaxLatency = latency
	}
	stats.mu.Unlock()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		atomic.AddInt64(&stats.SuccessRequests, 1)
	} else {
		atomic.AddInt64(&stats.FailedRequests, 1)
	}
	atomic.AddInt64(&stats.TotalRequests, 1)
}

func worker(client *http.Client, url string, eventType string, stats *Stats, requests int, rateLimiter <-chan time.Time) {
	for i := 0; i < requests; i++ {
		if rateLimiter != nil {
			<-rateLimiter
		}
		sendRequest(client, url, eventType, stats)
	}
}

func runLoadTest(config Config) *Stats {
	stats := &Stats{
		latencies:  make([]time.Duration, 0, config.TotalRequests),
		MinLatency: time.Hour,
	}

	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	var rateLimiter <-chan time.Time
	if config.QPS > 0 {
		rateLimiter = time.Tick(time.Second / time.Duration(config.QPS))
	}

	requestsPerWorker := config.TotalRequests / config.Concurrent
	remaining := config.TotalRequests % config.Concurrent

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < config.Concurrent; i++ {
		wg.Add(1)
		workerRequests := requestsPerWorker
		if i < remaining {
			workerRequests++
		}

		go func() {
			defer wg.Done()
			worker(client, config.ServerURL+"/webhook", config.EventType, stats, workerRequests, rateLimiter)
		}()
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Printf("\n\n")
	fmt.Println("========================================")
	fmt.Println("  Load Test Results")
	fmt.Println("========================================")
	fmt.Printf("Server URL:       %s\n", config.ServerURL)
	fmt.Printf("Event Type:       %s\n", config.EventType)
	fmt.Printf("Total Requests:   %d\n", config.TotalRequests)
	fmt.Printf("Concurrent:       %d\n", config.Concurrent)
	if config.QPS > 0 {
		fmt.Printf("Rate Limit:       %d QPS\n", config.QPS)
	}
	fmt.Printf("Total Duration:   %v\n", duration)
	fmt.Printf("\n")

	success := atomic.LoadInt64(&stats.SuccessRequests)
	failed := atomic.LoadInt64(&stats.FailedRequests)
	totalBytes := atomic.LoadInt64(&stats.TotalBytes)

	fmt.Printf("Results:\n")
	fmt.Printf("  Success:         %d\n", success)
	fmt.Printf("  Failed:          %d\n", failed)
	fmt.Printf("  Success Rate:    %.2f%%\n", float64(success)*100/float64(config.TotalRequests))
	fmt.Printf("  Throughput:      %.2f req/s\n", float64(config.TotalRequests)/duration.Seconds())
	fmt.Printf("  Data Transferred: %.2f MB\n", float64(totalBytes)/(1024*1024))
	fmt.Printf("\n")

	if len(stats.latencies) > 0 {
		// Calculate percentiles
		sorted := make([]time.Duration, len(stats.latencies))
		copy(sorted, stats.latencies)

		// Simple bubble sort (good enough for small datasets)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		p50 := sorted[len(sorted)*50/100]
		p90 := sorted[len(sorted)*90/100]
		p95 := sorted[len(sorted)*95/100]
		p99 := sorted[len(sorted)*99/100]

		fmt.Printf("Latency:\n")
		fmt.Printf("  Min:             %v\n", stats.MinLatency)
		fmt.Printf("  Max:             %v\n", stats.MaxLatency)
		fmt.Printf("  Average:         %v\n", duration/time.Duration(config.TotalRequests))
		fmt.Printf("  P50 (Median):    %v\n", p50)
		fmt.Printf("  P90:             %v\n", p90)
		fmt.Printf("  P95:             %v\n", p95)
		fmt.Printf("  P99:             %v\n", p99)
		fmt.Printf("\n")
	}

	return stats
}

func main() {
	config := Config{
		ServerURL:     os.Getenv("QUALITY_SERVER_URL"),
		EventType:     "push",
		Concurrent:    10,
		TotalRequests: 100,
		Timeout:       30 * time.Second,
		QPS:           0,
	}

	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "-url":
				if i+1 < len(os.Args) {
					config.ServerURL = os.Args[i+1]
					i++
				}
			case "-type":
				if i+1 < len(os.Args) {
					config.EventType = os.Args[i+1]
					i++
				}
			case "-c", "-concurrent":
				if i+1 < len(os.Args) {
					fmt.Sscanf(os.Args[i+1], "%d", &config.Concurrent)
					i++
				}
			case "-n", "-requests":
				if i+1 < len(os.Args) {
					fmt.Sscanf(os.Args[i+1], "%d", &config.TotalRequests)
					i++
				}
			case "-qps":
				if i+1 < len(os.Args) {
					fmt.Sscanf(os.Args[i+1], "%d", &config.QPS)
					i++
				}
			case "-timeout":
				if i+1 < len(os.Args) {
					timeoutSec, _ := fmt.Sscanf(os.Args[i+1], "%d", &config.Timeout)
					if timeoutSec > 0 {
						config.Timeout = time.Duration(timeoutSec) * time.Second
					}
					i++
				}
			case "-h", "--help":
				fmt.Println("Load Testing Tool for quality-server")
				fmt.Println("\nUsage:")
				fmt.Println("  loadtest [options]")
				fmt.Println("\nOptions:")
				fmt.Println("  -url <url>           Server URL (default: $QUALITY_SERVER_URL or http://localhost:5001)")
				fmt.Println("  -type <type>         Event type: push or pr (default: push)")
				fmt.Println("  -c, -concurrent <n>  Concurrent connections (default: 10)")
				fmt.Println("  -n, -requests <n>    Total requests (default: 100)")
				fmt.Println("  -qps <n>             Rate limit in queries per second (default: unlimited)")
				fmt.Println("  -timeout <seconds>   Request timeout (default: 30)")
				fmt.Println("  -h, --help           Show this help")
				fmt.Println("\nExamples:")
				fmt.Println("  # Basic load test")
				fmt.Println("  ./loadtest -url http://localhost:5001 -n 1000 -c 50")
				fmt.Println("\n  # Test with rate limit")
				fmt.Println("  ./loadtest -url http://localhost:5001 -n 500 -c 20 -qps 100")
				fmt.Println("\n  # Test PR events")
				fmt.Println("  ./loadtest -url http://localhost:5001 -type pr -n 200 -c 10")
				fmt.Println("\n  # Stress test")
				fmt.Println("  ./loadtest -url http://localhost:5001 -n 10000 -c 100")
				os.Exit(0)
			}
		}
	}

	if config.ServerURL == "" {
		config.ServerURL = os.Getenv("QUALITY_SERVER_URL")
		if config.ServerURL == "" {
			config.ServerURL = "http://localhost:5001"
		}
	}

	if config.EventType != "push" && config.EventType != "pr" {
		config.EventType = "push"
	}

	fmt.Println("========================================")
	fmt.Println("  Quality Server Load Test")
	fmt.Println("========================================")
	fmt.Printf("Target:     %s\n", config.ServerURL)
	fmt.Printf("Event:      %s\n", config.EventType)
	fmt.Printf("Requests:   %d\n", config.TotalRequests)
	fmt.Printf("Concurrent: %d\n", config.Concurrent)
	if config.QPS > 0 {
		fmt.Printf("Rate Limit: %d QPS\n", config.QPS)
	}
	fmt.Println("========================================")
	fmt.Println("Starting load test...")
	fmt.Println("========================================")

	runLoadTest(config)
}
