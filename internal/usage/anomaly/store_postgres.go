package anomaly

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) ListKeyUsage(ctx context.Context, windowStart time.Time, windowEnd time.Time) ([]KeyUsage, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, listKeyUsageSQL, windowStart.UTC(), windowEnd.UTC())
	if err != nil {
		return nil, fmt.Errorf("query key usage anomalies: %w", err)
	}
	defer rows.Close()

	usages := []KeyUsage{}
	for rows.Next() {
		var item KeyUsage
		var prefix sql.NullString
		if err := rows.Scan(&item.APIKeyID, &prefix, &item.Count); err != nil {
			return nil, fmt.Errorf("scan key usage anomaly: %w", err)
		}
		if prefix.Valid {
			item.APIKeyPrefix = prefix.String
		}
		usages = append(usages, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate key usage anomalies: %w", err)
	}
	return usages, nil
}

const listKeyUsageSQL = `
SELECT
  ue.api_key_id::text,
  ak.key_prefix,
  COUNT(*)::bigint AS call_count
FROM usage_events ue
LEFT JOIN api_keys ak ON ak.id = ue.api_key_id
WHERE ue.api_key_id IS NOT NULL
  AND ue.created_at >= $1
  AND ue.created_at < $2
GROUP BY ue.api_key_id, ak.key_prefix`
