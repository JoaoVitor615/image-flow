package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/segmentio/kafka-go"
)

type kafkaReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// EventConsumer defines the contract for consuming image domain events
type EventConsumer interface {
	FetchImageUploaded(ctx context.Context) (*domain.ImageUploadedEvent, kafka.Message, error)
	Commit(ctx context.Context, msg kafka.Message) error
	Close() error
}

// KafkaConsumer implements EventConsumer using segmentio/kafka-go
type KafkaConsumer struct {
	reader kafkaReader
}

// NewKafkaConsumer creates a new KafkaConsumer configured with brokers, topic, and consumer group
func NewKafkaConsumer(brokers []string, topic string, groupID string) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,               // manual commits via CommitMessages
		StartOffset:    kafka.FirstOffset,
		MaxWait:        3 * time.Second, // 3 seconds timeout
	})

	return &KafkaConsumer{
		reader: reader,
	}
}

// FetchImageUploaded fetches the next message from Kafka and unmarshals it into an ImageUploadedEvent
func (c *KafkaConsumer) FetchImageUploaded(ctx context.Context) (*domain.ImageUploadedEvent, kafka.Message, error) {
	msg, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return nil, kafka.Message{}, err
	}

	var event domain.ImageUploadedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return nil, msg, fmt.Errorf("failed to unmarshal image uploaded event: %w", err)
	}

	return &event, msg, nil
}

// Commit commits the offset of the processed message to Kafka
func (c *KafkaConsumer) Commit(ctx context.Context, msg kafka.Message) error {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to commit message offset: %w", err)
	}
	return nil
}

// Close gracefully closes the consumer connection to Kafka
func (c *KafkaConsumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}
