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

type mockKafkaWriter struct {
	writtenMessages []kafka.Message
	writeErr        error
	closed          bool
	closeErr        error
}

func (m *mockKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.writtenMessages = append(m.writtenMessages, msgs...)
	return nil
}

func (m *mockKafkaWriter) Close() error {
	m.closed = true
	return m.closeErr
}

func TestKafkaProducer_PublishImageUploaded(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully publish ImageUploadedEvent with correct key, headers, and payload", func(t *testing.T) {
		mockWriter := &mockKafkaWriter{}
		producer := &KafkaProducer{writer: mockWriter}

		imageID := uuid.New()
		jobID := uuid.New()
		now := time.Now().UTC()

		event := domain.ImageUploadedEvent{
			EventID:        uuid.New().String(),
			EventType:      "image.uploaded",
			Version:        "v1",
			ImageID:        imageID,
			JobID:          jobID,
			OriginalPath:   "originals/test.png",
			OriginalFormat: "png",
			CreatedAt:      now,
		}

		err := producer.PublishImageUploaded(ctx, event)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(mockWriter.writtenMessages) != 1 {
			t.Fatalf("expected 1 message written, got %d", len(mockWriter.writtenMessages))
		}

		msg := mockWriter.writtenMessages[0]

		// Verify partition key
		if string(msg.Key) != imageID.String() {
			t.Fatalf("expected message key %s, got %s", imageID.String(), string(msg.Key))
		}

		// Verify headers
		var hasEventType, hasVersion bool
		for _, h := range msg.Headers {
			if h.Key == "event_type" && string(h.Value) == "image.uploaded" {
				hasEventType = true
			}
			if h.Key == "version" && string(h.Value) == "v1" {
				hasVersion = true
			}
		}
		if !hasEventType {
			t.Fatal("expected event_type header to be present and match 'image.uploaded'")
		}
		if !hasVersion {
			t.Fatal("expected version header to be present and match 'v1'")
		}

		// Verify JSON payload deserialization
		var unmarshaled domain.ImageUploadedEvent
		if err := json.Unmarshal(msg.Value, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal message payload: %v", err)
		}

		if unmarshaled.ImageID != imageID {
			t.Fatalf("expected payload ImageID %s, got %s", imageID, unmarshaled.ImageID)
		}
		if unmarshaled.JobID != jobID {
			t.Fatalf("expected payload JobID %s, got %s", jobID, unmarshaled.JobID)
		}
		if unmarshaled.OriginalPath != "originals/test.png" {
			t.Fatalf("expected payload OriginalPath 'originals/test.png', got '%s'", unmarshaled.OriginalPath)
		}
		if unmarshaled.OriginalFormat != "png" {
			t.Fatalf("expected payload OriginalFormat 'png', got '%s'", unmarshaled.OriginalFormat)
		}
	})

	t.Run("should return error when writer fails", func(t *testing.T) {
		mockWriter := &mockKafkaWriter{
			writeErr: errors.New("broker connection refused"),
		}
		producer := &KafkaProducer{writer: mockWriter}

		event := domain.ImageUploadedEvent{
			EventID:   uuid.New().String(),
			EventType: "image.uploaded",
			ImageID:   uuid.New(),
			JobID:     uuid.New(),
		}

		err := producer.PublishImageUploaded(ctx, event)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestKafkaProducer_Close(t *testing.T) {
	t.Run("should close writer successfully", func(t *testing.T) {
		mockWriter := &mockKafkaWriter{}
		producer := &KafkaProducer{writer: mockWriter}

		if err := producer.Close(); err != nil {
			t.Fatalf("expected no error closing producer, got: %v", err)
		}

		if !mockWriter.closed {
			t.Fatal("expected writer to be marked as closed")
		}
	})

	t.Run("should handle nil writer safely on close", func(t *testing.T) {
		producer := &KafkaProducer{writer: nil}
		if err := producer.Close(); err != nil {
			t.Fatalf("expected no error on nil writer close, got: %v", err)
		}
	})
}

func TestNewKafkaProducer(t *testing.T) {
	producer := NewKafkaProducer([]string{"localhost:9092"}, "image-uploaded")
	if producer == nil {
		t.Fatal("expected non-nil KafkaProducer")
	}
	_ = producer.Close()
}
