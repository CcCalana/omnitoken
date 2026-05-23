package router

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrVirtualModelDisabled = errors.New("virtual model is disabled")

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Resolve(ctx context.Context, requested string) (Resolution, error) {
	if s == nil || s.db == nil {
		return Resolution{RealModel: requested, IsVirtual: false}, nil
	}

	const query = `SELECT real_model, COALESCE(provider, 'ark'), status FROM virtual_models WHERE name = $1`
	var realModel, provider, status string
	err := s.db.QueryRowContext(ctx, query, requested).Scan(&realModel, &provider, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Resolution{RealModel: requested, IsVirtual: false}, nil
		}
		return Resolution{}, fmt.Errorf("resolve virtual model: %w", err)
	}

	if status != "active" {
		return Resolution{}, ErrVirtualModelDisabled
	}

	return Resolution{RealModel: realModel, Provider: provider, IsVirtual: true}, nil
}
