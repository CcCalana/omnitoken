package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
)

type Checker struct {
	store Store
}

func NewChecker(store Store) *Checker {
	if store == nil {
		return nil
	}
	return &Checker{store: store}
}

func (c *Checker) Check(ctx context.Context, subject auth.Subject, now time.Time) (Decision, error) {
	if subject.OrgID == uuid.Nil || subject.UserID == uuid.Nil {
		return Decision{Allowed: false, Reason: ReasonQuotaSubjectMissing}, ErrSubjectMissing
	}
	if c == nil || c.store == nil {
		return Decision{Allowed: false, Reason: ReasonQuotaStoreNotConfigured}, ErrStoreNotConfigured
	}

	start, end := MonthWindow(now)
	status, err := c.store.LoadMonthlyBudgetStatus(ctx, subject, start, end)
	if err != nil {
		return Decision{}, fmt.Errorf("load monthly budget status: %w", err)
	}

	decision := Decision{
		Allowed:         true,
		Reason:          ReasonAllowed,
		BudgetCents:     status.BudgetCents,
		UsedBudgetCents: status.UsedBudgetCents,
	}
	if !status.BudgetCents.Valid {
		decision.Reason = ReasonUnlimitedBudget
		return decision, nil
	}
	if status.Exhausted {
		decision.Allowed = false
		decision.Reason = ReasonMonthlyBudgetExhausted
	}
	return decision, nil
}

func MonthWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}
