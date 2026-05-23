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

func TestPostgresStoreCreateEncryptsAndReturnsPublicCredential(t *testing.T) {
	db := openCredentialFakeDB(t)
	envelope := testEnvelope(t)
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	setCredentialFakeState(fakeCredentialDBState{rows: [][]driver.Value{{
		uuid.New().String(), "deepseek", "https://api.deepseek.com/v1", "", int64(3), int64(1),
		StatusActive, HealthHealthy, "", []byte(`{}`), []byte(`{"alias":"deepseek-a"}`), now, now,
	}}})

	item, err := NewPostgresStore(db, envelope).Create(context.Background(), CreateParams{
		Provider: "deepseek",
		Alias:    "deepseek-a",
		BaseURL:  "https://api.deepseek.com/v1",
		Secret:   "secret-value",
		Priority: 3,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if item.Provider != "deepseek" || item.Status != StatusActive || string(item.Metadata) != `{"alias":"deepseek-a"}` {
		t.Fatalf("unexpected created public credential: %+v", item)
	}
	state := currentCredentialFakeState()
	if !strings.Contains(state.query, "INSERT INTO upstream_credentials") || strings.Contains(state.query, "secret-value") {
		t.Fatalf("unexpected create query: %s", state.query)
	}
	if len(state.args) < 4 {
		t.Fatalf("expected create args, got %#v", state.args)
	}
	encrypted, ok := state.args[3].([]byte)
	if !ok || bytes.Contains(encrypted, []byte("secret-value")) {
		t.Fatalf("secret was not encrypted before storage: %#v", state.args[3])
	}
}

func TestPostgresStoreCreateDetectsAliasConflict(t *testing.T) {
	db := openCredentialFakeDB(t)
	setCredentialFakeState(fakeCredentialDBState{})

	_, err := NewPostgresStore(db, testEnvelope(t)).Create(context.Background(), CreateParams{
		Provider: "ark",
		Alias:    "ark-a",
		BaseURL:  "https://ark.example/v3",
		Secret:   "secret",
		Priority: 1,
	})
	if !errors.Is(err, ErrAliasExists) {
		t.Fatalf("expected alias conflict, got %v", err)
	}
}

func TestPostgresStoreDisableReturnsPublicCredential(t *testing.T) {
	db := openCredentialFakeDB(t)
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	id := uuid.New().String()
	setCredentialFakeState(fakeCredentialDBState{rows: [][]driver.Value{{
		id, "ark", "https://ark.example/v3", "", int64(1), int64(1),
		StatusDisabled, HealthHealthy, "", []byte(`{}`), []byte(`{"alias":"ark-a"}`), now, now,
	}}})

	item, err := NewPostgresStore(db, nil).Disable(context.Background(), id)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if item.ID != id || item.Status != StatusDisabled {
		t.Fatalf("unexpected disabled credential: %+v", item)
	}
	if !strings.Contains(currentCredentialFakeState().query, "SET status = 'disabled'") {
		t.Fatalf("unexpected disable query: %s", currentCredentialFakeState().query)
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
	args  []driver.Value
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

func (fakeCredentialConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	credentialFakeMu.Lock()
	defer credentialFakeMu.Unlock()
	credentialFakeState.query = query
	credentialFakeState.args = make([]driver.Value, len(args))
	for i, arg := range args {
		credentialFakeState.args[i] = arg.Value
	}
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
