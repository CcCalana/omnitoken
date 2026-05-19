package quota

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

var quotaFakeRegisterOnce sync.Once
var quotaFake = &quotaFakeState{}

type quotaFakeState struct {
	mu      sync.Mutex
	row     []driver.Value
	err     error
	query   string
	args    []driver.Value
	columns []string
}

func TestPostgresStoreLoadMonthlyBudgetStatusMapsRow(t *testing.T) {
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	db := openQuotaFakeDB(t, []driver.Value{int64(37), int64(38), true}, nil)

	got, err := NewPostgresStore(db).LoadMonthlyBudgetStatus(context.Background(), testSubject(), start, end)
	if err != nil {
		t.Fatalf("load monthly budget status: %v", err)
	}
	if !got.BudgetCents.Valid || got.BudgetCents.Int64 != 37 || got.UsedBudgetCents != 38 || !got.Exhausted {
		t.Fatalf("unexpected budget status: %+v", got)
	}

	query, args := quotaFakeSnapshot()
	for _, want := range []string{
		"FROM users u",
		"LEFT JOIN usage_events ue",
		"LEFT JOIN cost_ledger cl",
		"CEIL(COALESCE(SUM(cl.cost_usd), 0) * 100)",
		"COALESCE(SUM(cl.cost_usd), 0) >= (u.monthly_budget_cents::numeric / 100)",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("quota query missing %q: %s", want, query)
		}
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %#v", args)
	}
	assertQuotaTimeArg(t, args[2], start)
	assertQuotaTimeArg(t, args[3], end)
}

func TestPostgresStoreLoadMonthlyBudgetStatusQueryError(t *testing.T) {
	queryErr := errors.New("query failed")
	db := openQuotaFakeDB(t, nil, queryErr)

	_, err := NewPostgresStore(db).LoadMonthlyBudgetStatus(context.Background(), testSubject(), time.Now(), time.Now())
	if !errors.Is(err, queryErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestNewPostgresStoreWithoutDB(t *testing.T) {
	t.Parallel()

	if got := NewPostgresStore(nil); got != nil {
		t.Fatalf("expected nil store without db, got %#v", got)
	}
}

func openQuotaFakeDB(t *testing.T, row []driver.Value, err error) *sql.DB {
	t.Helper()
	quotaFakeRegisterOnce.Do(func() {
		sql.Register("quota_fake_postgres", quotaFakeDriver{})
	})
	quotaFake.mu.Lock()
	quotaFake.row = row
	quotaFake.err = err
	quotaFake.query = ""
	quotaFake.args = nil
	quotaFake.columns = []string{"monthly_budget_cents", "used_budget_cents", "exhausted"}
	quotaFake.mu.Unlock()

	db, openErr := sql.Open("quota_fake_postgres", "")
	if openErr != nil {
		t.Fatalf("open fake quota db: %v", openErr)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close fake quota db: %v", err)
		}
	})
	return db
}

func quotaFakeSnapshot() (string, []driver.Value) {
	quotaFake.mu.Lock()
	defer quotaFake.mu.Unlock()
	args := make([]driver.Value, len(quotaFake.args))
	copy(args, quotaFake.args)
	return quotaFake.query, args
}

type quotaFakeDriver struct{}

func (quotaFakeDriver) Open(_ string) (driver.Conn, error) {
	return quotaFakeConn{}, nil
}

type quotaFakeConn struct{}

func (quotaFakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("quota fake driver does not support prepared statements")
}

func (quotaFakeConn) Close() error {
	return nil
}

func (quotaFakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("quota fake driver does not support transactions")
}

func (quotaFakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	quotaFake.mu.Lock()
	defer quotaFake.mu.Unlock()

	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	quotaFake.query = query
	quotaFake.args = values
	if quotaFake.err != nil {
		return nil, quotaFake.err
	}
	return &quotaFakeRows{columns: append([]string(nil), quotaFake.columns...), row: append([]driver.Value(nil), quotaFake.row...)}, nil
}

type quotaFakeRows struct {
	columns []string
	row     []driver.Value
	read    bool
}

func (r *quotaFakeRows) Columns() []string {
	return r.columns
}

func (r *quotaFakeRows) Close() error {
	return nil
}

func (r *quotaFakeRows) Next(dest []driver.Value) error {
	if r.read || r.row == nil {
		return io.EOF
	}
	copy(dest, r.row)
	r.read = true
	return nil
}

func assertQuotaTimeArg(t *testing.T, value driver.Value, want time.Time) {
	t.Helper()
	got, ok := value.(time.Time)
	if !ok {
		t.Fatalf("expected time arg, got %T", value)
	}
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
