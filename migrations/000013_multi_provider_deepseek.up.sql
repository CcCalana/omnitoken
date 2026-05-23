ALTER TABLE virtual_models
  ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT 'ark';

ALTER TABLE upstream_credentials
  ALTER COLUMN base_url SET DEFAULT '';

UPDATE upstream_credentials
SET base_url = 'https://ark.cn-beijing.volces.com/api/coding/v3'
WHERE provider = 'ark'
  AND base_url = '';

INSERT INTO model_catalog (
  canonical_model,
  provider_model,
  provider,
  context_window,
  max_output_tokens,
  modalities,
  supports_streaming,
  supports_prompt_cache,
  status
)
VALUES (
  'deepseek-v4-flash',
  'deepseek-v4-flash',
  'deepseek',
  64000,
  8192,
  ARRAY['text'],
  true,
  false,
  'active'
)
ON CONFLICT (canonical_model, provider) DO UPDATE
SET
  provider_model = excluded.provider_model,
  status = excluded.status,
  updated_at = now();

INSERT INTO model_pricing (
  model_id,
  input_rate_usd,
  output_rate_usd,
  reasoning_rate_usd,
  cached_input_rate_usd,
  currency,
  effective_from,
  source_url
)
SELECT
  model_catalog.id,
  0.14,
  0.28,
  0.00,
  0.00,
  'USD',
  '2026-05-23 00:00:00+00'::timestamptz,
  'demo-placeholder:not-real-deepseek-pricing'
FROM model_catalog
WHERE model_catalog.canonical_model = 'deepseek-v4-flash'
  AND model_catalog.provider = 'deepseek'
  AND NOT EXISTS (
    SELECT 1
    FROM model_pricing
    WHERE model_pricing.model_id = model_catalog.id
      AND model_pricing.source_url = 'demo-placeholder:not-real-deepseek-pricing'
  );
