-- 1. Prevent duplicate labels on the same issue
ALTER TABLE issue_labels
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS uq_issue_labels_org_issue_label
  ON issue_labels(org_id, issue_id, label);

-- 2. Full-text search on issues (title + description)
ALTER TABLE issues
  ADD COLUMN IF NOT EXISTS search_vector tsvector;

UPDATE issues
  SET search_vector = to_tsvector('english', coalesce(title,'') || ' ' || coalesce(description,''))
  WHERE search_vector IS NULL;

CREATE INDEX IF NOT EXISTS idx_issues_search_vector
  ON issues USING GIN (search_vector)
  WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION issues_search_vector_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector :=
    to_tsvector('english', coalesce(NEW.title,'') || ' ' || coalesce(NEW.description,''));
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger
    WHERE tgname = 'trg_issues_search_vector'
      AND tgrelid = 'issues'::regclass
  ) THEN
    EXECUTE 'CREATE TRIGGER trg_issues_search_vector
      BEFORE INSERT OR UPDATE OF title, description ON issues
      FOR EACH ROW EXECUTE FUNCTION issues_search_vector_update()';
  END IF;
END;
$$;

-- 3. Automation dead-letter queue for failed actions after all retries
CREATE TABLE IF NOT EXISTS automation_dead_letters (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  rule_id       UUID REFERENCES automation_rules(id) ON DELETE SET NULL,
  event_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  action_type   TEXT NOT NULL,
  action_params JSONB NOT NULL DEFAULT '{}'::jsonb,
  error_message TEXT NOT NULL,
  attempts      INTEGER NOT NULL DEFAULT 1,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_automation_dead_letters_org
  ON automation_dead_letters(org_id, created_at DESC);

-- 4. In-app user notifications
CREATE TABLE IF NOT EXISTS user_notifications (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type       TEXT NOT NULL,
  title      TEXT NOT NULL,
  message    TEXT NOT NULL DEFAULT '',
  entity_id  UUID,
  is_read    BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_notifications_user_unread
  ON user_notifications(org_id, user_id, created_at DESC)
  WHERE is_read = FALSE;
