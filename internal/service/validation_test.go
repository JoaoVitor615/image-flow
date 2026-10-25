package service_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/service"
)

func TestValidateFileSize(t *testing.T) {
	svc := service.NewImageService()

	t.Run("should succeed when file size is within limit", func(t *testing.T) {
		input := &service.UploadInput{
			Filename: "test.jpg",
			Size:     service.MaxFileSizeBytes - 100,
			Body:     bytes.NewReader(make([]byte, 10)),
		}

		err := svc.ValidateUpload(context.Background(), input)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("should fail when file size exceeds limit", func(t *testing.T) {
		input := &service.UploadInput{
			Filename: "large.jpg",
			Size:     service.MaxFileSizeBytes + 1,
			Body:     bytes.NewReader(make([]byte, 10)),
		}

		err := svc.ValidateUpload(context.Background(), input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(err, service.ErrFileTooLarge) {
			t.Fatalf("expected ErrFileTooLarge, got %v", err)
		}
	})
}
