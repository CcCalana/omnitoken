package auth

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/google/uuid"
)

var (
	fakeDriverOnce sync.Once
	fakeQueryMu    sync.Mutex
	fakeQueryRow   []driver.Value
	fakeQueryErr   error
)

func TestPostgresStoreLookupVirtualKey(t *testing.T) {
	db := openFakeDB(t)
	store := NewPostgresStore(db)
	apiKeyID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()
	hash := HashSecret("secret")

	setFakeQuery(row(apiKeyID, orgID, userID, hash, "active"), nil)

	record, err := store.LookupVirtualKey(context.Background(), "abcdefghijkl")
	if err != nil {
		t.Fatalf("lookup virtual key: %v", err)
	}
	if record.APIKeyID != apiKeyID || record.OrgID != orgID || record.UserID != userID || record.Status != "active" {
		t.Fatalf("record mismatch: %#v", record)
	}
	if !equalBytes(record.KeyHash, hash) {
		t.Fatalf("hash mismatch: %#v", record.KeyHash)
	}
}

func TestPostgresStoreLookupVirtualKeyNotFound(t *testing.T) {
	db := openFakeDB(t)
	store := NewPostgresStore(db)
	setFakeQuery(nil, nil)

	if _, err := store.LookupVirtualKey(context.Background(), "missing"); !errors.Is(err, ErrVirtualKeyNotFound) {
		t.Fatalf("expected ErrVirtualKeyNotFound, got %v", err)
	}
}

func TestPostgresStoreLookupVirtualKeyQueryError(t *testing.T) {
	db := openFakeDB(t)
	store := NewPostgresStore(db)
	queryErr := errors.New("query failed")
	setFakeQuery(nil, queryErr)

	if _, err := store.LookupVirtualKey(context.Background(), "abcdefghijkl"); !errors.Is(err, queryErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func openFakeDB(t *testing.T) *sql.DB {
	t.Helper()

	fakeDriverOnce.Do(func() {
		sql.Register("auth_fake_postgres", fakeDriver{})
	})
	db, err := sql.Open("auth_fake_postgres", "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func setFakeQuery(values []driver.Value, err error) {
	fakeQueryMu.Lock()
	defer fakeQueryMu.Unlock()
	fakeQueryRow = values
	fakeQueryErr = err
}

func row(apiKeyID uuid.UUID, orgID uuid.UUID, userID uuid.UUID, hash []byte, status string) []driver.Value {
	return []driver.Value{apiKeyID.String(), orgID.String(), userID.String(), hash, status}
}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return fakeConn{}, nil
}

type fakeConn struct{}

func (fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (fakeConn) Close() error {
	return nil
}

func (fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not supported")
}

func (fakeConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	fakeQueryMu.Lock()
	defer fakeQueryMu.Unlock()
	if fakeQueryErr != nil {
		return nil, fakeQueryErr
	}
	return &fakeRows{values: fakeQueryRow}, nil
}

type fakeRows struct {
	values []driver.Value
	read   bool
}

func (r *fakeRows) Columns() []string {
	return []string{"id", "organization_id", "user_id", "key_hash", "status"}
}

func (r *fakeRows) Close() error {
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.read || len(r.values) == 0 {
		return io.EOF
	}
	r.read = true
	copy(dest, r.values)
	return nil
}
