package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) LookupVirtualKey(ctx context.Context, prefix string) (VirtualKeyRecord, error) {
	const query = `
SELECT id, organization_id, user_id, key_hash, status
FROM api_keys
WHERE key_prefix = $1
  AND user_id IS NOT NULL`

	var record VirtualKeyRecord
	if err := s.db.QueryRowContext(ctx, query, prefix).Scan(
		&record.APIKeyID,
		&record.OrgID,
		&record.UserID,
		&record.KeyHash,
		&record.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VirtualKeyRecord{}, ErrVirtualKeyNotFound
		}
		return VirtualKeyRecord{}, fmt.Errorf("lookup virtual key: %w", err)
	}
	return record, nil
}
