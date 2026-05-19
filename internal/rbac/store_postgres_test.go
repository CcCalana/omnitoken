package rbac

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

var rbacFakeRegisterOnce sync.Once
var rbacFake = &rbacFakeState{}

type rbacFakeState struct {
	mu      sync.Mutex
	rows    [][]driver.Value
	err     error
	rowErr  error
	query   string
	args    []driver.Value
	columns []string
}

func TestPostgresStoreLookupRolesMapsRows(t *testing.T) {
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	db := openRBACFakeDB(t, [][]driver.Value{
		{StatusActive, "admin"},
		{StatusActive, "viewer"},
	}, nil, nil)

	got, err := NewPostgresStore(db).LookupRoles(context.Background(), Actor{OrgID: orgID, UserID: userID})
	if err != nil {
		t.Fatalf("lookup roles: %v", err)
	}
	if !got.Found || got.Status != StatusActive || len(got.Roles) != 2 {
		t.Fatalf("unexpected lookup: %+v", got)
	}
	if got.Roles[0] != RoleAdmin || got.Roles[1] != RoleViewer {
		t.Fatalf("unexpected roles: %+v", got.Roles)
	}

	query, args := rbacFakeSnapshot()
	for _, want := range []string{
		"FROM users u",
		"LEFT JOIN role_assignments ra",
		"LEFT JOIN roles r",
		"u.organization_id = $1",
		"u.id = $2",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("rbac query missing %q: %s", want, query)
		}
	}
	if len(args) != 2 || fmt.Sprint(args[0]) != orgID.String() || fmt.Sprint(args[1]) != userID.String() {
		t.Fatalf("unexpected lookup args: %#v", args)
	}
}

func TestPostgresStoreLookupRolesHandlesUserWithoutRoles(t *testing.T) {
	db := openRBACFakeDB(t, [][]driver.Value{{StatusActive, nil}}, nil, nil)

	got, err := NewPostgresStore(db).LookupRoles(context.Background(), testActor())
	if err != nil {
		t.Fatalf("lookup roles: %v", err)
	}
	if !got.Found || got.Status != StatusActive || len(got.Roles) != 0 {
		t.Fatalf("unexpected lookup: %+v", got)
	}
}

func TestPostgresStoreLookupRolesHandlesMissingUser(t *testing.T) {
	db := openRBACFakeDB(t, nil, nil, nil)

	got, err := NewPostgresStore(db).LookupRoles(context.Background(), testActor())
	if err != nil {
		t.Fatalf("lookup roles: %v", err)
	}
	if got.Found || got.Status != "" || len(got.Roles) != 0 {
		t.Fatalf("expected empty lookup, got %+v", got)
	}
}

func TestPostgresStoreLookupRolesQueryError(t *testing.T) {
	queryErr := errors.New("query failed")
	db := openRBACFakeDB(t, nil, queryErr, nil)

	_, err := NewPostgresStore(db).LookupRoles(context.Background(), testActor())
	if !errors.Is(err, queryErr) {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestPostgresStoreLookupRolesRowsError(t *testing.T) {
	rowErr := errors.New("rows failed")
	db := openRBACFakeDB(t, [][]driver.Value{{StatusActive, "admin"}}, nil, rowErr)

	_, err := NewPostgresStore(db).LookupRoles(context.Background(), testActor())
	if !errors.Is(err, rowErr) {
		t.Fatalf("expected rows error, got %v", err)
	}
}

func TestNewPostgresStoreWithoutDB(t *testing.T) {
	t.Parallel()

	if got := NewPostgresStore(nil); got != nil {
		t.Fatalf("expected nil store without db, got %#v", got)
	}
}

func openRBACFakeDB(t *testing.T, rows [][]driver.Value, err error, rowErr error) *sql.DB {
	t.Helper()
	rbacFakeRegisterOnce.Do(func() {
		sql.Register("rbac_fake_postgres", rbacFakeDriver{})
	})
	rbacFake.mu.Lock()
	rbacFake.rows = rows
	rbacFake.err = err
	rbacFake.rowErr = rowErr
	rbacFake.query = ""
	rbacFake.args = nil
	rbacFake.columns = []string{"status", "canonical_name"}
	rbacFake.mu.Unlock()

	db, openErr := sql.Open("rbac_fake_postgres", "")
	if openErr != nil {
		t.Fatalf("open fake rbac db: %v", openErr)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close fake rbac db: %v", err)
		}
	})
	return db
}

func rbacFakeSnapshot() (string, []driver.Value) {
	rbacFake.mu.Lock()
	defer rbacFake.mu.Unlock()
	args := make([]driver.Value, len(rbacFake.args))
	copy(args, rbacFake.args)
	return rbacFake.query, args
}

type rbacFakeDriver struct{}

func (rbacFakeDriver) Open(_ string) (driver.Conn, error) {
	return rbacFakeConn{}, nil
}

type rbacFakeConn struct{}

func (rbacFakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("rbac fake driver does not support prepared statements")
}

func (rbacFakeConn) Close() error {
	return nil
}

func (rbacFakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("rbac fake driver does not support transactions")
}

func (rbacFakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	rbacFake.mu.Lock()
	defer rbacFake.mu.Unlock()

	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	rbacFake.query = query
	rbacFake.args = values
	if rbacFake.err != nil {
		return nil, rbacFake.err
	}
	return &rbacFakeRows{
		columns: append([]string(nil), rbacFake.columns...),
		rows:    cloneRBACRows(rbacFake.rows),
		rowErr:  rbacFake.rowErr,
	}, nil
}

type rbacFakeRows struct {
	columns []string
	rows    [][]driver.Value
	rowErr  error
	index   int
}

func (r *rbacFakeRows) Columns() []string {
	return r.columns
}

func (r *rbacFakeRows) Close() error {
	return nil
}

func (r *rbacFakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		if r.rowErr != nil {
			return r.rowErr
		}
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func cloneRBACRows(rows [][]driver.Value) [][]driver.Value {
	out := make([][]driver.Value, len(rows))
	for i, row := range rows {
		out[i] = append([]driver.Value(nil), row...)
	}
	return out
}
