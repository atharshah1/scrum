package events

import (
	"context"
	"log/slog"
	"time"
)

type OutboxDispatcher struct {
	log         *slog.Logger
	store       *OutboxStore
	publisher   KafkaPublisher
	batchSize   int
	interval    time.Duration
	maxAttempts int
	backoff     time.Duration
}

func NewOutboxDispatcher(log *slog.Logger, store *OutboxStore, publisher KafkaPublisher, batchSize int, interval time.Duration, maxAttempts int, backoff time.Duration) *OutboxDispatcher {
	if store == nil || publisher == nil {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if backoff <= 0 {
		backoff = 250 * time.Millisecond
	}
	return &OutboxDispatcher{
		log:         log,
		store:       store,
		publisher:   publisher,
		batchSize:   batchSize,
		interval:    interval,
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
	if d == nil {
		return
	}
	d.flush(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.flush(ctx)
		}
	}
}

func (d *OutboxDispatcher) flush(ctx context.Context) {
	records, err := d.store.ClaimBatch(ctx, d.batchSize)
	if err != nil {
		if d.log != nil {
			d.log.Warn("outbox_claim_failed", "error", err)
		}
		return
	}
	for _, record := range records {
		if err := d.publisher.Publish(ctx, record.Event); err != nil {
			_ = d.store.MarkRetry(ctx, record.ID, record.Attempts, d.maxAttempts, d.backoff, err.Error())
			if d.log != nil {
				d.log.Warn("outbox_publish_retry", "event_id", record.Event.ID, "error", err)
			}
			continue
		}
		_ = d.store.MarkSent(ctx, record.ID)
	}
}
