CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE organizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, name)
);

CREATE TABLE api_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id uuid REFERENCES projects(id) ON DELETE CASCADE,
  key_prefix varchar(16) NOT NULL,
  key_hash bytea NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'expired')),
  models_allowlist text[] NOT NULL DEFAULT '{}',
  rpm_limit integer,
  tpm_limit integer,
  monthly_budget_usd numeric(18, 6),
  expires_at timestamptz,
  last_used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (key_prefix)
);

CREATE TABLE upstream_credentials (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  base_url text NOT NULL,
  encrypted_secret bytea NOT NULL,
  region text,
  priority integer NOT NULL DEFAULT 100,
  weight integer NOT NULL DEFAULT 1,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  health_state text NOT NULL DEFAULT 'unknown' CHECK (health_state IN ('unknown', 'healthy', 'degraded', 'down')),
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE model_catalog (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  canonical_model text NOT NULL,
  provider_model text NOT NULL,
  provider text NOT NULL,
  context_window integer,
  max_output_tokens integer,
  modalities text[] NOT NULL DEFAULT '{}',
  supports_streaming boolean NOT NULL DEFAULT true,
  supports_tools boolean NOT NULL DEFAULT false,
  supports_prompt_cache boolean NOT NULL DEFAULT false,
  supports_reasoning boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (canonical_model, provider)
);

CREATE TABLE model_pricing (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id uuid NOT NULL REFERENCES model_catalog(id) ON DELETE CASCADE,
  input_rate_usd numeric(18, 10) NOT NULL DEFAULT 0,
  output_rate_usd numeric(18, 10) NOT NULL DEFAULT 0,
  cached_input_rate_usd numeric(18, 10) NOT NULL DEFAULT 0,
  cache_creation_rate_usd numeric(18, 10) NOT NULL DEFAULT 0,
  reasoning_rate_usd numeric(18, 10) NOT NULL DEFAULT 0,
  currency char(3) NOT NULL DEFAULT 'USD',
  effective_from timestamptz NOT NULL DEFAULT now(),
  source_url text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE usage_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id text NOT NULL UNIQUE,
  trace_id text,
  organization_id uuid REFERENCES organizations(id) ON DELETE SET NULL,
  project_id uuid REFERENCES projects(id) ON DELETE SET NULL,
  api_key_id uuid REFERENCES api_keys(id) ON DELETE SET NULL,
  upstream_credential_id uuid REFERENCES upstream_credentials(id) ON DELETE SET NULL,
  model_requested text NOT NULL,
  model_actual text,
  provider text,
  status_code integer,
  error_code text,
  latency_ms integer,
  ttft_ms integer,
  streaming boolean NOT NULL DEFAULT false,
  cache_status text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE usage_token_breakdown (
  usage_event_id uuid PRIMARY KEY REFERENCES usage_events(id) ON DELETE CASCADE,
  prompt_tokens integer NOT NULL DEFAULT 0,
  completion_tokens integer NOT NULL DEFAULT 0,
  reasoning_tokens integer NOT NULL DEFAULT 0,
  cached_tokens integer NOT NULL DEFAULT 0,
  cache_creation_tokens integer NOT NULL DEFAULT 0,
  cache_read_tokens integer NOT NULL DEFAULT 0,
  image_tokens integer NOT NULL DEFAULT 0,
  audio_input_seconds numeric(18, 6) NOT NULL DEFAULT 0,
  audio_output_seconds numeric(18, 6) NOT NULL DEFAULT 0,
  total_tokens integer NOT NULL DEFAULT 0
);

CREATE TABLE cost_ledger (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  usage_event_id uuid NOT NULL REFERENCES usage_events(id) ON DELETE CASCADE,
  cost_usd numeric(18, 10) NOT NULL,
  list_cost_usd numeric(18, 10),
  discount_amount_usd numeric(18, 10),
  billing_policy_version text,
  settlement_status text NOT NULL DEFAULT 'pending' CHECK (settlement_status IN ('pending', 'settled', 'failed')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor text NOT NULL,
  action text NOT NULL,
  resource_type text NOT NULL,
  resource_id text,
  ip_address inet,
  user_agent text,
  before_state jsonb,
  after_state jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
