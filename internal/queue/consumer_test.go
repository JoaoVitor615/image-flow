package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type mockKafkaReader struct {
	messageToFetch kafka.Message
	fetchErr       error
	committedMsgs  []kafka.Message
	commitErr      error
	closed         bool
	closeErr       error
}

func (m *mockKafkaReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if m.fetchErr != nil {
		return kafka.Message{}, m.fetchErr
	}
	return m.messageToFetch, nil
}

func (m *mockKafkaReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.commitErr != nil {
		return m.commitErr
	}
	m.committedMsgs = append(m.committedMsgs, msgs...)
	return nil
}

func (m *mockKafkaReader) Close() error {
	m.closed = true
	return m.closeErr
}

func TestKafkaConsumer_FetchImageUploaded(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully fetch and deserialize ImageUploadedEvent", func(t *testing.T) {
		expectedEvent := domain.ImageUploadedEvent{
			EventID:        uuid.New().String(),
			EventType:      "image.uploaded",
			Version:        "v1",
			ImageID:        uuid.New(),
			JobID:          uuid.New(),
			OriginalPath:   "originals/product.png",
			OriginalFormat: "png",
			CreatedAt:      time.Now().UTC(),
		}

		payload, err := json.Marshal(expectedEvent)
		if err != nil {
			t.Fatalf("failed to marshal expected event: %v", err)
		}

		mockReader := &mockKafkaReader{
			messageToFetch: kafka.Message{
				Topic:     "image-uploaded",
				Partition: 0,
				Offset:    42,
				Key:       []byte(expectedEvent.ImageID.String()),
				Value:     payload,
			},
		}

		consumer := &KafkaConsumer{reader: mockReader}

		event, msg, err := consumer.FetchImageUploaded(ctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if event == nil {
			t.Fatal("expected non-nil event")
		}
		if event.ImageID != expectedEvent.ImageID {
			t.Fatalf("expected ImageID %s, got %s", expectedEvent.ImageID, event.ImageID)
		}
		if event.JobID != expectedEvent.JobID {
			t.Fatalf("expected JobID %s, got %s", expectedEvent.JobID, event.JobID)
		}
		if event.OriginalPath != expectedEvent.OriginalPath {
			t.Fatalf("expected OriginalPath %s, got %s", expectedEvent.OriginalPath, event.OriginalPath)
		}
		if msg.Offset != 42 {
			t.Fatalf("expected message offset 42, got %d", msg.Offset)
		}
	})

	t.Run("should return error when reader fetch fails", func(t *testing.T) {
		mockReader := &mockKafkaReader{
			fetchErr: errors.New("context deadline exceeded"),
		}
		consumer := &KafkaConsumer{reader: mockReader}

		_, _, err := consumer.FetchImageUploaded(ctx)
		if err == nil {
			t.Fatal("expected fetch error, got nil")
		}
	})

	t.Run("should return error when payload is malformed JSON", func(t *testing.T) {
		mockReader := &mockKafkaReader{
			messageToFetch: kafka.Message{
				Value: []byte("invalid json content"),
			},
		}
		consumer := &KafkaConsumer{reader: mockReader}

		event, rawMsg, err := consumer.FetchImageUploaded(ctx)
		if err == nil {
			t.Fatal("expected unmarshal error, got nil")
		}
		if event != nil {
			t.Fatal("expected nil event for malformed payload")
		}
		if string(rawMsg.Value) != "invalid json content" {
			t.Fatalf("expected raw message returned, got: %s", string(rawMsg.Value))
		}
	})
}

func TestKafkaConsumer_Commit(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully commit message offset", func(t *testing.T) {
		mockReader := &mockKafkaReader{}
		consumer := &KafkaConsumer{reader: mockReader}

		msg := kafka.Message{
			Topic:     "image-uploaded",
			Partition: 1,
			Offset:    10,
		}

		if err := consumer.Commit(ctx, msg); err != nil {
			t.Fatalf("expected no error committing, got: %v", err)
		}

		if len(mockReader.committedMsgs) != 1 {
			t.Fatalf("expected 1 committed message, got %d", len(mockReader.committedMsgs))
		}
		if mockReader.committedMsgs[0].Offset != 10 {
			t.Fatalf("expected offset 10, got %d", mockReader.committedMsgs[0].Offset)
		}
	})

	t.Run("should return error when commit fails", func(t *testing.T) {
		mockReader := &mockKafkaReader{
			commitErr: errors.New("broker unavailable"),
		}
		consumer := &KafkaConsumer{reader: mockReader}

		msg := kafka.Message{Offset: 5}
		if err := consumer.Commit(ctx, msg); err == nil {
			t.Fatal("expected commit error, got nil")
		}
	})
}

func TestKafkaConsumer_Close(t *testing.T) {
	t.Run("should successfully close consumer connection", func(t *testing.T) {
		mockReader := &mockKafkaReader{}
		consumer := &KafkaConsumer{reader: mockReader}

		if err := consumer.Close(); err != nil {
			t.Fatalf("expected no error closing consumer, got: %v", err)
		}
		if !mockReader.closed {
			t.Fatal("expected reader to be marked as closed")
		}
	})

	t.Run("should handle nil reader gracefully", func(t *testing.T) {
		consumer := &KafkaConsumer{reader: nil}
		if err := consumer.Close(); err != nil {
			t.Fatalf("expected no error on nil reader close, got: %v", err)
		}
	})
}

func TestNewKafkaConsumer(t *testing.T) {
	consumer := NewKafkaConsumer([]string{"localhost:9092"}, "image-uploaded", "image-processing-group")
	if consumer == nil {
		t.Fatal("expected non-nil KafkaConsumer")
	}
	_ = consumer.Close()
}
