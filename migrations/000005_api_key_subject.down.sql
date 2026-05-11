DROP INDEX IF EXISTS api_keys_user_id_idx;

ALTER TABLE api_keys
  DROP CONSTRAINT IF EXISTS api_keys_organization_user_fk;

ALTER TABLE api_keys
  DROP COLUMN IF EXISTS user_id;
