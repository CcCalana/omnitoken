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
