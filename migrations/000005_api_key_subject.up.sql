ALTER TABLE api_keys
  ADD COLUMN user_id uuid;

ALTER TABLE api_keys
  ADD CONSTRAINT api_keys_organization_user_fk
  FOREIGN KEY (organization_id, user_id)
  REFERENCES users(organization_id, id)
  ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS api_keys_user_id_idx
  ON api_keys (user_id);
