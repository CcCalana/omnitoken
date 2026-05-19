ALTER TABLE users
  ADD COLUMN monthly_budget_cents bigint
  CHECK (monthly_budget_cents IS NULL OR monthly_budget_cents >= 0);
