package credentials

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	omnicrypto "github.com/omnitoken/omnitoken/internal/crypto"
)

var (
	credentialFakeOnce  sync.Once
	credentialFakeMu    sync.Mutex
	credentialFakeState fakeCredentialDBState
)

func TestPostgresStoreLoadDecryptsCredentials(t *testing.T) {
	db := openCredentialFakeDB(t)
	envelope := testEnvelope(t)
	encrypted, err := envelope.Encrypt([]byte("ark-secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	id := uuid.New().String()
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	setCredentialFakeState(fakeCredentialDBState{rows: [][]driver.Value{{
		id, "ark", "https://ark.example/v3", encrypted, "cn", int64(10), int64(2),
		StatusActive, HealthHealthy, "", []byte(`{"rpm":7}`), []byte(`{"alias":"ark-a"}`), now, now,
	}}})

	items, err := NewPostgresStore(db, envelope).Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	got := items[0]
	if got.ID != id || got.Secret != "ark-secret" || got.Weight != 2 || string(got.Metadata) != `{"alias":"ark-a"}` {
		t.Fatalf("unexpected credential: %+v", got)
	}
	if !strings.Contains(currentCredentialFakeState().query, "encrypted_secret") {
		t.Fatalf("load query should include encrypted_secret: %s", currentCredentialFakeState().query)
	}
}

func TestPostgresStoreListPublicOmitsEncryptedSecret(t *testing.T) {
	db := openCredentialFakeDB(t)
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	setCredentialFakeState(fakeCredentialDBState{rows: [][]driver.Value{{
		uuid.New().String(), "ark", "https://ark.example/v3", "cn", int64(10), int64(1),
		StatusActive, HealthHealthy, "", []byte(`{}`), []byte(`{"alias":"ark-a"}`), now, now,
	}}})

	items, err := NewPostgresStore(db, nil).ListPublic(context.Background())
	if err != nil {
		t.Fatalf("list public: %v", err)
	}
	if len(items) != 1 || items[0].Provider != "ark" || string(items[0].Metadata) != `{"alias":"ark-a"}` {
		t.Fatalf("unexpected public items: %+v", items)
	}
	if strings.Contains(currentCredentialFakeState().query, "encrypted_secret") {
		t.Fatalf("public query exposed encrypted secret: %s", currentCredentialFakeState().query)
	}
}

func TestPostgresStoreLoadHandlesErrors(t *testing.T) {
	db := openCredentialFakeDB(t)
	envelope := testEnvelope(t)
	queryErr := errors.New("query failed")
	setCredentialFakeState(fakeCredentialDBState{err: queryErr})
	if _, err := NewPostgresStore(db, envelope).Load(context.Background()); !errors.Is(err, queryErr) {
		t.Fatalf("expected query error, got %v", err)
	}

	setCredentialFakeState(fakeCredentialDBState{rows: [][]driver.Value{{
		uuid.New().String(), "ark", "https://ark.example/v3", []byte("bad"), "cn", int64(10), int64(1),
		StatusActive, HealthHealthy, "", []byte(`{}`), []byte(`{}`), time.Now(), time.Now(),
	}}})
	if _, err := NewPostgresStore(db, envelope).Load(context.Background()); err == nil {
		t.Fatal("expected decrypt error")
	}
}

func TestPostgresStoreNilReceivers(t *testing.T) {
	items, err := (*PostgresStore)(nil).Load(context.Background())
	if err != nil || items != nil {
		t.Fatalf("nil load = %+v err=%v", items, err)
	}
	itemsPublic, err := (*PostgresStore)(nil).ListPublic(context.Background())
	if err != nil || itemsPublic != nil {
		t.Fatalf("nil list = %+v err=%v", itemsPublic, err)
	}
}

func testEnvelope(t *testing.T) *omnicrypto.Envelope {
	t.Helper()
	env, err := omnicrypto.NewEnvelope(bytes.Repeat([]byte{0x42}, omnicrypto.MasterKeySize))
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	return env
}

func openCredentialFakeDB(t *testing.T) *sql.DB {
	t.Helper()
	credentialFakeOnce.Do(func() {
		sql.Register("credential_fake_postgres", fakeCredentialDriver{})
	})
	db, err := sql.Open("credential_fake_postgres", "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func setCredentialFakeState(state fakeCredentialDBState) {
	credentialFakeMu.Lock()
	defer credentialFakeMu.Unlock()
	credentialFakeState = state
}

func currentCredentialFakeState() fakeCredentialDBState {
	credentialFakeMu.Lock()
	defer credentialFakeMu.Unlock()
	return credentialFakeState
}

type fakeCredentialDBState struct {
	query string
	rows  [][]driver.Value
	err   error
}

type fakeCredentialDriver struct{}

func (fakeCredentialDriver) Open(string) (driver.Conn, error) {
	return fakeCredentialConn{}, nil
}

type fakeCredentialConn struct{}

func (fakeCredentialConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (fakeCredentialConn) Close() error { return nil }

func (fakeCredentialConn) Begin() (driver.Tx, error) { return nil, errors.New("begin not supported") }

func (fakeCredentialConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	credentialFakeMu.Lock()
	defer credentialFakeMu.Unlock()
	credentialFakeState.query = query
	if credentialFakeState.err != nil {
		return nil, credentialFakeState.err
	}
	return &fakeCredentialRows{rows: append([][]driver.Value(nil), credentialFakeState.rows...)}, nil
}

type fakeCredentialRows struct {
	rows [][]driver.Value
	pos  int
}

func (r *fakeCredentialRows) Columns() []string {
	count := 1
	if len(r.rows) > 0 {
		count = len(r.rows[0])
	}
	columns := make([]string, count)
	for i := range columns {
		columns[i] = "col"
	}
	return columns
}

func (r *fakeCredentialRows) Close() error { return nil }

func (r *fakeCredentialRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}
