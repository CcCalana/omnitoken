package usage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

var (
	usageFakeDriverOnce sync.Once
	usageFakeMu         sync.Mutex
	usageFakeState      fakeUsageDBState
)

func TestPostgresStoreInsertUsage(t *testing.T) {
	db := openUsageFakeDB(t)
	eventID := uuid.New()
	setUsageFakeState(fakeUsageDBState{queryRow: []driver.Value{eventID.String()}})

	record := UsageRecord{
		RequestID:            "req-1",
		OrganizationID:       uuid.New(),
		UserID:               uuid.New(),
		APIKeyID:             uuid.New(),
		UpstreamCredentialID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		ModelRequested:       "client-model",
		ModelRouted:          "glm-5.1",
		ModelActual:          "glm-5.1",
		ModelFallback:        "ark-code-latest",
		Provider:             "ark",
		StatusCode:           200,
		LatencyMS:            42,
		Tokens: TokenBreakdown{
			PromptTokens:     15,
			CompletionTokens: 2,
			TotalTokens:      17,
		},
	}

	if err := NewPostgresStore(db).InsertUsage(context.Background(), record); err != nil {
		t.Fatalf("insert usage: %v", err)
	}

	state := currentUsageFakeState()
	if !state.committed {
		t.Fatal("expected transaction commit")
	}
	if len(state.execQueries) != 2 {
		t.Fatalf("expected token and cost inserts, got %d", len(state.execQueries))
	}
	if !strings.Contains(state.query, "INSERT INTO usage_events") || !strings.Contains(state.query, "upstream_credential_id") || !strings.Contains(state.query, "model_routed") || !strings.Contains(state.execQueries[1], "INSERT INTO cost_ledger") {
		t.Fatalf("unexpected queries: query=%s exec=%v", state.query, state.execQueries)
	}
}

func TestPostgresStoreInsertUsageDuplicateRequestIDIsNoop(t *testing.T) {
	db := openUsageFakeDB(t)
	setUsageFakeState(fakeUsageDBState{})

	err := NewPostgresStore(db).InsertUsage(context.Background(), UsageRecord{RequestID: "dup", ModelRequested: "m", Provider: "ark"})
	if err != nil {
		t.Fatalf("duplicate request id should be noop: %v", err)
	}
	state := currentUsageFakeState()
	if len(state.execQueries) != 0 {
		t.Fatalf("unexpected exec after duplicate: %v", state.execQueries)
	}
}

func TestPostgresStoreInsertUsageExecError(t *testing.T) {
	db := openUsageFakeDB(t)
	execErr := errors.New("exec failed")
	setUsageFakeState(fakeUsageDBState{
		queryRow: []driver.Value{uuid.New().String()},
		execErr:  execErr,
	})

	err := NewPostgresStore(db).InsertUsage(context.Background(), UsageRecord{RequestID: "req", ModelRequested: "m", Provider: "ark"})
	if !errors.Is(err, execErr) {
		t.Fatalf("expected exec error, got %v", err)
	}
}

func TestPostgresStoreInsertUsageQueryError(t *testing.T) {
	db := openUsageFakeDB(t)
	queryErr := errors.New("query failed")
	setUsageFakeState(fakeUsageDBState{queryErr: queryErr})

	err := NewPostgresStore(db).InsertUsage(context.Background(), UsageRecord{RequestID: "req", ModelRequested: "m", Provider: "ark"})
	if !errors.Is(err, queryErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestPostgresStoreInsertUsageCommitError(t *testing.T) {
	db := openUsageFakeDB(t)
	commitErr := errors.New("commit failed")
	setUsageFakeState(fakeUsageDBState{
		queryRow:  []driver.Value{uuid.New().String()},
		commitErr: commitErr,
	})

	err := NewPostgresStore(db).InsertUsage(context.Background(), UsageRecord{RequestID: "req", ModelRequested: "m", Provider: "ark"})
	if !errors.Is(err, commitErr) {
		t.Fatalf("expected commit error, got %v", err)
	}
}

func openUsageFakeDB(t *testing.T) *sql.DB {
	t.Helper()
	usageFakeDriverOnce.Do(func() {
		sql.Register("usage_fake_postgres", fakeUsageDriver{})
	})
	db, err := sql.Open("usage_fake_postgres", "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func setUsageFakeState(state fakeUsageDBState) {
	usageFakeMu.Lock()
	defer usageFakeMu.Unlock()
	usageFakeState = state
}

func currentUsageFakeState() fakeUsageDBState {
	usageFakeMu.Lock()
	defer usageFakeMu.Unlock()
	return usageFakeState
}

type fakeUsageDBState struct {
	query       string
	queryRow    []driver.Value
	queryErr    error
	execQueries []string
	execErr     error
	commitErr   error
	committed   bool
	rolledBack  bool
}

type fakeUsageDriver struct{}

func (fakeUsageDriver) Open(string) (driver.Conn, error) {
	return fakeUsageConn{}, nil
}

type fakeUsageConn struct{}

func (fakeUsageConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (fakeUsageConn) Close() error {
	return nil
}

func (fakeUsageConn) Begin() (driver.Tx, error) {
	return fakeUsageTx{}, nil
}

func (fakeUsageConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return fakeUsageTx{}, nil
}

func (fakeUsageConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	usageFakeMu.Lock()
	defer usageFakeMu.Unlock()
	usageFakeState.query = query
	if usageFakeState.queryErr != nil {
		return nil, usageFakeState.queryErr
	}
	return &fakeUsageRows{values: usageFakeState.queryRow}, nil
}

func (fakeUsageConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	usageFakeMu.Lock()
	defer usageFakeMu.Unlock()
	usageFakeState.execQueries = append(usageFakeState.execQueries, query)
	if usageFakeState.execErr != nil {
		return nil, usageFakeState.execErr
	}
	return driver.RowsAffected(1), nil
}

type fakeUsageTx struct{}

func (fakeUsageTx) Commit() error {
	usageFakeMu.Lock()
	defer usageFakeMu.Unlock()
	if usageFakeState.commitErr != nil {
		return usageFakeState.commitErr
	}
	usageFakeState.committed = true
	return nil
}

func (fakeUsageTx) Rollback() error {
	usageFakeMu.Lock()
	defer usageFakeMu.Unlock()
	usageFakeState.rolledBack = true
	return nil
}

type fakeUsageRows struct {
	values []driver.Value
	read   bool
}

func (r *fakeUsageRows) Columns() []string {
	return []string{"id"}
}

func (r *fakeUsageRows) Close() error {
	return nil
}

func (r *fakeUsageRows) Next(dest []driver.Value) error {
	if r.read || len(r.values) == 0 {
		return io.EOF
	}
	r.read = true
	copy(dest, r.values)
	return nil
}
