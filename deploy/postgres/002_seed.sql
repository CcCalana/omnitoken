INSERT INTO organizations (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'Demo Organization')
ON CONFLICT (id) DO NOTHING;

INSERT INTO projects (id, organization_id, name)
VALUES (
  '00000000-0000-0000-0000-000000000101',
  '00000000-0000-0000-0000-000000000001',
  'Default Project'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO model_catalog (
  canonical_model,
  provider_model,
  provider,
  context_window,
  max_output_tokens,
  modalities,
  supports_streaming,
  supports_prompt_cache,
  status
)
VALUES
  ('gpt-4o', 'gpt-4o', 'openai', 128000, 16384, ARRAY['text', 'image'], true, true, 'active'),
  ('gpt-4o-mini', 'gpt-4o-mini', 'openai', 128000, 16384, ARRAY['text', 'image'], true, true, 'active'),
  ('claude-3-5-sonnet', 'claude-3-5-sonnet', 'anthropic', 200000, 8192, ARRAY['text', 'image'], true, true, 'active'),
  ('ark-code-latest', 'glm-5.1', 'ark', 200000, 8192, ARRAY['text'], true, false, 'active')
ON CONFLICT (canonical_model, provider) DO NOTHING;

UPDATE model_catalog
SET supports_reasoning = true
WHERE canonical_model = 'ark-code-latest';

INSERT INTO users (id, organization_id, email, display_name)
VALUES
  ('00000000-0000-0000-0000-000000000201', '00000000-0000-0000-0000-000000000001', 'admin@democorp.local', 'Demo Admin'),
  ('00000000-0000-0000-0000-000000000202', '00000000-0000-0000-0000-000000000001', 'user01@democorp.local', 'Demo User 01'),
  ('00000000-0000-0000-0000-000000000203', '00000000-0000-0000-0000-000000000001', 'user02@democorp.local', 'Demo User 02'),
  ('00000000-0000-0000-0000-000000000204', '00000000-0000-0000-0000-000000000001', 'user03@democorp.local', 'Demo User 03'),
  ('00000000-0000-0000-0000-000000000205', '00000000-0000-0000-0000-000000000001', 'user04@democorp.local', 'Demo User 04'),
  ('00000000-0000-0000-0000-000000000206', '00000000-0000-0000-0000-000000000001', 'user05@democorp.local', 'Demo User 05'),
  ('00000000-0000-0000-0000-000000000207', '00000000-0000-0000-0000-000000000001', 'user06@democorp.local', 'Demo User 06'),
  ('00000000-0000-0000-0000-000000000208', '00000000-0000-0000-0000-000000000001', 'user07@democorp.local', 'Demo User 07'),
  ('00000000-0000-0000-0000-000000000209', '00000000-0000-0000-0000-000000000001', 'user08@democorp.local', 'Demo User 08'),
  ('00000000-0000-0000-0000-000000000210', '00000000-0000-0000-0000-000000000001', 'user09@democorp.local', 'Demo User 09'),
  ('00000000-0000-0000-0000-000000000211', '00000000-0000-0000-0000-000000000001', 'user10@democorp.local', 'Demo User 10')
ON CONFLICT (organization_id, email) DO UPDATE
SET
  display_name = excluded.display_name,
  updated_at = now();

INSERT INTO role_assignments (organization_id, user_id, role_id)
SELECT
  '00000000-0000-0000-0000-000000000001',
  users.id,
  roles.id
FROM users
JOIN roles ON roles.canonical_name = CASE
  WHEN users.email = 'admin@democorp.local' THEN 'admin'
  ELSE 'member'
END
WHERE users.organization_id = '00000000-0000-0000-0000-000000000001'
  AND users.email IN (
    'admin@democorp.local',
    'user01@democorp.local',
    'user02@democorp.local',
    'user03@democorp.local',
    'user04@democorp.local',
    'user05@democorp.local',
    'user06@democorp.local',
    'user07@democorp.local',
    'user08@democorp.local',
    'user09@democorp.local',
    'user10@democorp.local'
  )
ON CONFLICT (organization_id, user_id, role_id) DO NOTHING;
