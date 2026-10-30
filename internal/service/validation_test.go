package service_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/JoaoVitor615/image-flow/internal/service"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/google/uuid"
)

// helper to generate valid PNG bytes in memory
func createTestPNG(width, height int) *bytes.Reader {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return bytes.NewReader(buf.Bytes())
}

// helper to generate valid JPEG bytes in memory
func createTestJPEG(width, height int) *bytes.Reader {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return bytes.NewReader(buf.Bytes())
}

type mockRepository struct {
	createdImage *domain.Image
	createdJob   *domain.Job
	storedImage  *domain.Image
	variants     []domain.ImageVariant
	createImgErr error
	createJobErr error
	getImageErr  error
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

func (m *mockRepository) GetImageByID(ctx context.Context, id uuid.UUID) (*domain.Image, error) {
	if m.getImageErr != nil {
		return nil, m.getImageErr
	}
	if m.storedImage != nil {
		return m.storedImage, nil
	}
	return nil, service.ErrNotFound
}

func (m *mockRepository) GetVariantsByImageID(ctx context.Context, imageID uuid.UUID) ([]domain.ImageVariant, error) {
	return m.variants, nil
}

type mockProducer struct {
	publishedEvent *domain.ImageUploadedEvent
	publishErr     error
}

func (m *mockProducer) PublishImageUploaded(ctx context.Context, event domain.ImageUploadedEvent) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.publishedEvent = &event
	return nil
}

func (m *mockProducer) Close() error {
	return nil
}

