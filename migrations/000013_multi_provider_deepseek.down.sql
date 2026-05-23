DELETE FROM model_pricing
USING model_catalog
WHERE model_pricing.model_id = model_catalog.id
  AND model_catalog.provider = 'deepseek'
  AND model_catalog.canonical_model = 'deepseek-v4-flash'
  AND model_pricing.source_url = 'demo-placeholder:not-real-deepseek-pricing';

DELETE FROM model_catalog
WHERE provider = 'deepseek'
  AND canonical_model = 'deepseek-v4-flash';

ALTER TABLE upstream_credentials
  ALTER COLUMN base_url DROP DEFAULT;

ALTER TABLE virtual_models
  DROP COLUMN IF EXISTS provider;
