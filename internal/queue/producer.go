package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/segmentio/kafka-go"
)

// EventProducer defines the contract for dispatching domain events
type EventProducer interface {
	PublishImageUploaded(ctx context.Context, event domain.ImageUploadedEvent) error
	Close() error
}

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// KafkaProducer implements EventProducer using segmentio/kafka-go
type KafkaProducer struct {
	writer kafkaWriter
}

// NewKafkaProducer creates a new KafkaProducer configured with broker addresses and target topic
func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  topic,
			Balancer:               &kafka.Hash{},
			AllowAutoTopicCreation: true,
		},
	}
}

// PublishImageUploaded serializes and publishes an ImageUploadedEvent to Kafka
func (p *KafkaProducer) PublishImageUploaded(ctx context.Context, event domain.ImageUploadedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal image uploaded event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(event.ImageID.String()),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "version", Value: []byte(event.Version)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}

// Close closes the underlying Kafka writer connection
func (p *KafkaProducer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
