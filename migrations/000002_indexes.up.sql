CREATE INDEX IF NOT EXISTS usage_events_created_at_idx
  ON usage_events (created_at);

CREATE INDEX IF NOT EXISTS usage_events_organization_created_at_idx
  ON usage_events (organization_id, created_at);

CREATE INDEX IF NOT EXISTS usage_events_api_key_created_at_idx
  ON usage_events (api_key_id, created_at);

CREATE INDEX IF NOT EXISTS audit_logs_created_at_idx
  ON audit_logs (created_at);

CREATE INDEX IF NOT EXISTS cost_ledger_usage_event_id_idx
  ON cost_ledger (usage_event_id);
