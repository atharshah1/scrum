package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

func splitBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type KafkaEventPublisher struct {
	writer *kafka.Writer
	topic  string
}

func NewKafkaEventPublisher(brokers, topic string, batchTimeout time.Duration) *KafkaEventPublisher {
	parsed := splitBrokers(brokers)
	if len(parsed) == 0 || strings.TrimSpace(topic) == "" {
		return nil
	}
	if batchTimeout <= 0 {
		batchTimeout = 20 * time.Millisecond
	}
	return &KafkaEventPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(parsed...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			BatchTimeout: batchTimeout,
		},
		topic: topic,
	}
}

func (p *KafkaEventPublisher) Publish(ctx context.Context, event Event) error {
	if p == nil || p.writer == nil {
		return nil
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.OrgID.String()),
		Value: payload,
		Time:  event.CreatedAt,
	})
}

func (p *KafkaEventPublisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

type KafkaAutomationConsumer struct {
	reader  *kafka.Reader
	log     *slog.Logger
	handler func(Event)
}

func NewKafkaAutomationConsumer(log *slog.Logger, brokers, topic, groupID string, handler func(Event)) *KafkaAutomationConsumer {
	parsed := splitBrokers(brokers)
	if len(parsed) == 0 || strings.TrimSpace(topic) == "" || strings.TrimSpace(groupID) == "" || handler == nil {
		return nil
	}
	return &KafkaAutomationConsumer{
		log:     log,
		handler: handler,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  parsed,
			GroupID:  groupID,
			Topic:    topic,
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
	}
}

func (c *KafkaAutomationConsumer) Start(ctx context.Context) error {
	if c == nil || c.reader == nil {
		return errors.New("kafka consumer not configured")
	}
	defer c.reader.Close()
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		var event Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.log.Warn("automation_kafka_unmarshal_failed", "error", err)
			continue
		}
		c.handler(event)
	}
}
