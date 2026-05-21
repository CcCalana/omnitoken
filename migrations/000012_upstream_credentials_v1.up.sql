ALTER TABLE upstream_credentials
  ADD COLUMN IF NOT EXISTS quota_hint jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

UPDATE upstream_credentials
SET health_state = 'healthy'
WHERE health_state IN ('unknown', 'down') OR health_state IS NULL;

ALTER TABLE upstream_credentials
  ALTER COLUMN health_state SET DEFAULT 'healthy';

ALTER TABLE upstream_credentials
  DROP CONSTRAINT IF EXISTS upstream_credentials_health_state_check;

ALTER TABLE upstream_credentials
  ADD CONSTRAINT upstream_credentials_health_state_check
  CHECK (health_state IN ('healthy', 'degraded', 'quarantined'));

ALTER TABLE upstream_credentials
  DROP CONSTRAINT IF EXISTS upstream_credentials_weight_check;

ALTER TABLE upstream_credentials
  ADD CONSTRAINT upstream_credentials_weight_check CHECK (weight > 0);

CREATE INDEX IF NOT EXISTS upstream_credentials_provider_priority_idx
  ON upstream_credentials (provider, priority, id);

CREATE INDEX IF NOT EXISTS usage_events_upstream_credential_id_idx
  ON usage_events (upstream_credential_id);

ALTER TABLE usage_events
  ADD COLUMN IF NOT EXISTS model_routed text NOT NULL DEFAULT '';
