package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

const (
	defaultGatewayURL    = "http://localhost:8080"
	defaultAdminURL      = "http://localhost:8081"
	defaultOrganization  = "00000000-0000-0000-0000-000000000001"
	defaultProject       = "00000000-0000-0000-0000-000000000101"
	defaultModel         = "chat-fast"
	defaultMaxRequests   = 50
	defaultMaxTokens     = 32
	defaultDuration      = 5 * time.Minute
	defaultClientTimeout = 30 * time.Second
)

type config struct {
	GatewayURL     string
	AdminURL       string
	AdminToken     string
	DatabaseURL    string
	DeepSeekAPIKey string
	OrganizationID string
	ProjectID      string
	Model          string
	MaxRequests    int
	MaxTokens      int
	Duration       time.Duration
	ViewerEmail    string
	ViewerPassword string
	MemberEmail    string
	MemberPassword string
}

type adminUsersResponse struct {
	Users []adminUser `json:"users"`
}

type adminUser struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
}

type createVirtualKeyResponse struct {
	APIKeyID   string `json:"api_key_id"`
	UserID     string `json:"user_id"`
	KeyPrefix  string `json:"key_prefix"`
	VirtualKey string `json:"virtual_key"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type auditLog struct {
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	StatusCode   int    `json:"status_code"`
}

type preparedUser struct {
	User    adminUser
	APIKey  createVirtualKeyResponse
	Budget0 bool
}

type requestResult struct {
	UserID     string
	APIKeyID   string
	StatusCode int
	Streaming  bool
	Budget0    bool
	Err        error
}

type ledgerSummary struct {
	UsageEvents    int
	LedgerCostUSD  float64
	DerivedCostUSD float64
	MissingFields  int
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "estimated cost ~= ¥%.4f (max_requests=%d, max_tokens=%d)\n", estimateCostCNY(cfg.MaxRequests, cfg.MaxTokens), cfg.MaxRequests, cfg.MaxTokens)

	client := &http.Client{Timeout: defaultClientTimeout}
	if cfg.DeepSeekAPIKey != "" {
		if err := preflightDeepSeek(ctx, client, cfg.DeepSeekAPIKey, cfg.MaxTokens); err != nil {
			return err
		}
		fmt.Fprintln(out, "deepseek preflight ok")
	}

	users, err := fetchAdminUsers(ctx, client, cfg)
	if err != nil {
		return err
	}
	targets, err := selectSeedUsers(users)
	if err != nil {
		return err
	}

	prepared, err := prepareUsers(ctx, client, cfg, targets)
	if err != nil {
		return err
	}

	if err := runRBACChecks(ctx, client, cfg, prepared, out); err != nil {
		return err
	}

	runStart := time.Now().UTC()
	results := runConcurrentRequests(ctx, client, cfg, prepared)
	runEnd := time.Now().UTC().Add(2 * time.Second)
	if err := assertRequestResults(results); err != nil {
		return err
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	summary, err := verifyLedger(ctx, db, prepared, runStart, runEnd)
	if err != nil {
		return err
	}
	if summary.UsageEvents == 0 {
		return fmt.Errorf("ledger verification found no usage_events for prepared keys")
	}
	if summary.MissingFields > 0 {
		return fmt.Errorf("ledger attribution missing required fields in %d rows", summary.MissingFields)
	}
	if !withinOnePercent(summary.LedgerCostUSD, summary.DerivedCostUSD) {
		return fmt.Errorf("ledger closure mismatch: ledger=%.10f derived=%.10f", summary.LedgerCostUSD, summary.DerivedCostUSD)
	}

	fmt.Fprintf(out, "ok: requests=%d usage_events=%d ledger=%.10f derived=%.10f\n", len(results), summary.UsageEvents, summary.LedgerCostUSD, summary.DerivedCostUSD)
	return nil
}

func parseConfig(args []string) (config, error) {
	cfg := config{
		GatewayURL:     getenv("OMNITOKEN_GATEWAY_URL", defaultGatewayURL),
		AdminURL:       getenv("OMNITOKEN_ADMIN_URL", defaultAdminURL),
		AdminToken:     os.Getenv("OMNITOKEN_ADMIN_TOKEN"),
		DatabaseURL:    os.Getenv("OMNITOKEN_TEST_DATABASE_URL"),
		DeepSeekAPIKey: os.Getenv("OMNITOKEN_DEEPSEEK_API_KEY"),
		OrganizationID: getenv("OMNITOKEN_TEST_ORGANIZATION_ID", defaultOrganization),
		ProjectID:      getenv("OMNITOKEN_TEST_PROJECT_ID", defaultProject),
		Model:          getenv("OMNITOKEN_TEST_MODEL", defaultModel),
		MaxRequests:    getenvInt("MAX_REQUESTS", defaultMaxRequests),
		MaxTokens:      getenvInt("OMNITOKEN_TEST_MAX_TOKENS", defaultMaxTokens),
		Duration:       getenvDuration("OMNITOKEN_TEST_DURATION", defaultDuration),
		ViewerEmail:    os.Getenv("OMNITOKEN_VIEWER_EMAIL"),
		ViewerPassword: os.Getenv("OMNITOKEN_VIEWER_PASSWORD"),
		MemberEmail:    os.Getenv("OMNITOKEN_MEMBER_EMAIL"),
		MemberPassword: os.Getenv("OMNITOKEN_MEMBER_PASSWORD"),
	}
	fs := flag.NewFlagSet("e2e-runner", flag.ContinueOnError)
	fs.StringVar(&cfg.GatewayURL, "gateway-url", cfg.GatewayURL, "gateway base URL")
	fs.StringVar(&cfg.AdminURL, "admin-url", cfg.AdminURL, "admin base URL")
	fs.StringVar(&cfg.AdminToken, "admin-token", cfg.AdminToken, "admin bearer/bootstrap token")
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres URL for read-only verification")
	fs.StringVar(&cfg.DeepSeekAPIKey, "deepseek-api-key", cfg.DeepSeekAPIKey, "DeepSeek API key for preflight only")
	fs.StringVar(&cfg.OrganizationID, "organization-id", cfg.OrganizationID, "seed organization UUID")
	fs.StringVar(&cfg.ProjectID, "project-id", cfg.ProjectID, "seed project UUID")
	fs.StringVar(&cfg.Model, "model", cfg.Model, "virtual model name")
	fs.IntVar(&cfg.MaxRequests, "max-requests", cfg.MaxRequests, "total request cap")
	fs.IntVar(&cfg.MaxTokens, "max-tokens", cfg.MaxTokens, "max output tokens per request")
	fs.DurationVar(&cfg.Duration, "duration", cfg.Duration, "target concurrency duration")
	fs.StringVar(&cfg.ViewerEmail, "viewer-email", cfg.ViewerEmail, "optional viewer email for RBAC check")
	fs.StringVar(&cfg.ViewerPassword, "viewer-password", cfg.ViewerPassword, "optional viewer password for RBAC check")
	fs.StringVar(&cfg.MemberEmail, "member-email", cfg.MemberEmail, "optional member email for RBAC check")
	fs.StringVar(&cfg.MemberPassword, "member-password", cfg.MemberPassword, "optional member password for RBAC check")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func validateConfig(cfg config) error {
	if cfg.AdminToken == "" {
		return fmt.Errorf("--admin-token or OMNITOKEN_ADMIN_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("--database-url or OMNITOKEN_TEST_DATABASE_URL is required")
	}
	if cfg.MaxRequests < 30 {
		return fmt.Errorf("--max-requests must be at least 30")
	}
	if cfg.MaxTokens <= 0 {
		return fmt.Errorf("--max-tokens must be positive")
	}
	if cfg.Duration < 0 {
		return fmt.Errorf("--duration must be non-negative")
	}
	for name, raw := range map[string]string{"gateway-url": cfg.GatewayURL, "admin-url": cfg.AdminURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("--%s must be an absolute URL", name)
		}
	}
	return nil
}

func fetchAdminUsers(ctx context.Context, client *http.Client, cfg config) ([]adminUser, error) {
	var response adminUsersResponse
	if err := adminJSON(ctx, client, cfg.AdminURL, cfg.AdminToken, http.MethodGet, "/api/admin/users", nil, http.StatusOK, &response); err != nil {
		return nil, err
	}
	return response.Users, nil
}

func selectSeedUsers(users []adminUser) ([]adminUser, error) {
	byEmail := make(map[string]adminUser, len(users))
	for _, user := range users {
		byEmail[strings.ToLower(user.Email)] = user
	}
	selected := make([]adminUser, 0, 10)
	for i := 1; i <= 10; i++ {
		email := fmt.Sprintf("user%02d@democorp.local", i)
		user, ok := byEmail[email]
		if !ok {
			return nil, fmt.Errorf("seed user %s not found", email)
		}
		selected = append(selected, user)
	}
	return selected, nil
}

func prepareUsers(ctx context.Context, client *http.Client, cfg config, users []adminUser) ([]preparedUser, error) {
	prepared := make([]preparedUser, 0, len(users))
	for i, user := range users {
		budget := int64(100 + i)
		budget0 := i == 0
		if budget0 {
			budget = 0
		}
		if err := setBudget(ctx, client, cfg, user.UserID, budget); err != nil {
			return nil, err
		}
		key, err := createVirtualKey(ctx, client, cfg, user.UserID)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedUser{User: user, APIKey: key, Budget0: budget0})
	}
	return prepared, nil
}

func setBudget(ctx context.Context, client *http.Client, cfg config, userID string, budget int64) error {
	body := map[string]int64{"budget_cents": budget}
	return adminJSON(ctx, client, cfg.AdminURL, cfg.AdminToken, http.MethodPatch, "/api/admin/users/"+userID+"/quota", body, http.StatusOK, nil)
}

func createVirtualKey(ctx context.Context, client *http.Client, cfg config, userID string) (createVirtualKeyResponse, error) {
	body := map[string]string{
		"organization_id": cfg.OrganizationID,
		"project_id":      cfg.ProjectID,
		"user_id":         userID,
	}
	var response createVirtualKeyResponse
	err := adminJSON(ctx, client, cfg.AdminURL, cfg.AdminToken, http.MethodPost, "/api/admin/dev/virtual-keys", body, http.StatusCreated, &response)
	return response, err
}

func runRBACChecks(ctx context.Context, client *http.Client, cfg config, prepared []preparedUser, out io.Writer) error {
	if cfg.ViewerEmail == "" || cfg.ViewerPassword == "" {
		fmt.Fprintln(out, "SKIP: viewer credentials not provided")
	} else {
		token, err := login(ctx, client, cfg.AdminURL, cfg.ViewerEmail, cfg.ViewerPassword)
		if err != nil {
			return err
		}
		body := map[string]int64{"budget_cents": 1}
		if err := adminJSON(ctx, client, cfg.AdminURL, token, http.MethodPatch, "/api/admin/users/"+prepared[1].User.UserID+"/quota", body, http.StatusForbidden, nil); err != nil {
			return fmt.Errorf("viewer RBAC check: %w", err)
		}
		viewerUserID := userIDByEmail(prepared, cfg.ViewerEmail)
		if viewerUserID == "" {
			return fmt.Errorf("viewer %s is not one of the prepared seed users", cfg.ViewerEmail)
		}
		if err := verifyForbiddenAudit(ctx, client, cfg, viewerUserID, "user_quota", "update_quota"); err != nil {
			return err
		}
	}
	if cfg.MemberEmail == "" || cfg.MemberPassword == "" {
		fmt.Fprintln(out, "SKIP: member credentials not provided")
		return nil
	}
	token, err := login(ctx, client, cfg.AdminURL, cfg.MemberEmail, cfg.MemberPassword)
	if err != nil {
		return err
	}
	body := map[string]string{"name": "t100-forbidden", "real_model": "deepseek-v4-flash", "provider": "deepseek"}
	if err := adminJSON(ctx, client, cfg.AdminURL, token, http.MethodPost, "/api/admin/virtual-models", body, http.StatusForbidden, nil); err != nil {
		return fmt.Errorf("member RBAC check: %w", err)
	}
	return nil
}

func login(ctx context.Context, client *http.Client, adminURL, email, password string) (string, error) {
	body := map[string]string{"email": email, "password": password}
	var response loginResponse
	if err := adminJSON(ctx, client, adminURL, "", http.MethodPost, "/api/admin/login", body, http.StatusOK, &response); err != nil {
		return "", err
	}
	if response.Token == "" {
		return "", fmt.Errorf("login returned empty token")
	}
	return response.Token, nil
}

func runConcurrentRequests(ctx context.Context, client *http.Client, cfg config, users []preparedUser) []requestResult {
	perUser := cfg.MaxRequests / len(users)
	if perUser < 3 {
		perUser = 3
	}
	interval := time.Duration(0)
	if perUser > 1 && cfg.Duration > 0 {
		interval = cfg.Duration / time.Duration(perUser-1)
	}
	results := make(chan requestResult, len(users)*perUser)
	var wg sync.WaitGroup
	for _, user := range users {
		user := user
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perUser; i++ {
				streaming := i == 0 || i%2 == 1
				results <- sendGatewayRequest(ctx, client, cfg, user, streaming)
				if interval > 0 && i < perUser-1 {
					timer := time.NewTimer(interval)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
			}
		}()
	}
	wg.Wait()
	close(results)
	out := make([]requestResult, 0, len(users)*perUser)
	for result := range results {
		out = append(out, result)
	}
	return out
}

func userIDByEmail(users []preparedUser, email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, user := range users {
		if strings.ToLower(user.User.Email) == email {
			return user.User.UserID
		}
	}
	return ""
}

func sendGatewayRequest(ctx context.Context, client *http.Client, cfg config, user preparedUser, streaming bool) requestResult {
	payload := map[string]any{
		"model":       cfg.Model,
		"max_tokens":  cfg.MaxTokens,
		"stream":      streaming,
		"messages":    []map[string]string{{"role": "user", "content": "Return the word ok."}},
		"temperature": 0,
	}
	if streaming {
		payload["stream_options"] = map[string]bool{"include_usage": true}
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return requestResult{UserID: user.User.UserID, APIKeyID: user.APIKey.APIKeyID, Streaming: streaming, Budget0: user.Budget0, Err: err}
	}
	endpoint := strings.TrimRight(cfg.GatewayURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return requestResult{UserID: user.User.UserID, APIKeyID: user.APIKey.APIKeyID, Streaming: streaming, Budget0: user.Budget0, Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+user.APIKey.VirtualKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return requestResult{UserID: user.User.UserID, APIKeyID: user.APIKey.APIKeyID, Streaming: streaming, Budget0: user.Budget0, Err: err}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return requestResult{UserID: user.User.UserID, APIKeyID: user.APIKey.APIKeyID, StatusCode: resp.StatusCode, Streaming: streaming, Budget0: user.Budget0}
}

func assertRequestResults(results []requestResult) error {
	var nonBudget, nonBudgetOK, budget, budget402, gateway5xx int
	for _, result := range results {
		if result.Err != nil {
			return result.Err
		}
		if result.StatusCode >= 500 {
			gateway5xx++
		}
		if result.Budget0 {
			budget++
			if result.StatusCode == http.StatusPaymentRequired {
				budget402++
			}
			continue
		}
		nonBudget++
		if result.StatusCode >= 200 && result.StatusCode <= 299 {
			nonBudgetOK++
		}
	}
	if gateway5xx > 0 {
		return fmt.Errorf("gateway 5xx responses: %d", gateway5xx)
	}
	if budget != budget402 {
		return fmt.Errorf("budget=0 responses: got %d/%d HTTP 402", budget402, budget)
	}
	if nonBudget == 0 || float64(nonBudgetOK)/float64(nonBudget) < 0.90 {
		return fmt.Errorf("non-budget success below 90%%: %d/%d", nonBudgetOK, nonBudget)
	}
	return nil
}

func verifyLedger(ctx context.Context, db *sql.DB, prepared []preparedUser, start, end time.Time) (ledgerSummary, error) {
	keyIDs := make([]string, 0, len(prepared))
	expectedUsers := make(map[string]string, len(prepared))
	for _, user := range prepared {
		if user.Budget0 {
			continue
		}
		keyIDs = append(keyIDs, user.APIKey.APIKeyID)
		expectedUsers[user.User.UserID] = user.APIKey.APIKeyID
	}
	sort.Strings(keyIDs)
	query := `
SELECT
  COUNT(*)::bigint,
  COALESCE(SUM(cl.cost_usd), 0)::float8,
  COALESCE(SUM(
    ((utb.prompt_tokens::numeric / 1000000) * mpc.input_rate_usd)
    + ((utb.completion_tokens::numeric / 1000000) * mpc.output_rate_usd)
    + ((utb.reasoning_tokens::numeric / 1000000) * mpc.reasoning_rate_usd)
    + ((utb.cached_tokens::numeric / 1000000) * mpc.cached_input_rate_usd)
  ), 0)::float8,
  COUNT(*) FILTER (
    WHERE ue.user_id IS NULL
      OR ue.api_key_id IS NULL
      OR COALESCE(ue.model_routed, '') = ''
      OR ue.upstream_credential_id IS NULL
  )::bigint
FROM usage_events ue
JOIN usage_token_breakdown utb ON utb.usage_event_id = ue.id
JOIN cost_ledger cl ON cl.usage_event_id = ue.id
LEFT JOIN model_catalog mc
  ON mc.provider = ue.provider
  AND (
    (ue.model_actual <> '' AND mc.provider_model = ue.model_actual)
    OR (ue.model_routed <> '' AND mc.canonical_model = ue.model_routed)
    OR (ue.model_requested <> '' AND mc.canonical_model = ue.model_requested)
  )
LEFT JOIN model_pricing_current mpc ON mpc.model_id = mc.id
WHERE ue.api_key_id = ANY($1)
  AND ue.created_at >= $2
  AND ue.created_at < $3
  AND ue.status_code BETWEEN 200 AND 299`
	var summary ledgerSummary
	var count, missing int64
	if err := db.QueryRowContext(ctx, query, pq.Array(keyIDs), start.UTC(), end.UTC()).Scan(&count, &summary.LedgerCostUSD, &summary.DerivedCostUSD, &missing); err != nil {
		return ledgerSummary{}, fmt.Errorf("query ledger summary: %w", err)
	}
	summary.UsageEvents = int(count)
	summary.MissingFields = int(missing)
	if err := verifyAttributionSamples(ctx, db, expectedUsers, start, end); err != nil {
		return ledgerSummary{}, err
	}
	return summary, nil
}

func verifyForbiddenAudit(ctx context.Context, client *http.Client, cfg config, actorID, resourceType, action string) error {
	path := "/api/admin/audit-logs?actor_id=" + url.QueryEscape(actorID) + "&resource_type=" + url.QueryEscape(resourceType) + "&limit=5"
	var logs []auditLog
	if err := adminJSON(ctx, client, cfg.AdminURL, cfg.AdminToken, http.MethodGet, path, nil, http.StatusOK, &logs); err != nil {
		return fmt.Errorf("query forbidden audit log: %w", err)
	}
	for _, entry := range logs {
		if entry.Action == action && entry.ResourceType == resourceType && entry.StatusCode == http.StatusForbidden {
			return nil
		}
	}
	return fmt.Errorf("forbidden audit log not found for actor %s action %s", actorID, action)
}

func verifyAttributionSamples(ctx context.Context, db *sql.DB, expectedUsers map[string]string, start, end time.Time) error {
	userIDs := make([]string, 0, len(expectedUsers))
	for userID := range expectedUsers {
		userIDs = append(userIDs, userID)
	}
	sort.Strings(userIDs)
	if len(userIDs) > 3 {
		userIDs = userIDs[:3]
	}
	for _, userID := range userIDs {
		query := `
SELECT api_key_id::text, COALESCE(model_routed, ''), upstream_credential_id::text
FROM usage_events
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
  AND status_code BETWEEN 200 AND 299
ORDER BY created_at DESC
LIMIT 1`
		var apiKeyID, modelRouted, upstreamCredentialID string
		if err := db.QueryRowContext(ctx, query, userID, start.UTC(), end.UTC()).Scan(&apiKeyID, &modelRouted, &upstreamCredentialID); err != nil {
			return fmt.Errorf("query attribution sample for user %s: %w", userID, err)
		}
		if apiKeyID != expectedUsers[userID] || modelRouted == "" || upstreamCredentialID == "" {
			return fmt.Errorf("bad attribution for user %s", userID)
		}
	}
	return nil
}

func adminJSON(ctx context.Context, client *http.Client, baseURL, token, method, path string, body any, want int, out any) error {
	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
		reader = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, reader)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: got HTTP %d want %d: %s", method, path, resp.StatusCode, want, strings.TrimSpace(string(data)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

func preflightDeepSeek(ctx context.Context, client *http.Client, key string, maxTokens int) error {
	payload := map[string]any{
		"model":      "deepseek-chat",
		"max_tokens": maxTokens,
		"messages":   []map[string]string{{"role": "user", "content": "ok"}},
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.deepseek.com/chat/completions", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("deepseek preflight: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("deepseek preflight returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func estimateCostCNY(maxRequests, maxTokens int) float64 {
	// T-100 uses the task-level conservative estimate: ¥1/M input + ¥2/M output.
	const promptTokens = 16
	return (float64(maxRequests*promptTokens)/1000000)*1 + (float64(maxRequests*maxTokens)/1000000)*2
}

func withinOnePercent(a, b float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	denom := math.Max(math.Abs(a), math.Abs(b))
	return math.Abs(a-b)/denom <= 0.01
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(raw, "%d", &parsed); err != nil || parsed == 0 {
		return fallback
	}
	return parsed
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
