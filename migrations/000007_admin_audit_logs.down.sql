DROP INDEX IF EXISTS audit_logs_resource_created_at_idx;

DROP INDEX IF EXISTS audit_logs_actor_created_at_idx;

ALTER TABLE audit_logs
  DROP CONSTRAINT IF EXISTS audit_logs_resource_type_not_empty,
  DROP CONSTRAINT IF EXISTS audit_logs_action_not_empty,
  DROP CONSTRAINT IF EXISTS audit_logs_actor_type_check;

ALTER TABLE audit_logs
  RENAME COLUMN "after" TO after_state;

ALTER TABLE audit_logs
  RENAME COLUMN "before" TO before_state;

ALTER TABLE audit_logs
  RENAME COLUMN ip TO ip_address;

ALTER TABLE audit_logs
  DROP COLUMN status_code,
  DROP COLUMN request_id,
  DROP COLUMN actor_type;

ALTER TABLE audit_logs
  RENAME COLUMN actor_id TO actor;
