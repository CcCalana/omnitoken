package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	if db == nil {
		return nil
	}
	return &PostgresStore{db: db}
}

func (s *PostgresStore) LoadMonthlyBudgetStatus(ctx context.Context, subject auth.Subject, start time.Time, end time.Time) (BudgetStatus, error) {
	if s == nil || s.db == nil {
		return BudgetStatus{}, ErrStoreNotConfigured
	}

	var status BudgetStatus
	if err := s.db.QueryRowContext(ctx, monthlyBudgetStatusSQL,
		subject.OrgID,
		subject.UserID,
		start.UTC(),
		end.UTC(),
	).Scan(
		&status.BudgetCents,
		&status.UsedBudgetCents,
		&status.Exhausted,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BudgetStatus{}, fmt.Errorf("monthly budget subject not found: %w", err)
		}
		return BudgetStatus{}, fmt.Errorf("query monthly budget status: %w", err)
	}
	return status, nil
}

const monthlyBudgetStatusSQL = `
SELECT
  u.monthly_budget_cents,
  -- Display value only: round sub-cent cost upward so UI never understates use.
  COALESCE(CEIL(COALESCE(SUM(cl.cost_usd), 0) * 100), 0)::bigint AS used_budget_cents,
  CASE
    WHEN u.monthly_budget_cents IS NULL THEN false
    -- Enforcement uses exact numeric USD comparison; do not compare CEIL cents.
    ELSE COALESCE(SUM(cl.cost_usd), 0) >= (u.monthly_budget_cents::numeric / 100)
  END AS exhausted
FROM users u
LEFT JOIN usage_events ue
  ON ue.organization_id = u.organization_id
 AND ue.user_id = u.id
 AND ue.created_at >= $3
 AND ue.created_at < $4
LEFT JOIN cost_ledger cl ON cl.usage_event_id = ue.id
WHERE u.organization_id = $1
  AND u.id = $2
GROUP BY u.monthly_budget_cents`
