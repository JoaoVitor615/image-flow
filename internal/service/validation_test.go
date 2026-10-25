package service_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/service"
)

// helper to create a valid in-memory PNG image
func createTestPNG(width, height int) *bytes.Reader {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return bytes.NewReader(buf.Bytes())
}

// helper to create a valid in-memory JPEG image
func createTestJPEG(width, height int) *bytes.Reader {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return bytes.NewReader(buf.Bytes())
}

func TestValidationPipeline(t *testing.T) {
	svc := service.NewImageService()
	ctx := context.Background()

	t.Run("should succeed with a valid PNG image", func(t *testing.T) {
		reader := createTestPNG(100, 100)
		input := &service.UploadInput{
			Filename: "sample.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		err := svc.ValidateUpload(ctx, input)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if input.DetectedFormat != "png" {
			t.Fatalf("expected format 'png', got '%s'", input.DetectedFormat)
		}
		if input.Width != 100 || input.Height != 100 {
			t.Fatalf("expected dimensions 100x100, got %dx%d", input.Width, input.Height)
		}
	})

	t.Run("should succeed with a valid JPEG image", func(t *testing.T) {
		reader := createTestJPEG(200, 150)
		input := &service.UploadInput{
			Filename: "photo.jpg",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		err := svc.ValidateUpload(ctx, input)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if input.DetectedFormat != "jpeg" {
			t.Fatalf("expected format 'jpeg', got '%s'", input.DetectedFormat)
		}
		if input.Width != 200 || input.Height != 150 {
			t.Fatalf("expected dimensions 200x150, got %dx%d", input.Width, input.Height)
		}
	})

	t.Run("should fail when file size exceeds maximum limit", func(t *testing.T) {
		reader := createTestPNG(20, 20)
		input := &service.UploadInput{
			Filename: "large.png",
			Size:     service.MaxFileSizeBytes + 1,
			Body:     reader,
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrFileTooLarge) {
			t.Fatalf("expected ErrFileTooLarge, got %v", err)
		}
	})

	t.Run("should fail with unsupported content type", func(t *testing.T) {
		fakeContent := []byte("plain text content pretending to be an image")
		input := &service.UploadInput{
			Filename: "fake.txt",
			Size:     int64(len(fakeContent)),
			Body:     bytes.NewReader(fakeContent),
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrUnsupportedFormat) {
			t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
		}
	})

	t.Run("should fail with dimensions smaller than minimum allowed", func(t *testing.T) {
		reader := createTestPNG(5, 5) // MinImageDimension is 10
		input := &service.UploadInput{
			Filename: "tiny.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrInvalidDimensions) {
			t.Fatalf("expected ErrInvalidDimensions, got %v", err)
		}
	})

	t.Run("should fail with corrupted image body", func(t *testing.T) {
		// Valid PNG magic bytes followed by garbage
		corrupted := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("not_a_real_png")...)
		input := &service.UploadInput{
			Filename: "corrupted.png",
			Size:     int64(len(corrupted)),
			Body:     bytes.NewReader(corrupted),
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrInvalidImage) {
			t.Fatalf("expected ErrInvalidImage, got %v", err)
		}
	})
}
