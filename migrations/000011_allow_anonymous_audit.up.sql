ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_actor_type_check;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_actor_type_check CHECK (actor_type IN ('bootstrap', 'user', 'system', 'anonymous'));
