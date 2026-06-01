package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultConcurrency = 10
	defaultRequests    = 10
	defaultGatewayURL  = "http://localhost:8080"
	defaultAdminURL    = "http://localhost:8081"
	defaultModel       = "ark-code-latest"
	defaultMaxRequests = 100
)

type config struct {
	concurrency   int
	requests      int
	gatewayURL    string
	adminURL      string
	key           string
	adminToken    string
	model         string
	timeout       time.Duration
	duration      time.Duration
	maxRequests   int
	allowFailures bool
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	MaxTokens int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type requestResult struct {
	status  int
	code    string
	latency time.Duration
	timeout bool
	err     error
}

type summary struct {
	total       int
	success2xx  int
	client4xx   int
	server5xx   int
	upstream429 int
	timeouts    int
	otherErrors int
	latencies   []time.Duration
	elapsed     time.Duration
	usage       overviewResponse
}

type overviewResponse struct {
	TotalTokens int64 `json:"total_tokens"`
	ActiveUsers int   `json:"active_users"`
}

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadtest: %v\n", err)
		os.Exit(2)
	}

	client := &http.Client{Timeout: cfg.timeout}
	if err := run(context.Background(), cfg, client, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "loadtest: %v\n", err)
		os.Exit(1)
	}
}

func parseConfig(args []string, getenv func(string) string) (config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	cfg := config{
		concurrency: defaultConcurrency,
		requests:    defaultRequests,
		gatewayURL:  defaultGatewayURL,
		adminURL:    defaultAdminURL,
		adminToken:  strings.TrimSpace(getenv("OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN")),
		model:       defaultModel,
		timeout:     15 * time.Second,
		maxRequests: maxRequests(getenv("MAX_REQUESTS")),
	}

	fs := flag.NewFlagSet("loadtest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&cfg.concurrency, "concurrency", cfg.concurrency, "number of concurrent workers")
	fs.IntVar(&cfg.requests, "requests", cfg.requests, "requests per worker")
	fs.StringVar(&cfg.gatewayURL, "gateway", cfg.gatewayURL, "gateway base URL")
	fs.StringVar(&cfg.adminURL, "admin", cfg.adminURL, "admin base URL")
	fs.StringVar(&cfg.key, "key", "", "virtual key for gateway requests")
	fs.StringVar(&cfg.adminToken, "admin-token", cfg.adminToken, "admin bootstrap token for usage verification")
	fs.StringVar(&cfg.model, "model", cfg.model, "model to request")
	fs.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "per-request timeout")
	fs.DurationVar(&cfg.duration, "duration", 0, "run for this duration instead of a fixed requests-per-worker count")
	fs.BoolVar(&cfg.allowFailures, "allow-failures", false, "print summary and exit zero even when some chat requests fail")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	if cfg.concurrency <= 0 {
		return config{}, fmt.Errorf("concurrency must be positive")
	}
	if cfg.requests <= 0 {
		return config{}, fmt.Errorf("requests must be positive")
	}
	if cfg.duration < 0 {
		return config{}, fmt.Errorf("duration must be non-negative")
	}
	if strings.TrimSpace(cfg.key) == "" {
		return config{}, fmt.Errorf("key is required")
	}
	if strings.TrimSpace(cfg.adminToken) == "" {
		return config{}, fmt.Errorf("admin-token is required")
	}
	if _, err := normalizedBaseURL(cfg.gatewayURL); err != nil {
		return config{}, fmt.Errorf("gateway URL: %w", err)
	}
	if _, err := normalizedBaseURL(cfg.adminURL); err != nil {
		return config{}, fmt.Errorf("admin URL: %w", err)
	}
	total := cfg.concurrency * cfg.requests
	if cfg.duration == 0 && total > cfg.maxRequests {
		return config{}, fmt.Errorf("requested %d total requests exceeds MAX_REQUESTS=%d", total, cfg.maxRequests)
	}

	cfg.key = strings.TrimSpace(cfg.key)
	cfg.adminToken = strings.TrimSpace(cfg.adminToken)
	cfg.gatewayURL = strings.TrimRight(cfg.gatewayURL, "/")
	cfg.adminURL = strings.TrimRight(cfg.adminURL, "/")
	return cfg, nil
}

func maxRequests(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultMaxRequests
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultMaxRequests
	}
	return value
}

func run(ctx context.Context, cfg config, client *http.Client, out io.Writer) error {
	if client == nil {
		client = http.DefaultClient
	}
	if out == nil {
		out = io.Discard
	}

	results := make(chan requestResult, cfg.concurrency)
	var wg sync.WaitGroup
	started := time.Now()
	var issued atomic.Int64
	for worker := 0; worker < cfg.concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				requestID, ok := nextRequestID(started, cfg, &issued)
				if !ok {
					return
				}
				results <- sendChatRequest(ctx, client, cfg, workerID, requestID)
			}
		}(worker)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	report := collectSummary(results)
	report.elapsed = time.Since(started)
	usage, err := fetchOverview(ctx, client, cfg)
	if err != nil {
		return err
	}
	if usage.TotalTokens <= 0 || usage.ActiveUsers < 1 {
		return fmt.Errorf("usage assertion failed: total_tokens=%d active_users=%d", usage.TotalTokens, usage.ActiveUsers)
	}
	report.usage = usage
	printSummary(out, report)
	if !cfg.allowFailures && report.success2xx != report.total {
		return fmt.Errorf("chat requests had failures: 2xx=%d total=%d 4xx=%d 5xx=%d upstream_429=%d timeouts=%d errors=%d",
			report.success2xx, report.total, report.client4xx, report.server5xx, report.upstream429, report.timeouts, report.otherErrors)
	}
	return nil
}

