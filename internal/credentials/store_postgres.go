package credentials

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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

func normalizeJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	out := append([]byte(nil), raw...)
	return json.RawMessage(out)
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
WHERE provider = 'ark'
ORDER BY priority ASC, id ASC`

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
