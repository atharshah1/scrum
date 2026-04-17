package events

import (
	"context"
	"log/slog"
	"sync"
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
	if err := b.internal.Publish(ctx, event); err != nil {
		return err
	}
	if b.kafka != nil {
		if b.outbox != nil {
			if err := b.outbox.Enqueue(ctx, event); err != nil {
				b.log.Warn("outbox enqueue failed", "error", err, "event", event.Type)
				return err
			}
			return nil
		}
		if err := b.kafka.Publish(ctx, event); err != nil {
			b.log.Warn("kafka publish failed", "error", err, "event", event.Type)
			return err
		}
	}
	return nil
}