func nextRequestID(started time.Time, cfg config, issued *atomic.Int64) (int, bool) {
	next := int(issued.Add(1))
	if cfg.maxRequests > 0 && next > cfg.maxRequests {
		return 0, false
	}
	if cfg.duration > 0 {
		if time.Since(started) >= cfg.duration {
			return 0, false
		}
		return next - 1, true
	}
	total := cfg.concurrency * cfg.requests
	if next > total {
		return 0, false
	}
	return next - 1, true
}

func sendChatRequest(ctx context.Context, client *http.Client, cfg config, workerID int, requestID int) requestResult {
	payload := chatRequest{
		Model: cfg.model,
		Messages: []chatMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf("Reply with pong. worker=%d request=%d", workerID, requestID),
			},
		},
		Stream:    false,
		MaxTokens: 16,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return requestResult{err: fmt.Errorf("marshal chat request: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.gatewayURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return requestResult{err: fmt.Errorf("build chat request: %w", err)}
	}
	req.Header.Set("Authorization", "Bearer "+cfg.key)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return requestResult{latency: latency, timeout: isTimeout(err), err: err}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return requestResult{status: resp.StatusCode, code: errorCode(respBody), latency: latency}
}

func fetchOverview(ctx context.Context, client *http.Client, cfg config) (overviewResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.adminURL+"/api/admin/overview", nil)
	if err != nil {
		return overviewResponse{}, fmt.Errorf("build admin overview request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.adminToken)

	resp, err := client.Do(req)
	if err != nil {
		return overviewResponse{}, fmt.Errorf("fetch admin overview: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return overviewResponse{}, fmt.Errorf("admin overview returned HTTP %d", resp.StatusCode)
	}

	var overview overviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		return overviewResponse{}, fmt.Errorf("decode admin overview: %w", err)
	}
	return overview, nil
}

func collectSummary(results <-chan requestResult) summary {
	report := summary{}
	for result := range results {
		report.total++
		if result.timeout {
			report.timeouts++
			continue
		}
		if result.err != nil {
			report.otherErrors++
			continue
		}
		report.latencies = append(report.latencies, result.latency)
		if result.code == "upstream_429" {
			report.upstream429++
		}
		switch {
		case result.status >= 200 && result.status < 300:
			report.success2xx++
		case result.status >= 400 && result.status < 500:
			report.client4xx++
		case result.status >= 500:
			report.server5xx++
		default:
			report.otherErrors++
		}
	}
	return report
}

func printSummary(out io.Writer, report summary) {
	successRate := 0.0
	if report.total > 0 {
		successRate = float64(report.success2xx) / float64(report.total) * 100
	}

	fmt.Fprintln(out, "OmniToken loadtest summary")
	fmt.Fprintf(out, "total_requests\t%d\n", report.total)
	fmt.Fprintf(out, "elapsed\t%s\n", report.elapsed.Round(time.Millisecond))
	fmt.Fprintf(out, "rps\t%.1f\n", requestsPerSecond(report.total, report.elapsed))
	fmt.Fprintf(out, "success_rate\t%.1f%%\n", successRate)
	fmt.Fprintf(out, "2xx\t%d\n", report.success2xx)
	fmt.Fprintf(out, "4xx\t%d\n", report.client4xx)
	fmt.Fprintf(out, "5xx\t%d\n", report.server5xx)
	fmt.Fprintf(out, "upstream_429\t%d\n", report.upstream429)
	fmt.Fprintf(out, "timeouts\t%d\n", report.timeouts)
	fmt.Fprintf(out, "errors\t%d\n", report.otherErrors)
	fmt.Fprintf(out, "avg_latency\t%s\n", averageLatency(report.latencies).Round(time.Millisecond))
	fmt.Fprintf(out, "p50_latency\t%s\n", percentileLatency(report.latencies, 0.50).Round(time.Millisecond))
	fmt.Fprintf(out, "p95_latency\t%s\n", percentileLatency(report.latencies, 0.95).Round(time.Millisecond))
	fmt.Fprintf(out, "p99_latency\t%s\n", percentileLatency(report.latencies, 0.99).Round(time.Millisecond))
	fmt.Fprintf(out, "max_latency\t%s\n", maxLatency(report.latencies).Round(time.Millisecond))
	fmt.Fprintf(out, "runtime_goroutines_final\t%d\n", runtime.NumGoroutine())
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Fprintf(out, "runtime_heap_alloc_final_bytes\t%d\n", mem.HeapAlloc)
	fmt.Fprintf(out, "usage_total_tokens\t%d\n", report.usage.TotalTokens)
}

func requestsPerSecond(total int, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(total) / elapsed.Seconds()
}

func averageLatency(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	var total time.Duration
	for _, value := range values {
		total += value
	}
	return total / time.Duration(len(values))
}

func percentileLatency(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i int, j int) bool { return sorted[i] < sorted[j] })
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func maxLatency(values []time.Duration) time.Duration {
	var max time.Duration
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func errorCode(body []byte) string {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ""
	}
	return envelope.Error.Code
}

func normalizedBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("must include scheme and host")
	}
	return parsed, nil
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
