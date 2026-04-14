ALTER TABLE automation_dead_letters
  ADD COLUMN IF NOT EXISTS replay_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_replayed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_replayed_by UUID,
  ADD COLUMN IF NOT EXISTS last_replay_error TEXT,
  ADD COLUMN IF NOT EXISTS resolved BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_automation_dead_letters_replay
  ON automation_dead_letters(org_id, resolved, created_at DESC);
