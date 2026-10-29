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

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/JoaoVitor615/image-flow/internal/service"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/google/uuid"
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

// mock repository for testing ImageService persistence
type mockRepository struct {
	createdImage *domain.Image
	createdJob   *domain.Job
	createImgErr error
	createJobErr error
}

func (m *mockRepository) CreateImage(ctx context.Context, img *domain.Image) error {
	if m.createImgErr != nil {
		return m.createImgErr
	}
	img.ID = uuid.New()
	m.createdImage = img
	return nil
}

func (m *mockRepository) CreateJob(ctx context.Context, job *domain.Job) error {
	if m.createJobErr != nil {
		return m.createJobErr
	}
	job.ID = uuid.New()
	m.createdJob = job
	return nil
}

func TestValidationPipeline(t *testing.T) {
	svc := service.NewImageService(nil, nil)
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

func TestImageService_Upload(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully save to storage and persist image and job", func(t *testing.T) {
		strg, err := storage.NewDiskStorage(t.TempDir())
		if err != nil {
			t.Fatalf("failed to create test storage: %v", err)
		}

		repo := &mockRepository{}
		svc := service.NewImageService(strg, repo)

		reader := createTestPNG(120, 80)
		input := &service.UploadInput{
			Filename: "product.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		result, err := svc.Upload(ctx, input)
		if err != nil {
			t.Fatalf("expected successful upload, got %v", err)
		}

		if result == nil {
			t.Fatal("expected non-nil upload result")
		}
		if result.ImageID == uuid.Nil {
			t.Fatal("expected valid ImageID, got nil UUID")
		}
		if result.JobID == uuid.Nil {
			t.Fatal("expected valid JobID, got nil UUID")
		}
		if result.Status != "pending" {
			t.Fatalf("expected status 'pending', got '%s'", result.Status)
		}

		// Verify repository received records
		if repo.createdImage == nil {
			t.Fatal("expected image record to be created in repository")
		}
		if repo.createdImage.Width != 120 || repo.createdImage.Height != 80 {
			t.Fatalf("expected 120x80, got %dx%d", repo.createdImage.Width, repo.createdImage.Height)
		}
		if repo.createdImage.OriginalFormat != "png" {
			t.Fatalf("expected format 'png', got '%s'", repo.createdImage.OriginalFormat)
		}

		if repo.createdJob == nil {
			t.Fatal("expected job record to be created in repository")
		}
		if repo.createdJob.ImageID != repo.createdImage.ID {
			t.Fatalf("expected job to link to image ID %s, got %s", repo.createdImage.ID, repo.createdJob.ImageID)
		}
		if repo.createdJob.Status != "pending" {
			t.Fatalf("expected job status 'pending', got '%s'", repo.createdJob.Status)
		}
	})

	t.Run("should fail when database image insert fails", func(t *testing.T) {
		strg, _ := storage.NewDiskStorage(t.TempDir())
		repo := &mockRepository{
			createImgErr: errors.New("db error"),
		}
		svc := service.NewImageService(strg, repo)

		reader := createTestPNG(50, 50)
		input := &service.UploadInput{
			Filename: "product.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		_, err := svc.Upload(ctx, input)
		if err == nil {
			t.Fatal("expected error from failed db insert, got nil")
		}
	})

	t.Run("should fail when database job insert fails", func(t *testing.T) {
		strg, _ := storage.NewDiskStorage(t.TempDir())
		repo := &mockRepository{
			createJobErr: errors.New("db error"),
		}
		svc := service.NewImageService(strg, repo)

		reader := createTestPNG(50, 50)
		input := &service.UploadInput{
			Filename: "product.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		_, err := svc.Upload(ctx, input)
		if err == nil {
			t.Fatal("expected error from failed job insert, got nil")
		}
	})
}
