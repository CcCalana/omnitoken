package quota

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
)

const (
	ReasonAllowed                 = "allowed"
	ReasonUnlimitedBudget         = "unlimited_budget"
	ReasonMonthlyBudgetExhausted  = "monthly_budget_exhausted"
	ReasonQuotaSubjectMissing     = "quota_subject_missing"
	ReasonQuotaStoreNotConfigured = "quota_store_not_configured"
)

var (
	ErrSubjectMissing     = errors.New("quota subject missing")
	ErrStoreNotConfigured = errors.New("quota store not configured")
)

type BudgetStatus struct {
	BudgetCents     sql.NullInt64
	UsedBudgetCents int64
	Exhausted       bool
}

type Decision struct {
	Allowed         bool
	Reason          string
	BudgetCents     sql.NullInt64
	UsedBudgetCents int64
}

type Store interface {
	LoadMonthlyBudgetStatus(ctx context.Context, subject auth.Subject, start time.Time, end time.Time) (BudgetStatus, error)
}

type BudgetChecker interface {
	Check(ctx context.Context, subject auth.Subject, now time.Time) (Decision, error)
}
