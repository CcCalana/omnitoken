package audit

import (
	"context"
	"database/sql"
	"fmt"
	"net"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) InsertAudit(ctx context.Context, record Record) error {
	if s == nil || s.db == nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, insertAuditSQL,
		record.ActorID,
		record.ActorType,
		record.Action,
		record.ResourceType,
		nullableText(record.ResourceID),
		nullableJSON(record.Before),
		nullableJSON(record.After),
		nullableIP(record.IP),
		record.UserAgent,
		record.RequestID,
		record.StatusCode,
		record.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func nullableIP(value net.IP) any {
	if len(value) == 0 {
		return nil
	}
	return value.String()
}

const insertAuditSQL = `
INSERT INTO audit_logs (
  actor_id,
  actor_type,
  action,
  resource_type,
  resource_id,
  "before",
  "after",
  ip,
  user_agent,
  request_id,
  status_code,
  created_at
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9, $10, $11, $12)`
