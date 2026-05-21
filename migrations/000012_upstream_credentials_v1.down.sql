DROP INDEX IF EXISTS usage_events_upstream_credential_id_idx;
DROP INDEX IF EXISTS upstream_credentials_provider_priority_idx;

ALTER TABLE usage_events
  DROP COLUMN IF EXISTS model_routed;

ALTER TABLE upstream_credentials
  DROP CONSTRAINT IF EXISTS upstream_credentials_weight_check;

UPDATE upstream_credentials
SET health_state = 'down'
WHERE health_state = 'quarantined';

ALTER TABLE upstream_credentials
  ALTER COLUMN health_state SET DEFAULT 'unknown';

ALTER TABLE upstream_credentials
  DROP CONSTRAINT IF EXISTS upstream_credentials_health_state_check;

ALTER TABLE upstream_credentials
  ADD CONSTRAINT upstream_credentials_health_state_check
  CHECK (health_state IN ('unknown', 'healthy', 'degraded', 'down'));

ALTER TABLE upstream_credentials
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS metadata,
  DROP COLUMN IF EXISTS quota_hint;
