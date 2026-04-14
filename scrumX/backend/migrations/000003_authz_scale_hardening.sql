ALTER TABLE issues
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_issues_active_project_updated
  ON issues(org_id, project_id, updated_at DESC, id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_issues_active_project_sprint_status_updated
  ON issues(org_id, project_id, sprint_id, status, updated_at DESC, id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_issue_labels_org_issue_label
  ON issue_labels(org_id, issue_id, label);

CREATE INDEX IF NOT EXISTS idx_issue_labels_org_label_issue
  ON issue_labels(org_id, label, issue_id);

CREATE TABLE IF NOT EXISTS project_memberships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('Admin','Member','Viewer')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_memberships_lookup
  ON project_memberships(org_id, project_id, user_id);

CREATE TABLE IF NOT EXISTS auth_refresh_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_id TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, user_id, token_id)
);

CREATE INDEX IF NOT EXISTS idx_auth_refresh_tokens_active
  ON auth_refresh_tokens(org_id, user_id, token_id, expires_at)
  WHERE revoked_at IS NULL;

ALTER TABLE workflow_transitions
  ADD COLUMN IF NOT EXISTS conditions JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS validators JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS post_functions JSONB NOT NULL DEFAULT '{}'::jsonb;
