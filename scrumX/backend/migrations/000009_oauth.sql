CREATE TABLE IF NOT EXISTS oauth_scopes (
  name TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_clients (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id TEXT NOT NULL UNIQUE,
  client_secret TEXT,
  name TEXT NOT NULL,
  redirect_uris JSONB NOT NULL DEFAULT '[]'::jsonb,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  owner_org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  is_confidential BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code_hash TEXT NOT NULL UNIQUE,
  client_id TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  redirect_uri TEXT NOT NULL,
  code_challenge TEXT NOT NULL,
  code_challenge_method TEXT NOT NULL DEFAULT 'S256',
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash TEXT NOT NULL UNIQUE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_id TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oauth_authorizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_id TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, client_id, org_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_clients_client_id
  ON oauth_clients(client_id);

CREATE INDEX IF NOT EXISTS idx_oauth_codes_lookup
  ON oauth_codes(client_id, expires_at)
  WHERE consumed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_oauth_refresh_tokens_active
  ON oauth_refresh_tokens(client_id, user_id, org_id, expires_at)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_oauth_authorizations_lookup
  ON oauth_authorizations(user_id, client_id, org_id);

INSERT INTO oauth_scopes (name, description)
VALUES
  ('read:issues', 'Read issues and issue activity'),
  ('write:issues', 'Create and update issues, comments, labels, and links'),
  ('read:projects', 'Read projects and boards'),
  ('write:projects', 'Create and update projects and related resources'),
  ('read:releases', 'Read releases and deployments'),
  ('write:releases', 'Create releases and deployments'),
  ('admin:org', 'Manage organization-level settings and access'),
  ('automation:execute', 'Manage automation rules and dead-letter replay')
ON CONFLICT (name) DO NOTHING;

INSERT INTO oauth_clients (client_id, client_secret, name, redirect_uris, scopes, is_confidential)
VALUES (
  'scrumx-cli',
  NULL,
  'ScrumX CLI',
  '["http://127.0.0.1:8787/callback","http://localhost:8787/callback"]'::jsonb,
  '["read:issues","write:issues","read:projects","write:projects","read:releases","write:releases","admin:org","automation:execute"]'::jsonb,
  FALSE
)
ON CONFLICT (client_id) DO NOTHING;
