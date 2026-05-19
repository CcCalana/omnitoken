package anomaly

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

var anomalyFakeRegisterOnce sync.Once
var anomalyFake = &anomalyFakeState{}

type anomalyFakeState struct {
	mu      sync.Mutex
	rows    [][]driver.Value
	err     error
	query   string
	args    []driver.Value
	columns []string
}

func TestPostgresStoreListKeyUsageMapsRows(t *testing.T) {
	start := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	end := start.Add(DefaultWindow)
	db := openAnomalyFakeDB(t, [][]driver.Value{
		{"key-1", "abcdefghijkl", int64(101)},
		{"key-2", nil, int64(2)},
	}, nil)

	got, err := NewPostgresStore(db).ListKeyUsage(context.Background(), start, end)
	if err != nil {
		t.Fatalf("list key usage: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two rows, got %+v", got)
	}
	if got[0].APIKeyID != "key-1" || got[0].APIKeyPrefix != "abcdefghijkl" || got[0].Count != 101 {
		t.Fatalf("unexpected first row: %+v", got[0])
	}
	if got[1].APIKeyPrefix != "" || got[1].Count != 2 {
		t.Fatalf("unexpected second row: %+v", got[1])
	}

	query, args := anomalyFakeSnapshot()
	if !strings.Contains(query, "FROM usage_events ue") ||
		!strings.Contains(query, "LEFT JOIN api_keys ak") ||
		!strings.Contains(query, "ue.created_at >= $1") ||
		!strings.Contains(query, "ue.created_at < $2") ||
		!strings.Contains(query, "GROUP BY ue.api_key_id, ak.key_prefix") {
		t.Fatalf("unexpected anomaly query: %s", query)
	}
	assertAnomalyTimeArg(t, args[0], start)
	assertAnomalyTimeArg(t, args[1], end)
}

func TestPostgresStoreListKeyUsageQueryError(t *testing.T) {
	queryErr := errors.New("query failed")
	db := openAnomalyFakeDB(t, nil, queryErr)

	_, err := NewPostgresStore(db).ListKeyUsage(context.Background(), time.Now(), time.Now())
	if !errors.Is(err, queryErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestPostgresStoreNoopWithoutDB(t *testing.T) {
	t.Parallel()

	got, err := NewPostgresStore(nil).ListKeyUsage(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatalf("expected nil db noop, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil rows for noop, got %+v", got)
	}
}

func openAnomalyFakeDB(t *testing.T, rows [][]driver.Value, err error) *sql.DB {
	t.Helper()
	anomalyFakeRegisterOnce.Do(func() {
		sql.Register("anomaly_fake_postgres", anomalyFakeDriver{})
	})
	anomalyFake.mu.Lock()
	anomalyFake.rows = rows
	anomalyFake.err = err
	anomalyFake.query = ""
	anomalyFake.args = nil
	anomalyFake.columns = []string{"api_key_id", "key_prefix", "call_count"}
	anomalyFake.mu.Unlock()

	db, openErr := sql.Open("anomaly_fake_postgres", "")
	if openErr != nil {
		t.Fatalf("open fake anomaly db: %v", openErr)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close fake anomaly db: %v", err)
		}
	})
	return db
}

func anomalyFakeSnapshot() (string, []driver.Value) {
	anomalyFake.mu.Lock()
	defer anomalyFake.mu.Unlock()
	args := make([]driver.Value, len(anomalyFake.args))
	copy(args, anomalyFake.args)
	return anomalyFake.query, args
}

type anomalyFakeDriver struct{}

func (anomalyFakeDriver) Open(_ string) (driver.Conn, error) {
	return anomalyFakeConn{}, nil
}

type anomalyFakeConn struct{}

func (anomalyFakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("anomaly fake driver does not support prepared statements")
}

func (anomalyFakeConn) Close() error {
	return nil
}

func (anomalyFakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("anomaly fake driver does not support transactions")
}

func (anomalyFakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	anomalyFake.mu.Lock()
	defer anomalyFake.mu.Unlock()

	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	anomalyFake.query = query
	anomalyFake.args = values
	if anomalyFake.err != nil {
		return nil, anomalyFake.err
	}
	return &anomalyFakeRows{columns: append([]string(nil), anomalyFake.columns...), rows: cloneAnomalyRows(anomalyFake.rows)}, nil
}

type anomalyFakeRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *anomalyFakeRows) Columns() []string {
	return r.columns
}

func (r *anomalyFakeRows) Close() error {
	return nil
}

func (r *anomalyFakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func cloneAnomalyRows(rows [][]driver.Value) [][]driver.Value {
	out := make([][]driver.Value, len(rows))
	for i, row := range rows {
		out[i] = append([]driver.Value(nil), row...)
	}
	return out
}

func assertAnomalyTimeArg(t *testing.T, value driver.Value, want time.Time) {
	t.Helper()
	got, ok := value.(time.Time)
	if !ok {
		t.Fatalf("expected time arg, got %T", value)
	}
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
