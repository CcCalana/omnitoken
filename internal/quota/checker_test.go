package quota

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
)

func TestCheckerAllowsWhenBudgetHasRoom(t *testing.T) {
	t.Parallel()

	subject := testSubject()
	now := time.Date(2026, 5, 19, 11, 0, 0, 0, time.UTC)
	store := &fakeBudgetStore{status: BudgetStatus{
		BudgetCents:     sql.NullInt64{Int64: 100, Valid: true},
		UsedBudgetCents: 37,
	}}
	checker := NewChecker(store)

	decision, err := checker.Check(context.Background(), subject, now)
	if err != nil {
		t.Fatalf("check budget: %v", err)
	}
	if !decision.Allowed || decision.Reason != ReasonAllowed {
		t.Fatalf("expected allow, got %+v", decision)
	}
	if !store.start.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) ||
		!store.end.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected month window: %s - %s", store.start, store.end)
	}
}

func TestCheckerRejectsExhaustedBudget(t *testing.T) {
	t.Parallel()

	checker := NewChecker(&fakeBudgetStore{status: BudgetStatus{
		BudgetCents:     sql.NullInt64{Int64: 37, Valid: true},
		UsedBudgetCents: 37,
		Exhausted:       true,
	}})

	decision, err := checker.Check(context.Background(), testSubject(), time.Now())
	if err != nil {
		t.Fatalf("check budget: %v", err)
	}
	if decision.Allowed || decision.Reason != ReasonMonthlyBudgetExhausted {
		t.Fatalf("expected monthly_budget_exhausted deny, got %+v", decision)
	}
}

func TestCheckerAllowsUnlimitedBudget(t *testing.T) {
	t.Parallel()

	checker := NewChecker(&fakeBudgetStore{status: BudgetStatus{
		BudgetCents:     sql.NullInt64{},
		UsedBudgetCents: 1000,
		Exhausted:       false,
	}})

	decision, err := checker.Check(context.Background(), testSubject(), time.Now())
	if err != nil {
		t.Fatalf("check budget: %v", err)
	}
	if !decision.Allowed || decision.Reason != ReasonUnlimitedBudget {
		t.Fatalf("expected unlimited allow, got %+v", decision)
	}
}

func TestCheckerFailsClosedOnStoreError(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("database unavailable")
	checker := NewChecker(&fakeBudgetStore{err: storeErr})

	decision, err := checker.Check(context.Background(), testSubject(), time.Now())
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected wrapped store error, got %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected fail-closed decision, got %+v", decision)
	}
}

func TestCheckerRejectsMissingSubject(t *testing.T) {
	t.Parallel()

	decision, err := NewChecker(&fakeBudgetStore{}).Check(context.Background(), auth.Subject{}, time.Now())
	if !errors.Is(err, ErrSubjectMissing) {
		t.Fatalf("expected subject error, got %v", err)
	}
	if decision.Allowed || decision.Reason != ReasonQuotaSubjectMissing {
		t.Fatalf("expected missing-subject deny, got %+v", decision)
	}
}

type fakeBudgetStore struct {
	status BudgetStatus
	err    error
	start  time.Time
	end    time.Time
}

func testSubject() auth.Subject {
	return auth.Subject{
		OrgID:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		UserID:   uuid.MustParse("00000000-0000-0000-0000-000000000201"),
		APIKeyID: uuid.MustParse("00000000-0000-0000-0000-000000000301"),
	}
}

func (s *fakeBudgetStore) LoadMonthlyBudgetStatus(_ context.Context, _ auth.Subject, start time.Time, end time.Time) (BudgetStatus, error) {
	s.start = start
	s.end = end
	if s.err != nil {
		return BudgetStatus{}, s.err
	}
	return s.status, nil
}
