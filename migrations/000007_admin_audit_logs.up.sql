ALTER TABLE audit_logs
  RENAME COLUMN actor TO actor_id;

ALTER TABLE audit_logs
  RENAME COLUMN ip_address TO ip;

ALTER TABLE audit_logs
  RENAME COLUMN before_state TO "before";

ALTER TABLE audit_logs
  RENAME COLUMN after_state TO "after";

ALTER TABLE audit_logs
  ADD COLUMN actor_type text NOT NULL DEFAULT 'bootstrap',
  ADD COLUMN request_id text NOT NULL DEFAULT '',
  ADD COLUMN status_code integer NOT NULL DEFAULT 0;

ALTER TABLE audit_logs
  ADD CONSTRAINT audit_logs_actor_type_check CHECK (actor_type IN ('bootstrap', 'user', 'system')),
  ADD CONSTRAINT audit_logs_action_not_empty CHECK (action <> ''),
  ADD CONSTRAINT audit_logs_resource_type_not_empty CHECK (resource_type <> '');

CREATE INDEX IF NOT EXISTS audit_logs_actor_created_at_idx
  ON audit_logs (actor_id, created_at DESC);

CREATE INDEX IF NOT EXISTS audit_logs_resource_created_at_idx
  ON audit_logs (resource_type, resource_id, created_at DESC);
