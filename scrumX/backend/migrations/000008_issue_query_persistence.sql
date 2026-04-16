CREATE TABLE IF NOT EXISTS issue_saved_queries (
  id         UUID PRIMARY KEY,
  org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  query      TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_issue_saved_queries_owner_updated
  ON issue_saved_queries(org_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS issue_recent_queries (
  org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  query        TEXT NOT NULL,
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (org_id, user_id, query)
);

CREATE INDEX IF NOT EXISTS idx_issue_recent_queries_owner_last_used
  ON issue_recent_queries(org_id, user_id, last_used_at DESC);
