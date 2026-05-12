package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/config"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body.Status != "ok" || body.Service != "admin" {
		t.Fatalf("unexpected health body: %+v", body)
	}
}

func TestOverviewFallsBackToZeroWithoutStore(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	rec := httptest.NewRecorder()

	makeOverviewHandler(testLogger(), nil, func() time.Time { return now }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"trend":[]`) || !strings.Contains(rec.Body.String(), `"model_usage":[]`) {
		t.Fatalf("expected empty JSON arrays, got %s", rec.Body.String())
	}

	var body overviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode overview response: %v", err)
	}
	if body.Period != "2026-05" {
		t.Fatalf("unexpected period: %q", body.Period)
	}
	if body.TotalTokens != 0 || body.EstimatedCostUSD != 0 || body.ActiveUsers != 0 || body.QuotaWarningUsers != 0 {
		t.Fatalf("expected zero overview, got %+v", body)
	}
	if len(body.Trend) != 0 || len(body.ModelUsage) != 0 {
		t.Fatalf("expected empty overview series, got %+v", body)
	}
}

func TestOverviewReturnsStoreData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	store := &fakeOverviewStore{
		response: overviewResponse{
			Period:            "2026-05",
			TotalTokens:       126,
			EstimatedCostUSD:  0.003,
			ActiveUsers:       2,
			QuotaWarningUsers: 0,
			Trend: []dailyTokenUsage{
				{Date: "2026-05-11", Tokens: 126, Cost: 0.003},
			},
			ModelUsage: []modelUsage{
				{Model: "ark-code-latest", Tokens: 126, Cost: 0.003, Share: 1},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	rec := httptest.NewRecorder()

	makeOverviewHandler(testLogger(), store, func() time.Time { return now }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !store.called || !store.now.Equal(now) {
		t.Fatalf("store was not called with fixed now: called=%v now=%s", store.called, store.now)
	}

	var body overviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode overview response: %v", err)
	}
	if body.TotalTokens != 126 || body.EstimatedCostUSD != 0.003 || body.ActiveUsers != 2 {
		t.Fatalf("unexpected overview response: %+v", body)
	}
	if len(body.ModelUsage) != 1 || body.ModelUsage[0].Share != 1 {
		t.Fatalf("unexpected model usage: %+v", body.ModelUsage)
	}
}

func TestOverviewStoreErrorReturns500(t *testing.T) {
	t.Parallel()

	store := &fakeOverviewStore{err: errors.New("database unavailable")}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	rec := httptest.NewRecorder()

	makeOverviewHandler(testLogger(), store, func() time.Time {
		return time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"]["code"] != "overview_query_failed" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestPostgresOverviewStoreLoadOverviewMapsSQLResults(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"total_tokens", "estimated_cost_usd", "active_users"},
			rows: [][]driver.Value{
				{int64(300), float64(1.25), int64(2)},
			},
		},
		{
			columns: []string{"usage_date", "total_tokens", "cost_usd"},
			rows: [][]driver.Value{
				{"2026-05-11", int64(180), float64(0.75)},
				{"2026-05-12", int64(120), float64(0.50)},
			},
		},
		{
			columns: []string{"model", "total_tokens", "cost_usd"},
			rows: [][]driver.Value{
				{"ark-code-latest", int64(200), float64(0.80)},
				{"ark-lite", int64(100), float64(0.45)},
			},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadOverview(context.Background(), now)
	if err != nil {
		t.Fatalf("load overview: %v", err)
	}

	if got.Period != "2026-05" || got.TotalTokens != 300 || got.EstimatedCostUSD != 1.25 || got.ActiveUsers != 2 {
		t.Fatalf("unexpected overview summary: %+v", got)
	}
	if len(got.Trend) != 2 || got.Trend[0].Date != "2026-05-11" || got.Trend[1].Tokens != 120 {
		t.Fatalf("unexpected trend: %+v", got.Trend)
	}
	if len(got.ModelUsage) != 2 {
		t.Fatalf("expected two model usage rows, got %+v", got.ModelUsage)
	}
	if got.ModelUsage[0].Model != "ark-code-latest" || got.ModelUsage[0].Share != float64(200)/float64(300) {
		t.Fatalf("unexpected first model usage row: %+v", got.ModelUsage[0])
	}
	if got.ModelUsage[1].Share != float64(100)/float64(300) {
		t.Fatalf("unexpected second model usage row: %+v", got.ModelUsage[1])
	}

	queries := fakeAdminSQLSnapshot()
	if len(queries) != 3 {
		t.Fatalf("expected 3 queries, got %d", len(queries))
	}
	monthStart, monthEnd := monthWindow(now)
	assertTimeArg(t, queries[0].args[0], monthStart)
	assertTimeArg(t, queries[0].args[1], monthEnd)
	assertTimeArg(t, queries[1].args[0], now.AddDate(0, 0, -30))
	assertTimeArg(t, queries[1].args[1], now)
	if !strings.Contains(queries[0].query, "usage_events ue") || !strings.Contains(queries[0].query, "ue.created_at >= $1") {
		t.Fatalf("summary query should filter on usage_events.created_at: %s", queries[0].query)
	}
	if !strings.Contains(queries[1].query, "GROUP BY (ue.created_at AT TIME ZONE 'UTC')::date") {
		t.Fatalf("trend query should group by UTC date: %s", queries[1].query)
	}
	if !strings.Contains(queries[2].query, "COALESCE(NULLIF(ue.model_actual, ''), ue.model_requested, 'unknown')") {
		t.Fatalf("model query should group by model fallback expression: %s", queries[2].query)
	}
}

func TestPostgresOverviewStoreLoadOverviewGuardsZeroTokenShare(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	db := openFakeAdminDB(t, []adminFakeSQLResponse{
		{
			columns: []string{"total_tokens", "estimated_cost_usd", "active_users"},
			rows: [][]driver.Value{
				{int64(0), float64(0), int64(0)},
			},
		},
		{
			columns: []string{"usage_date", "total_tokens", "cost_usd"},
			rows:    [][]driver.Value{},
		},
		{
			columns: []string{"model", "total_tokens", "cost_usd"},
			rows: [][]driver.Value{
				{"ark-code-latest", int64(0), float64(0)},
			},
		},
	})
	store := &postgresOverviewStore{db: db}

	got, err := store.LoadOverview(context.Background(), now)
	if err != nil {
		t.Fatalf("load overview: %v", err)
	}
	if len(got.ModelUsage) != 1 {
		t.Fatalf("expected one model usage row, got %+v", got.ModelUsage)
	}
	if got.ModelUsage[0].Share != 0 {
		t.Fatalf("expected zero share when total tokens is zero, got %+v", got.ModelUsage[0])
	}
}

func TestOverviewCORSPreflight(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/overview", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatal("expected CORS header")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PATCH, DELETE, OPTIONS" {
		t.Fatalf("unexpected CORS methods: %q", got)
	}
}

func TestOverviewCORSDeniesUnlistedOrigin(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/overview", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unexpected CORS header: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "" {
		t.Fatalf("unexpected allowed headers: %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestDevVirtualKeyEndpointDisabledWithoutBootstrapToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", nil)
	rec := httptest.NewRecorder()

	newMux(testLogger(), testAdminConfig(), nil, &fakeVirtualKeyCreator{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestDevVirtualKeyEndpointRequiresBootstrapToken(t *testing.T) {
	t.Parallel()

	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, &fakeVirtualKeyCreator{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"]["code"] != "invalid_api_key" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestDevVirtualKeyEndpointCreatesKey(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	createdAt := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	creator := &fakeVirtualKeyCreator{
		result: createVirtualKeyResult{
			APIKeyID:       uuid.New(),
			OrganizationID: orgID,
			UserID:         userID,
			KeyPrefix:      "abcdefghijkl",
			VirtualKey:     "omt_abcdefghijkl_secret",
			CreatedAt:      createdAt,
		},
	}
	cfg := testAdminConfig()
	cfg.BootstrapToken = "dev-bootstrap"
	body := `{"organization_id":"` + orgID.String() + `","user_id":"` + userID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/dev/virtual-keys", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer dev-bootstrap")
	rec := httptest.NewRecorder()

	newMux(testLogger(), cfg, nil, creator).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if creator.params.OrganizationID != orgID || creator.params.UserID != userID {
		t.Fatalf("creator params mismatch: %#v", creator.params)
	}
	var response createVirtualKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.DevOnly {
		t.Fatal("expected dev_only=true")
	}
	if response.VirtualKey != creator.result.VirtualKey || response.KeyPrefix != creator.result.KeyPrefix {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testAdminConfig() config.AdminConfig {
	return config.AdminConfig{
		Addr:        ":8081",
		CORSOrigins: []string{"http://localhost:3000"},
		CORSMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
	}
}

type fakeVirtualKeyCreator struct {
	params createVirtualKeyParams
	result createVirtualKeyResult
	err    error
}

func (f *fakeVirtualKeyCreator) CreateVirtualKey(_ context.Context, params createVirtualKeyParams) (createVirtualKeyResult, error) {
	f.params = params
	return f.result, f.err
}

type fakeOverviewStore struct {
	response overviewResponse
	err      error
	called   bool
	now      time.Time
}

func (f *fakeOverviewStore) LoadOverview(_ context.Context, now time.Time) (overviewResponse, error) {
	f.called = true
	f.now = now
	return f.response, f.err
}

func assertTimeArg(t *testing.T, value driver.Value, want time.Time) {
	t.Helper()

	got, ok := value.(time.Time)
	if !ok {
		t.Fatalf("expected time arg %s, got %T %[2]v", want, value)
	}
	if !got.Equal(want) {
		t.Fatalf("expected time arg %s, got %s", want, got)
	}
}

var adminFakeSQL = &adminFakeSQLState{}
var adminFakeSQLRegisterOnce sync.Once

type adminFakeSQLState struct {
	mu        sync.Mutex
	responses []adminFakeSQLResponse
	queries   []adminFakeSQLQuery
}

type adminFakeSQLResponse struct {
	columns []string
	rows    [][]driver.Value
	err     error
}

type adminFakeSQLQuery struct {
	query string
	args  []driver.Value
}

func openFakeAdminDB(t *testing.T, responses []adminFakeSQLResponse) *sql.DB {
	t.Helper()

	adminFakeSQLRegisterOnce.Do(func() {
		sql.Register("admin_fake_postgres", adminFakeDriver{})
	})

	adminFakeSQL.mu.Lock()
	adminFakeSQL.responses = responses
	adminFakeSQL.queries = nil
	adminFakeSQL.mu.Unlock()

	db, err := sql.Open("admin_fake_postgres", "")
	if err != nil {
		t.Fatalf("open fake admin db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close fake admin db: %v", err)
		}
	})
	return db
}

func fakeAdminSQLSnapshot() []adminFakeSQLQuery {
	adminFakeSQL.mu.Lock()
	defer adminFakeSQL.mu.Unlock()

	queries := make([]adminFakeSQLQuery, len(adminFakeSQL.queries))
	for i, query := range adminFakeSQL.queries {
		args := make([]driver.Value, len(query.args))
		copy(args, query.args)
		queries[i] = adminFakeSQLQuery{query: query.query, args: args}
	}
	return queries
}

type adminFakeDriver struct{}

func (adminFakeDriver) Open(_ string) (driver.Conn, error) {
	return adminFakeConn{}, nil
}

type adminFakeConn struct{}

func (adminFakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("admin fake driver does not support prepared statements")
}

func (adminFakeConn) Close() error {
	return nil
}

func (adminFakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("admin fake driver does not support transactions")
}

func (adminFakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	adminFakeSQL.mu.Lock()
	defer adminFakeSQL.mu.Unlock()

	index := len(adminFakeSQL.queries)
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	adminFakeSQL.queries = append(adminFakeSQL.queries, adminFakeSQLQuery{query: query, args: values})

	if index >= len(adminFakeSQL.responses) {
		return nil, errors.New("unexpected query")
	}
	response := adminFakeSQL.responses[index]
	if response.err != nil {
		return nil, response.err
	}
	return &adminFakeRows{
		columns: response.columns,
		rows:    response.rows,
	}, nil
}

type adminFakeRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *adminFakeRows) Columns() []string {
	return r.columns
}

func (r *adminFakeRows) Close() error {
	return nil
}

func (r *adminFakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
