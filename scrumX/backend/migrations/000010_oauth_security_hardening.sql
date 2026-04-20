ALTER TABLE oauth_clients
  ADD COLUMN IF NOT EXISTS client_secret_hash TEXT;

UPDATE oauth_clients
SET client_secret_hash = crypt(encode(digest(client_secret, 'sha256'), 'hex'), gen_salt('bf'))
WHERE COALESCE(client_secret, '') <> ''
  AND COALESCE(client_secret_hash, '') = '';

UPDATE oauth_clients
SET client_secret = NULL
WHERE COALESCE(client_secret_hash, '') <> '';

CREATE TABLE IF NOT EXISTS oauth_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_token_hash TEXT NOT NULL UNIQUE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_id TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
  redirect_uri TEXT NOT NULL,
  state TEXT NOT NULL,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  code_challenge TEXT NOT NULL,
  code_challenge_method TEXT NOT NULL DEFAULT 'S256',
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_sessions_lookup
  ON oauth_sessions(session_token_hash, expires_at)
  WHERE consumed_at IS NULL AND revoked_at IS NULL;
