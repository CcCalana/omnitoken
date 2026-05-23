package credentials

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	omnicrypto "github.com/omnitoken/omnitoken/internal/crypto"
)

type PostgresStore struct {
	db       *sql.DB
	envelope *omnicrypto.Envelope
}

func NewPostgresStore(db *sql.DB, envelope *omnicrypto.Envelope) *PostgresStore {
	return &PostgresStore{db: db, envelope: envelope}
}

func (s *PostgresStore) Load(ctx context.Context) ([]Credential, error) {
	if s == nil || s.db == nil || s.envelope == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, loadCredentialsSQL)
	if err != nil {
		return nil, fmt.Errorf("query upstream credentials: %w", err)
	}
	defer rows.Close()

	items := []Credential{}
	for rows.Next() {
		var item Credential
		var encrypted []byte
		var quotaHint, metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.Provider,
			&item.BaseURL,
			&encrypted,
			&item.Region,
			&item.Priority,
			&item.Weight,
			&item.Status,
			&item.HealthState,
			&item.LastError,
			&quotaHint,
			&metadata,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan upstream credential: %w", err)
		}
		secret, err := s.envelope.Decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt upstream credential %s: %w", item.ID, err)
		}
		item.Secret = string(secret)
		item.QuotaHint = normalizeJSON(quotaHint)
		item.Metadata = normalizeJSON(metadata)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upstream credentials: %w", err)
	}
	return items, nil
}

func (s *PostgresStore) ListPublic(ctx context.Context) ([]PublicCredential, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, listCredentialsSQL)
	if err != nil {
		return nil, fmt.Errorf("query public upstream credentials: %w", err)
	}
	defer rows.Close()

	items := []PublicCredential{}
	for rows.Next() {
		var item PublicCredential
		var quotaHint, metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.Provider,
			&item.BaseURL,
			&item.Region,
			&item.Priority,
			&item.Weight,
			&item.Status,
			&item.HealthState,
			&item.LastError,
			&quotaHint,
			&metadata,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan public upstream credential: %w", err)
		}
		item.QuotaHint = normalizeJSON(quotaHint)
		item.Metadata = normalizeJSON(metadata)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public upstream credentials: %w", err)
	}
	return items, nil
}

func (s *PostgresStore) Create(ctx context.Context, params CreateParams) (PublicCredential, error) {
	if s == nil || s.db == nil {
		return PublicCredential{}, ErrCredentialMissing
	}
	if s.envelope == nil {
		return PublicCredential{}, omnicrypto.ErrMasterKeyMissing
	}
	encrypted, err := s.envelope.Encrypt([]byte(params.Secret))
	if err != nil {
		return PublicCredential{}, fmt.Errorf("encrypt upstream credential: %w", err)
	}
	metadata, err := json.Marshal(map[string]string{"alias": strings.TrimSpace(params.Alias)})
	if err != nil {
		return PublicCredential{}, fmt.Errorf("marshal credential metadata: %w", err)
	}
	id := uuid.New()
	row := s.db.QueryRowContext(ctx, createCredentialSQL,
		id,
		strings.TrimSpace(params.Provider),
		strings.TrimSpace(params.BaseURL),
		encrypted,
		params.Priority,
		metadata,
	)
	item, err := scanPublicCredential(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PublicCredential{}, ErrAliasExists
		}
		return PublicCredential{}, fmt.Errorf("insert upstream credential: %w", err)
	}
	return item, nil
}

func (s *PostgresStore) Disable(ctx context.Context, id string) (PublicCredential, error) {
	if s == nil || s.db == nil {
		return PublicCredential{}, ErrCredentialMissing
	}
	row := s.db.QueryRowContext(ctx, disableCredentialSQL, strings.TrimSpace(id))
	item, err := scanPublicCredential(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PublicCredential{}, ErrCredentialMissing
		}
		return PublicCredential{}, fmt.Errorf("disable upstream credential: %w", err)
	}
	return item, nil
}

func normalizeJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	out := append([]byte(nil), raw...)
	return json.RawMessage(out)
}

type publicCredentialScanner interface {
	Scan(dest ...any) error
}

func scanPublicCredential(scanner publicCredentialScanner) (PublicCredential, error) {
	var item PublicCredential
	var quotaHint, metadata []byte
	if err := scanner.Scan(
		&item.ID,
		&item.Provider,
		&item.BaseURL,
		&item.Region,
		&item.Priority,
		&item.Weight,
		&item.Status,
		&item.HealthState,
		&item.LastError,
		&quotaHint,
		&metadata,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return PublicCredential{}, err
	}
	item.QuotaHint = normalizeJSON(quotaHint)
	item.Metadata = normalizeJSON(metadata)
	return item, nil
}

const loadCredentialsSQL = `
SELECT
  id::text,
  provider,
  base_url,
  encrypted_secret,
  COALESCE(region, '') AS region,
  priority,
  weight,
  status,
  health_state,
  COALESCE(last_error, '') AS last_error,
  COALESCE(quota_hint, '{}'::jsonb) AS quota_hint,
  COALESCE(metadata, '{}'::jsonb) AS metadata,
  created_at,
  updated_at
FROM upstream_credentials
WHERE status = 'active'
ORDER BY provider ASC, priority ASC, id ASC`

const createCredentialSQL = `
WITH inserted AS (
  INSERT INTO upstream_credentials (
    id,
    provider,
    base_url,
    encrypted_secret,
    priority,
    weight,
    status,
    health_state,
    metadata,
    updated_at
  )
  SELECT $1, $2, $3, $4, $5, 1, 'active', 'healthy', $6::jsonb, now()
  WHERE NOT EXISTS (
    SELECT 1
    FROM upstream_credentials
    WHERE provider = $2
      AND metadata->>'alias' = ($6::jsonb)->>'alias'
  )
  RETURNING
    id::text,
    provider,
    base_url,
    COALESCE(region, '') AS region,
    priority,
    weight,
    status,
    health_state,
    COALESCE(last_error, '') AS last_error,
    COALESCE(quota_hint, '{}'::jsonb) AS quota_hint,
    COALESCE(metadata, '{}'::jsonb) AS metadata,
    created_at,
    updated_at
)
SELECT * FROM inserted`

const disableCredentialSQL = `
UPDATE upstream_credentials
SET status = 'disabled',
    updated_at = now()
WHERE id = $1
RETURNING
  id::text,
  provider,
  base_url,
  COALESCE(region, '') AS region,
  priority,
  weight,
  status,
  health_state,
  COALESCE(last_error, '') AS last_error,
  COALESCE(quota_hint, '{}'::jsonb) AS quota_hint,
  COALESCE(metadata, '{}'::jsonb) AS metadata,
  created_at,
  updated_at`

const listCredentialsSQL = `
SELECT
  id::text,
  provider,
  base_url,
  COALESCE(region, '') AS region,
  priority,
  weight,
  status,
  health_state,
  COALESCE(last_error, '') AS last_error,
  COALESCE(quota_hint, '{}'::jsonb) AS quota_hint,
  COALESCE(metadata, '{}'::jsonb) AS metadata,
  created_at,
  updated_at
FROM upstream_credentials
ORDER BY provider ASC, priority ASC, id ASC`
