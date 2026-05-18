package audit

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

var auditFakeRegisterOnce sync.Once
var auditFake = &auditFakeState{}

type auditFakeState struct {
	mu    sync.Mutex
	query string
	args  []driver.Value
	err   error
}

func TestPostgresStoreInsertAuditMapsFields(t *testing.T) {
	db := openAuditFakeDB(t)
	createdAt := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)

	err := NewPostgresStore(db).InsertAudit(context.Background(), Record{
		ActorID:      "bootstrap",
		ActorType:    ActorTypeBootstrap,
		Action:       "create_virtual_key",
		ResourceType: "virtual_key",
		ResourceID:   "key-1",
		Before:       nil,
		After:        []byte(`{"key_prefix":"abcdefghijkl"}`),
		IP:           net.ParseIP("192.0.2.20"),
		UserAgent:    "audit-test",
		RequestID:    "req-1",
		StatusCode:   201,
		CreatedAt:    createdAt,
	})
	if err != nil {
		t.Fatalf("insert audit: %v", err)
	}

	query, args := auditFakeSnapshot()
	if !strings.Contains(query, "INSERT INTO audit_logs") ||
		!strings.Contains(query, "actor_id") ||
		!strings.Contains(query, "actor_type") ||
		!strings.Contains(query, `"before"`) ||
		!strings.Contains(query, `"after"`) ||
		!strings.Contains(query, "request_id") {
		t.Fatalf("audit insert query missing expected columns: %s", query)
	}
	if len(args) != 12 {
		t.Fatalf("expected 12 args, got %d: %#v", len(args), args)
	}
	if args[0] != "bootstrap" || args[1] != ActorTypeBootstrap || args[2] != "create_virtual_key" {
		t.Fatalf("unexpected actor/action args: %#v", args[:3])
	}
	if args[5] != nil {
		t.Fatalf("expected nil before arg, got %#v", args[5])
	}
	if args[6] != `{"key_prefix":"abcdefghijkl"}` {
		t.Fatalf("unexpected after arg: %#v", args[6])
	}
	if args[7] != "192.0.2.20" || args[10] != int64(201) && args[10] != 201 {
		t.Fatalf("unexpected ip/status args: %#v", args)
	}
	if got, ok := args[11].(time.Time); !ok || !got.Equal(createdAt) {
		t.Fatalf("unexpected created_at arg: %T %[1]v", args[11])
	}
}

func TestPostgresStoreInsertAuditExecError(t *testing.T) {
	db := openAuditFakeDB(t)
	execErr := errors.New("exec failed")
	auditFake.mu.Lock()
	auditFake.err = execErr
	auditFake.mu.Unlock()

	err := NewPostgresStore(db).InsertAudit(context.Background(), Record{})
	if !errors.Is(err, execErr) {
		t.Fatalf("expected exec error, got %v", err)
	}
}

func TestPostgresStoreNoopWithoutDB(t *testing.T) {
	t.Parallel()

	if err := NewPostgresStore(nil).InsertAudit(context.Background(), Record{}); err != nil {
		t.Fatalf("expected nil db noop, got %v", err)
	}
}

func openAuditFakeDB(t *testing.T) *sql.DB {
	t.Helper()
	auditFakeRegisterOnce.Do(func() {
		sql.Register("audit_fake_postgres", auditFakeDriver{})
	})
	auditFake.mu.Lock()
	auditFake.query = ""
	auditFake.args = nil
	auditFake.err = nil
	auditFake.mu.Unlock()

	db, err := sql.Open("audit_fake_postgres", "")
	if err != nil {
		t.Fatalf("open fake audit db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close fake audit db: %v", err)
		}
	})
	return db
}

func auditFakeSnapshot() (string, []driver.Value) {
	auditFake.mu.Lock()
	defer auditFake.mu.Unlock()
	args := make([]driver.Value, len(auditFake.args))
	copy(args, auditFake.args)
	return auditFake.query, args
}

type auditFakeDriver struct{}

func (auditFakeDriver) Open(_ string) (driver.Conn, error) {
	return auditFakeConn{}, nil
}

type auditFakeConn struct{}

func (auditFakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("audit fake driver does not support prepared statements")
}

func (auditFakeConn) Close() error {
	return nil
}

func (auditFakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("audit fake driver does not support transactions")
}

func (auditFakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	auditFake.mu.Lock()
	defer auditFake.mu.Unlock()

	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	auditFake.query = query
	auditFake.args = values
	if auditFake.err != nil {
		return nil, auditFake.err
	}
	return driver.RowsAffected(1), nil
}
