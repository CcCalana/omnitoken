-- Q-1: Phase 1 users are tenant-scoped and deterministic; email is NOT NULL with UNIQUE (organization_id, email), and demo seed uses userNN@democorp.local.
-- Q-2: users.updated_at is stored now; application code will set it explicitly instead of relying on a database trigger.
CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  email text NOT NULL,
  display_name text NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, id),
  UNIQUE (organization_id, email)
);

CREATE TABLE roles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  canonical_name text NOT NULL UNIQUE CHECK (canonical_name IN ('admin', 'member', 'viewer')),
  description text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_assignments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL,
  user_id uuid NOT NULL,
  role_id uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, user_id, role_id),
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  FOREIGN KEY (organization_id, user_id) REFERENCES users(organization_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS role_assignments_user_id_idx
  ON role_assignments (user_id);

CREATE INDEX IF NOT EXISTS role_assignments_organization_role_idx
  ON role_assignments (organization_id, role_id);

INSERT INTO roles (canonical_name, description)
VALUES
  ('admin', 'Full organization administrator'),
  ('member', 'Gateway user with self-service usage visibility'),
  ('viewer', 'Read-only admin console viewer')
ON CONFLICT (canonical_name) DO UPDATE
SET description = excluded.description;
