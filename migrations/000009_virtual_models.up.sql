CREATE TABLE IF NOT EXISTS virtual_models (
    name text PRIMARY KEY,
    real_model text NOT NULL,
    status text DEFAULT 'active',
    description text,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now(),
    CHECK (name <> ''),
    CHECK (real_model <> '')
);
