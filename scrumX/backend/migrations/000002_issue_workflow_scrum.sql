ALTER TABLE issues
  ADD COLUMN IF NOT EXISTS parent_id UUID REFERENCES issues(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS issue_type TEXT NOT NULL DEFAULT 'task';

CREATE INDEX IF NOT EXISTS idx_issues_org_status ON issues(org_id, status);
CREATE INDEX IF NOT EXISTS idx_issues_org_assignee ON issues(org_id, assignee_id);
CREATE INDEX IF NOT EXISTS idx_issues_org_sprint ON issues(org_id, sprint_id);
CREATE INDEX IF NOT EXISTS idx_issues_org_parent ON issues(org_id, parent_id);
CREATE INDEX IF NOT EXISTS idx_issues_org_type ON issues(org_id, issue_type);

CREATE TABLE IF NOT EXISTS issue_watchers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, issue_id, user_id)
);

CREATE TABLE IF NOT EXISTS issue_activities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  actor_id UUID REFERENCES users(id),
  action TEXT NOT NULL,
  field TEXT,
  from_value TEXT,
  to_value TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workflow_transitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  from_status TEXT NOT NULL,
  to_status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, project_id, from_status, to_status)
);

CREATE TABLE IF NOT EXISTS board_columns (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  board_id UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  statuses JSONB NOT NULL DEFAULT '[]'::jsonb,
  position INTEGER NOT NULL DEFAULT 0,
  UNIQUE(board_id, position)
);
