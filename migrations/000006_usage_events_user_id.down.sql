DROP INDEX IF EXISTS usage_events_user_created_at_idx;

ALTER TABLE usage_events
  DROP COLUMN IF EXISTS user_id;
