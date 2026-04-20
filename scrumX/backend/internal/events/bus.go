package events

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Handler func(Event)

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type KafkaPublisher interface {
	Publish(ctx context.Context, event Event) error
}

type InternalBus struct {
	mu          sync.RWMutex
	subscribers map[string][]Handler
}

func NewInternalBus() *InternalBus {
	return &InternalBus{subscribers: map[string][]Handler{}}
}

func (b *InternalBus) Subscribe(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

func (b *InternalBus) Publish(_ context.Context, event Event) error {
	b.mu.RLock()
	handlers := append([]Handler{}, b.subscribers[event.Type]...)
	wildcard := append([]Handler{}, b.subscribers["*"]...)
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(event)
	}
	for _, h := range wildcard {
		go h(event)
	}
	return nil
}

type Bus struct {
	internal *InternalBus
	kafka    KafkaPublisher
	outbox   *OutboxStore
	log      *slog.Logger
}

const maxPublishAttempts = 3

func NewBus(log *slog.Logger, internal *InternalBus, kafka KafkaPublisher, outbox *OutboxStore) *Bus {
	return &Bus{internal: internal, kafka: kafka, outbox: outbox, log: log}
}

func (b *Bus) Subscribe(eventType string, handler Handler) {
	b.internal.Subscribe(eventType, handler)
}

func (b *Bus) Publish(ctx context.Context, event Event) error {
	if err := Validate(event); err != nil {
		return err
	}
	if err := b.publishWithRetry(ctx, "internal", event.Type, func(runCtx context.Context) error {
		return b.internal.Publish(runCtx, event)
	}); err != nil {
		return err
	}
	if b.kafka != nil {
		if b.outbox != nil {
			if err := b.publishWithRetry(ctx, "outbox", event.Type, func(runCtx context.Context) error {
				return b.outbox.Enqueue(runCtx, event)
			}); err != nil {
				return err
			}
			return nil
		}
		if err := b.publishWithRetry(ctx, "kafka", event.Type, func(runCtx context.Context) error {
			return b.kafka.Publish(runCtx, event)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bus) publishWithRetry(ctx context.Context, target, eventType string, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= maxPublishAttempts; attempt++ {
		if err := fn(ctx); err != nil {
			lastErr = err
			b.log.Warn("event_publish_attempt_failed", "target", target, "event", eventType, "attempt", attempt, "max_attempts", maxPublishAttempts, "error", err)
			if attempt == maxPublishAttempts {
				break
			}
			delay := time.Duration(attempt) * 100 * time.Millisecond
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		return nil
	}
	if lastErr != nil {
		b.log.Error("event_publish_failed", "target", target, "event", eventType, "error", lastErr)
	}
	return lastErr
}