func TestValidationPipeline(t *testing.T) {
	svc := service.NewImageService(nil, nil, nil)
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
			t.Fatalf("expected detected format 'png', got '%s'", input.DetectedFormat)
		}
		if input.Width != 100 || input.Height != 100 {
			t.Fatalf("expected 100x100 dimensions, got %dx%d", input.Width, input.Height)
		}
	})

	t.Run("should succeed with a valid JPEG image", func(t *testing.T) {
		reader := createTestJPEG(200, 150)
		input := &service.UploadInput{
			Filename: "sample.jpg",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		err := svc.ValidateUpload(ctx, input)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if input.DetectedFormat != "jpeg" {
			t.Fatalf("expected detected format 'jpeg', got '%s'", input.DetectedFormat)
		}
		if input.Width != 200 || input.Height != 150 {
			t.Fatalf("expected 200x150 dimensions, got %dx%d", input.Width, input.Height)
		}
	})

	t.Run("should fail when file size exceeds 10MB limit", func(t *testing.T) {
		input := &service.UploadInput{
			Filename: "huge.png",
			Size:     11 * 1024 * 1024,
			Body:     bytes.NewReader([]byte("dummy content")),
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrFileTooLarge) {
			t.Fatalf("expected ErrFileTooLarge, got %v", err)
		}
	})

	t.Run("should fail with unsupported content type (e.g. plain text)", func(t *testing.T) {
		textContent := []byte("plain text content pretending to be an image")
		input := &service.UploadInput{
			Filename: "fake.txt",
			Size:     int64(len(textContent)),
			Body:     bytes.NewReader(textContent),
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrUnsupportedFormat) {
			t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
		}
	})

	t.Run("should fail when image width is below minimum 10px", func(t *testing.T) {
		reader := createTestPNG(8, 50)
		input := &service.UploadInput{
			Filename: "small.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrInvalidDimensions) {
			t.Fatalf("expected ErrInvalidDimensions, got %v", err)
		}
	})

	t.Run("should fail when image height is below minimum 10px", func(t *testing.T) {
		reader := createTestPNG(50, 8)
		input := &service.UploadInput{
			Filename: "small_height.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		err := svc.ValidateUpload(ctx, input)
		if !errors.Is(err, service.ErrInvalidDimensions) {
			t.Fatalf("expected ErrInvalidDimensions, got %v", err)
		}
	})

	t.Run("should fail with corrupted image payload", func(t *testing.T) {
		// JPEG magic bytes followed by garbage
		corrupted := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x00, 0x01}
		input := &service.UploadInput{
			Filename: "corrupt.jpg",
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

	t.Run("should successfully save to storage, persist image and job, and publish event", func(t *testing.T) {
		strg, err := storage.NewDiskStorage(t.TempDir())
		if err != nil {
			t.Fatalf("failed to create test storage: %v", err)
		}

		repo := &mockRepository{}
		prod := &mockProducer{}
		svc := service.NewImageService(strg, repo, prod)

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

		// Verify event was published to message queue
		if prod.publishedEvent == nil {
			t.Fatal("expected event to be published to producer")
		}
		if prod.publishedEvent.ImageID != repo.createdImage.ID {
			t.Fatalf("expected event ImageID %s, got %s", repo.createdImage.ID, prod.publishedEvent.ImageID)
		}
		if prod.publishedEvent.JobID != repo.createdJob.ID {
			t.Fatalf("expected event JobID %s, got %s", repo.createdJob.ID, prod.publishedEvent.JobID)
		}
		if prod.publishedEvent.EventType != "image.uploaded" {
			t.Fatalf("expected event type 'image.uploaded', got '%s'", prod.publishedEvent.EventType)
		}
		if prod.publishedEvent.OriginalFormat != "png" {
			t.Fatalf("expected event format 'png', got '%s'", prod.publishedEvent.OriginalFormat)
		}
	})

	t.Run("should fail when database image insert fails", func(t *testing.T) {
		strg, _ := storage.NewDiskStorage(t.TempDir())
		repo := &mockRepository{
			createImgErr: errors.New("db error"),
		}
		svc := service.NewImageService(strg, repo, nil)

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
		svc := service.NewImageService(strg, repo, nil)

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

	t.Run("should fail when event producer fails", func(t *testing.T) {
		strg, _ := storage.NewDiskStorage(t.TempDir())
		repo := &mockRepository{}
		prod := &mockProducer{
			publishErr: errors.New("kafka unavailable"),
		}
		svc := service.NewImageService(strg, repo, prod)

		reader := createTestPNG(50, 50)
		input := &service.UploadInput{
			Filename: "product.png",
			Size:     int64(reader.Len()),
			Body:     reader,
		}

		_, err := svc.Upload(ctx, input)
		if err == nil {
			t.Fatal("expected error when event producer fails, got nil")
		}
	})
}

func TestImageService_GetImageFile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	strg, err := storage.NewDiskStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Save test original image to storage
	pngReader := createTestPNG(60, 40)
	origPath, err := strg.SaveOriginal("test-orig.png", pngReader)
	if err != nil {
		t.Fatalf("failed to save original: %v", err)
	}

	// Save test variant image to storage
	webpBytes := []byte("fake webp variant bytes")
	variantPath, err := strg.SaveVariant("thumb.webp", bytes.NewReader(webpBytes))
	if err != nil {
		t.Fatalf("failed to save variant: %v", err)
	}

	imgID := uuid.New()
	sampleRecord := &domain.Image{
		ID:             imgID,
		OriginalPath:   origPath,
		OriginalFormat: "png",
		Width:          60,
		Height:         40,
		FileSizeBytes:  int64(pngReader.Len()),
	}

	sampleVariants := []domain.ImageVariant{
		{
			ID:            uuid.New(),
			ImageID:       imgID,
			SizeType:      "thumbnail",
			Format:        "webp",
			Path:          variantPath,
			Width:         60,
			Height:        40,
			FileSizeBytes: int64(len(webpBytes)),
		},
	}

	t.Run("should return original file when no format is specified", func(t *testing.T) {
		repo := &mockRepository{storedImage: sampleRecord}
		svc := service.NewImageService(strg, repo, nil)

		rc, mimeType, err := svc.GetImageFile(ctx, imgID, "", "")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		defer rc.Close()

		if mimeType != "image/png" {
			t.Fatalf("expected mime image/png, got %s", mimeType)
		}
		content, _ := io.ReadAll(rc)
		if len(content) == 0 {
			t.Fatal("expected non-empty content")
		}
	})

	t.Run("should return pre-generated variant when format exists in variants", func(t *testing.T) {
		repo := &mockRepository{
			storedImage: sampleRecord,
			variants:    sampleVariants,
		}
		svc := service.NewImageService(strg, repo, nil)

		rc, mimeType, err := svc.GetImageFile(ctx, imgID, "webp", "")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		defer rc.Close()

		if mimeType != "image/webp" {
			t.Fatalf("expected mime image/webp, got %s", mimeType)
		}
		content, _ := io.ReadAll(rc)
		if !bytes.Equal(content, webpBytes) {
			t.Fatalf("expected variant bytes, got %s", content)
		}
	})

	t.Run("should return pre-generated variant when size matches", func(t *testing.T) {
		repo := &mockRepository{
			storedImage: sampleRecord,
			variants:    sampleVariants,
		}
		svc := service.NewImageService(strg, repo, nil)

		rc, mimeType, err := svc.GetImageFile(ctx, imgID, "", "thumbnail")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		defer rc.Close()

		if mimeType != "image/webp" {
			t.Fatalf("expected mime image/webp, got %s", mimeType)
		}
	})

	t.Run("should return pre-generated variant when both size and format match", func(t *testing.T) {
		repo := &mockRepository{
			storedImage: sampleRecord,
			variants:    sampleVariants,
		}
		svc := service.NewImageService(strg, repo, nil)

		rc, mimeType, err := svc.GetImageFile(ctx, imgID, "webp", "thumbnail")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		defer rc.Close()

		if mimeType != "image/webp" {
			t.Fatalf("expected mime image/webp, got %s", mimeType)
		}
	})

	t.Run("should return ErrVariantNotFound when format has not been generated", func(t *testing.T) {
		repo := &mockRepository{
			storedImage: sampleRecord,
			variants:    sampleVariants, // only has webp, not jpeg
		}
		svc := service.NewImageService(strg, repo, nil)

		_, _, err := svc.GetImageFile(ctx, imgID, "jpeg", "")
		if !errors.Is(err, service.ErrVariantNotFound) {
			t.Fatalf("expected ErrVariantNotFound, got %v", err)
		}
	})

	t.Run("should return error when image is not found in database", func(t *testing.T) {
		repo := &mockRepository{storedImage: nil}
		svc := service.NewImageService(strg, repo, nil)

		_, _, err := svc.GetImageFile(ctx, uuid.New(), "", "")
		if !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
