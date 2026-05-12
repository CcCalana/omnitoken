ALTER TABLE usage_events
  ADD COLUMN user_id uuid REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS usage_events_user_created_at_idx
  ON usage_events (user_id, created_at);
