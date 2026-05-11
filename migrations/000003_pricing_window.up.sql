ALTER TABLE model_pricing
  ADD COLUMN effective_to timestamptz;

ALTER TABLE model_pricing
  ADD CONSTRAINT model_pricing_effective_window_chk
  CHECK (effective_to IS NULL OR effective_to > effective_from);

CREATE INDEX IF NOT EXISTS model_pricing_model_effective_idx
  ON model_pricing (model_id, effective_from DESC, created_at DESC);

CREATE VIEW model_pricing_current AS
SELECT
  id,
  model_id,
  input_rate_usd,
  output_rate_usd,
  cached_input_rate_usd,
  cache_creation_rate_usd,
  reasoning_rate_usd,
  currency,
  effective_from,
  effective_to,
  source_url,
  created_at
FROM (
  SELECT
    model_pricing.*,
    row_number() OVER (
      PARTITION BY model_id
      ORDER BY effective_from DESC, created_at DESC, id DESC
    ) AS row_number
  FROM model_pricing
  WHERE effective_from <= now()
    AND (effective_to IS NULL OR effective_to > now())
) ranked
WHERE row_number = 1;
