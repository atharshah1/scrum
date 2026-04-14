-- Durable idempotency and attempt metadata for distributed automation workers.
CREATE TABLE IF NOT EXISTS automation_action_executions (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id             UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  rule_id            UUID REFERENCES automation_rules(id) ON DELETE SET NULL,
  event_id           UUID NOT NULL,
  action_fingerprint TEXT NOT NULL,
  status             TEXT NOT NULL DEFAULT 'processing',
  attempts           INTEGER NOT NULL DEFAULT 0,
  last_error         TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, rule_id, event_id, action_fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_automation_action_executions_status
  ON automation_action_executions(org_id, status, updated_at DESC);

