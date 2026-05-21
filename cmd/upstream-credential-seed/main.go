package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	omnicrypto "github.com/omnitoken/omnitoken/internal/crypto"
)

const (
	defaultProvider                 = "ark"
	defaultBaseURL                  = "https://ark.cn-beijing.volces.com/api/coding/v3"
	actionCreateUpstreamCredential  = "create_upstream_credential"
	actionUpdateUpstreamCredential  = "update_upstream_credential"
	actionDisableUpstreamCredential = "disable_upstream_credential"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func runCLI(args []string, getenv func(string) string, stdout io.Writer, stderr io.Writer) int {
	opts := seedOptions{}
	flags := flag.NewFlagSet("omnitoken-seed-upstream", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.databaseURL, "database-url", "", "Postgres URL; defaults to OMNITOKEN_DATABASE_URL")
	flags.StringVar(&opts.masterKeyFile, "master-key-file", "", "Master key file; defaults to OMNITOKEN_MASTER_KEY_FILE")
	flags.StringVar(&opts.masterKey, "master-key", "", "Master key hex; defaults to OMNITOKEN_MASTER_KEY")
	flags.StringVar(&opts.baseURL, "base-url", "", "Ark OpenAI-compatible base URL; defaults to OMNITOKEN_ARK_OPENAI_BASE_URL")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if opts.databaseURL == "" {
		opts.databaseURL = strings.TrimSpace(getenv("OMNITOKEN_DATABASE_URL"))
	}
	if opts.masterKeyFile == "" {
		opts.masterKeyFile = strings.TrimSpace(getenv("OMNITOKEN_MASTER_KEY_FILE"))
	}
	if opts.masterKey == "" {
		opts.masterKey = strings.TrimSpace(getenv("OMNITOKEN_MASTER_KEY"))
	}
	if opts.baseURL == "" {
		opts.baseURL = strings.TrimSpace(getenv("OMNITOKEN_ARK_OPENAI_BASE_URL"))
	}
	if opts.baseURL == "" {
		opts.baseURL = defaultBaseURL
	}
	opts.keys = arkKeysFromEnv(getenv)

	if len(opts.keys) == 0 {
		fmt.Fprintln(stdout, "loaded 0 upstream credentials")
		return 0
	}
	if opts.databaseURL == "" {
		fmt.Fprintln(stderr, "missing -database-url or OMNITOKEN_DATABASE_URL")
		return 2
	}

	masterKey, err := omnicrypto.LoadMasterKey(opts.masterKeyFile, opts.masterKey)
	if err != nil {
		if errors.Is(err, omnicrypto.ErrMasterKeyMissing) || errors.Is(err, omnicrypto.ErrInvalidMasterKey) {
			fmt.Fprintln(stderr, "invalid master key configuration")
			return 2
		}
		fmt.Fprintln(stderr, "failed to read master key")
		return 1
	}
	envelope, err := omnicrypto.NewEnvelope(masterKey)
	if err != nil {
		fmt.Fprintln(stderr, "invalid master key configuration")
		return 2
	}

	db, err := sql.Open("postgres", opts.databaseURL)
	if err != nil {
		fmt.Fprintln(stderr, "open postgres failed")
		return 1
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	count, err := seed(ctx, db, envelope, opts)
	if err != nil {
		fmt.Fprintf(stderr, "seed upstream credentials: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "loaded %d upstream credentials\n", count)
	return 0
}

type seedOptions struct {
	databaseURL   string
	masterKeyFile string
	masterKey     string
	baseURL       string
	keys          []string
}

func arkKeysFromEnv(getenv func(string) string) []string {
	values := []string{}
	for _, value := range strings.Split(getenv("OMNITOKEN_ARK_KEYS"), ",") {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values = append(values, trimmed)
		}
	}

	numbered := []string{}
	for i := 1; i <= 20; i++ {
		value := strings.TrimSpace(getenv("OMNITOKEN_ARK_KEYS_" + strconv.Itoa(i)))
		if value != "" {
			numbered = append(numbered, value)
		}
	}
	values = append(values, numbered...)
	return values
}

func seed(ctx context.Context, db *sql.DB, envelope *omnicrypto.Envelope, opts seedOptions) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	existing, err := loadSeededCredentials(ctx, tx, defaultProvider)
	if err != nil {
		return 0, err
	}
	seen := map[string]struct{}{}
	count := 0
	for _, key := range opts.keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		encrypted, err := envelope.Encrypt([]byte(key))
		if err != nil {
			return 0, fmt.Errorf("encrypt credential: %w", err)
		}
		priority := count + 1
		alias := fmt.Sprintf("ark-seed-%d", priority)
		metadata, err := metadataForAlias(alias)
		if err != nil {
			return 0, err
		}
		after, err := credentialAuditSnapshot("", opts.baseURL, alias, priority, "active", "healthy")
		if err != nil {
			return 0, err
		}
		if current, ok := existing[alias]; ok {
			if _, err := tx.ExecContext(ctx, updateCredentialSQL,
				current.ID,
				opts.baseURL,
				encrypted,
				priority,
				metadata,
			); err != nil {
				return 0, fmt.Errorf("update credential: %w", err)
			}
			after, err = credentialAuditSnapshot(current.ID, opts.baseURL, alias, priority, "active", "healthy")
			if err != nil {
				return 0, err
			}
			if err := insertSeedAudit(ctx, tx, actionUpdateUpstreamCredential, current.ID, current.Snapshot, after); err != nil {
				return 0, err
			}
			delete(existing, alias)
		} else {
			var id string
			if err := tx.QueryRowContext(ctx, insertCredentialSQL,
				defaultProvider,
				opts.baseURL,
				encrypted,
				priority,
				metadata,
			).Scan(&id); err != nil {
				return 0, fmt.Errorf("insert credential: %w", err)
			}
			after, err = credentialAuditSnapshot(id, opts.baseURL, alias, priority, "active", "healthy")
			if err != nil {
				return 0, err
			}
			if err := insertSeedAudit(ctx, tx, actionCreateUpstreamCredential, id, nil, after); err != nil {
				return 0, err
			}
		}
		count++
	}
	for alias, current := range existing {
		if _, err := tx.ExecContext(ctx, disableCredentialSQL, current.ID); err != nil {
			return 0, fmt.Errorf("disable stale seed credential: %w", err)
		}
		after, err := credentialAuditSnapshot(current.ID, current.BaseURL, alias, current.Priority, "disabled", "quarantined")
		if err != nil {
			return 0, err
		}
		if err := insertSeedAudit(ctx, tx, actionDisableUpstreamCredential, current.ID, current.Snapshot, after); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return count, nil
}

type seededCredential struct {
	ID       string
	BaseURL  string
	Priority int
	Snapshot []byte
}

func loadSeededCredentials(ctx context.Context, tx *sql.Tx, provider string) (map[string]seededCredential, error) {
	rows, err := tx.QueryContext(ctx, loadSeededCredentialsSQL, provider)
	if err != nil {
		return nil, fmt.Errorf("query seeded credentials: %w", err)
	}
	defer rows.Close()

	items := map[string]seededCredential{}
	for rows.Next() {
		var alias string
		var item seededCredential
		var snapshot string
		if err := rows.Scan(&alias, &item.ID, &item.BaseURL, &item.Priority, &snapshot); err != nil {
			return nil, fmt.Errorf("scan seeded credential: %w", err)
		}
		item.Snapshot = []byte(snapshot)
		items[alias] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate seeded credentials: %w", err)
	}
	return items, nil
}

func metadataForAlias(alias string) (string, error) {
	raw, err := json.Marshal(map[string]string{"alias": alias})
	if err != nil {
		return "", fmt.Errorf("marshal credential metadata: %w", err)
	}
	return string(raw), nil
}

func credentialAuditSnapshot(id string, baseURL string, alias string, priority int, status string, healthState string) ([]byte, error) {
	raw, err := json.Marshal(map[string]any{
		"id":           id,
		"provider":     defaultProvider,
		"base_url":     baseURL,
		"priority":     priority,
		"weight":       1,
		"status":       status,
		"health_state": healthState,
		"metadata": map[string]string{
			"alias": alias,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal credential audit snapshot: %w", err)
	}
	return raw, nil
}

func insertSeedAudit(ctx context.Context, tx *sql.Tx, action string, resourceID string, before []byte, after []byte) error {
	if _, err := tx.ExecContext(ctx, insertSeedAuditSQL,
		action,
		resourceID,
		nullableJSON(before),
		nullableJSON(after),
	); err != nil {
		return fmt.Errorf("insert credential audit log: %w", err)
	}
	return nil
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

const insertCredentialSQL = `
INSERT INTO upstream_credentials (
  provider,
  base_url,
  encrypted_secret,
  priority,
  weight,
  status,
  health_state,
  metadata
)
VALUES ($1, $2, $3, $4, 1, 'active', 'healthy', $5::jsonb)
RETURNING id::text
`

const updateCredentialSQL = `
UPDATE upstream_credentials
SET base_url = $2,
    encrypted_secret = $3,
    priority = $4,
    weight = 1,
    status = 'active',
    health_state = 'healthy',
    last_error = NULL,
    metadata = $5::jsonb,
    updated_at = now()
WHERE id = $1::uuid`

const disableCredentialSQL = `
UPDATE upstream_credentials
SET status = 'disabled',
    health_state = 'quarantined',
    updated_at = now()
WHERE id = $1::uuid`

const loadSeededCredentialsSQL = `
SELECT
  metadata->>'alias' AS alias,
  id::text,
  base_url,
  priority,
  jsonb_build_object(
    'id', id::text,
    'provider', provider,
    'base_url', base_url,
    'priority', priority,
    'weight', weight,
    'status', status,
    'health_state', health_state,
    'metadata', metadata
  )::text AS snapshot
FROM upstream_credentials
WHERE provider = $1
  AND metadata->>'alias' LIKE 'ark-seed-%'`

const insertSeedAuditSQL = `
INSERT INTO audit_logs (
  actor_id,
  actor_type,
  action,
  resource_type,
  resource_id,
  "before",
  "after",
  user_agent,
  request_id,
  status_code,
  created_at
)
VALUES (
  'credential-seed',
  'system',
  $1,
  'upstream_credential',
  $2,
  $3::jsonb,
  $4::jsonb,
  'omnitoken-seed-upstream',
  'seed-upstream-credential',
  200,
  now()
)`

func sortedEnvKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
