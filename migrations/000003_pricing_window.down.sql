DROP VIEW IF EXISTS model_pricing_current;
DROP INDEX IF EXISTS model_pricing_model_effective_idx;

ALTER TABLE model_pricing
  DROP CONSTRAINT IF EXISTS model_pricing_effective_window_chk;

ALTER TABLE model_pricing
  DROP COLUMN IF EXISTS effective_to;
