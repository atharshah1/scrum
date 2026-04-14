package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type OutboxStore struct {
	db *sql.DB
}

type OutboxRecord struct {
	ID       uuid.UUID
	Event    Event
	Attempts int
}

func NewOutboxStore(db *sql.DB) *OutboxStore {
	if db == nil {
		return nil
	}
	return &OutboxStore{db: db}
}

func (s *OutboxStore) Enqueue(ctx context.Context, event Event) error {
	if s == nil || s.db == nil {
		return errors.New("outbox store not configured")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO event_outbox (id, event_type, payload, status, attempts, next_attempt_at, created_at, updated_at)
VALUES ($1,$2,$3,'pending',0,NOW(),NOW(),NOW())`, uuid.New(), event.Type, payload)
	return err
}

func (s *OutboxStore) ClaimBatch(ctx context.Context, size int) ([]OutboxRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("outbox store not configured")
	}
	if size <= 0 {
		size = 50
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `WITH picked AS (
	SELECT id
	FROM event_outbox
	WHERE status IN ('pending','retry') AND next_attempt_at <= NOW()
	ORDER BY created_at ASC
	FOR UPDATE SKIP LOCKED
	LIMIT $1
)
UPDATE event_outbox e
SET status='processing', updated_at=NOW()
FROM picked
WHERE e.id=picked.id
RETURNING e.id, e.payload, e.attempts`, size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OutboxRecord, 0, size)
	badPayloadIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var record OutboxRecord
		var payload []byte
		if err := rows.Scan(&record.ID, &payload, &record.Attempts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &record.Event); err != nil {
			badPayloadIDs = append(badPayloadIDs, record.ID)
			continue
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for _, id := range badPayloadIDs {
		_ = s.MarkDead(ctx, id, "invalid outbox payload")
	}
	return out, nil
}

func (s *OutboxStore) MarkSent(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.db == nil {
		return errors.New("outbox store not configured")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM event_outbox WHERE id=$1`, id)
	return err
}

func (s *OutboxStore) MarkDead(ctx context.Context, id uuid.UUID, lastErr string) error {
	if s == nil || s.db == nil {
		return errors.New("outbox store not configured")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE event_outbox SET status='dead', last_error=$2, updated_at=NOW() WHERE id=$1`, id, lastErr)
	return err
}

func (s *OutboxStore) MarkRetry(ctx context.Context, id uuid.UUID, attempts, maxAttempts int, baseBackoff time.Duration, lastErr string) error {
	if s == nil || s.db == nil {
		return errors.New("outbox store not configured")
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	nextAttempt := attempts + 1
	if nextAttempt >= maxAttempts {
		return s.MarkDead(ctx, id, lastErr)
	}
	if baseBackoff <= 0 {
		baseBackoff = 250 * time.Millisecond
	}
	wait := time.Duration(1<<attempts) * baseBackoff
	nextTime := time.Now().Add(wait)
	_, err := s.db.ExecContext(ctx, `UPDATE event_outbox
SET status='retry', attempts=$2, last_error=$3, next_attempt_at=$4, updated_at=NOW()
WHERE id=$1`, id, nextAttempt, lastErr, nextTime)
	return err
}
