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

-- Demo-Ready placeholder pricing only.
-- These Ark rates are NOT real provider pricing and MUST NOT be used for commercial quotes.
-- Rates are USD per 1M tokens.
INSERT INTO model_pricing (
  model_id,
  input_rate_usd,
  output_rate_usd,
  reasoning_rate_usd,
  cached_input_rate_usd,
  currency,
  effective_from,
  source_url
)
SELECT
  model_catalog.id,
  0.50,
  1.50,
  0.00,
  0.00,
  'USD',
  '2026-05-11 00:00:00+00'::timestamptz,
  'demo-placeholder:not-real-ark-pricing'
FROM model_catalog
WHERE model_catalog.canonical_model = 'ark-code-latest'
  AND model_catalog.provider = 'ark'
  AND NOT EXISTS (
    SELECT 1
    FROM model_pricing
    WHERE model_pricing.model_id = model_catalog.id
      AND model_pricing.source_url = 'demo-placeholder:not-real-ark-pricing'
  );

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
  WHEN users.email = 'user01@democorp.local' THEN 'viewer'
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

INSERT INTO virtual_models (name, real_model, status, description)
VALUES
  ('chat-fast', 'kimi-k2.6', 'active', 'Fast routing mapped to kimi-k2.6'),
  ('chat-balanced', 'glm-5.1', 'active', 'Balanced routing mapped to glm-5.1'),
  ('chat-quality', 'deepseek-v3.2', 'active', 'Quality routing mapped to deepseek-v3.2'),
  ('chat-code', 'doubao-seed-code', 'active', 'Coding specialized routing mapped to doubao-seed-code'),
  ('chat-experimental', 'minimax-m2.7', 'active', 'Experimental routing mapped to minimax-m2.7')
ON CONFLICT (name) DO UPDATE
SET
  real_model = excluded.real_model,
  status = excluded.status,
  description = excluded.description,
  updated_at = now();

-- Set passwords for local admin/viewer login to 'password' (bcrypt cost 10).
UPDATE users
SET password_hash = '$2a$10$Q1GeBLzIJ9SfDlkO2nyHIOfwSWlU6wNvpKU4YRfDV1i.UE3CxSJu2',
    updated_at = now()
WHERE email IN ('admin@democorp.local', 'user01@democorp.local');
