package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/JoaoVitor615/image-flow/internal/queue"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/google/uuid"
)

// Repository defines data access methods required by ImageService
type Repository interface {
	CreateImage(ctx context.Context, img *domain.Image) error
	CreateJob(ctx context.Context, job *domain.Job) error
	GetImageByID(ctx context.Context, id uuid.UUID) (*domain.Image, error)
	GetVariantsByImageID(ctx context.Context, imageID uuid.UUID) ([]domain.ImageVariant, error)
}

// UploadResult contains the generated identifiers and initial status of an upload
type UploadResult struct {
	ImageID uuid.UUID `json:"image_id"`
	JobID   uuid.UUID `json:"job_id"`
	Status  string    `json:"status"`
}

// ImageService orchestrates business logic, storage, persistence, and event dispatch for images
type ImageService struct {
	storage  storage.Storage
	repo     Repository
	producer queue.EventProducer
	rules    []ValidationRule
}

// NewImageService initializes an ImageService configured with storage, repository, producer, and validation rules
func NewImageService(storage storage.Storage, repo Repository, producer queue.EventProducer) *ImageService {
	return &ImageService{
		storage:  storage,
		repo:     repo,
		producer: producer,
		rules: []ValidationRule{
			ValidateFileSize,
			ValidateContentType,
			ValidateImageDimensions,
		},
	}
}

// ValidateUpload executes all registered validation rules against the uploaded input
func (s *ImageService) ValidateUpload(ctx context.Context, input *UploadInput) error {
	for _, rule := range s.rules {
		if err := rule(input); err != nil {
			return err
		}
	}
	return nil
}

// Upload validates, saves original file to disk, persists image and job records, and publishes the upload event
func (s *ImageService) Upload(ctx context.Context, input *UploadInput) (*UploadResult, error) {
	// Run all validations (size, content type, dimensions)
	if err := s.ValidateUpload(ctx, input); err != nil {
		return nil, err
	}

	// Generate a secure, unique filename (UUID + detected format)
	filename := fmt.Sprintf("%s.%s", uuid.New().String(), input.DetectedFormat)

	// Save original file to persistent storage
	savedPath, err := s.storage.SaveOriginal(filename, input.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to save original image to storage: %w", err)
	}

	// Persist image metadata into PostgreSQL
	imageRecord := &domain.Image{
		OriginalPath:   savedPath,
		OriginalFormat: input.DetectedFormat,
		Width:          input.Width,
		Height:         input.Height,
		FileSizeBytes:  input.Size,
	}
	if err := s.repo.CreateImage(ctx, imageRecord); err != nil {
		return nil, fmt.Errorf("failed to persist image in database: %w", err)
	}

	// Persist processing job with initial 'pending' status
	jobRecord := &domain.Job{
		ImageID:     imageRecord.ID,
		Status:      "pending",
		Attempts:    0,
		MaxAttempts: 3,
	}
	if err := s.repo.CreateJob(ctx, jobRecord); err != nil {
		return nil, fmt.Errorf("failed to persist processing job in database: %w", err)
	}

	// Publish upload event to message broker for background variant processing
	if s.producer != nil {

		event := domain.ImageUploadedEvent{
			EventID:        uuid.New().String(),
			EventType:      "image.uploaded",
			Version:        "v1",
			ImageID:        imageRecord.ID,
			JobID:          jobRecord.ID,
			OriginalPath:   savedPath,
			OriginalFormat: input.DetectedFormat,
			CreatedAt:      time.Now().UTC(),
		}

		if err := s.producer.PublishImageUploaded(ctx, event); err != nil {
			return nil, fmt.Errorf("failed to publish image uploaded event: %w", err)
		}
	}

	return &UploadResult{
		ImageID: imageRecord.ID,
		JobID:   jobRecord.ID,
		Status:  jobRecord.Status,
	}, nil
}

// GetImageFile retrieves the image file stream from disk, returning original or locating a pre-generated variant
func (s *ImageService) GetImageFile(ctx context.Context, id uuid.UUID, targetFormat, targetSize string) (io.ReadCloser, string, error) {
	imgRecord, err := s.repo.GetImageByID(ctx, id)
	if err != nil {
		return nil, "", err
	}

	targetFormat = normalizeFormat(targetFormat)
	targetSize = strings.ToLower(strings.TrimSpace(targetSize))

	// Return original image if no resize is requested and format matches or is unspecified
	if targetSize == "" && (targetFormat == "" || targetFormat == imgRecord.OriginalFormat) {
		return s.openImage(imgRecord.OriginalPath, imgRecord.OriginalFormat)
	}

	variants, err := s.repo.GetVariantsByImageID(ctx, id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query image variants: %w", err)
	}

	matchedVariant := selectVariant(variants, targetFormat, targetSize, imgRecord.OriginalFormat)
	if matchedVariant == nil {
		return nil, "", ErrVariantNotFound
	}

	return s.openImage(matchedVariant.Path, matchedVariant.Format)
}

func (s *ImageService) openImage(path, format string) (io.ReadCloser, string, error) {
	file, err := s.storage.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open stored image: %w", err)
	}
	return file, "image/" + format, nil
}

func selectVariant(variants []domain.ImageVariant, targetFormat, targetSize, defaultFormat string) *domain.ImageVariant {
	var candidate *domain.ImageVariant

	for i := range variants {
		v := &variants[i]

		if targetSize != "" && v.SizeType != targetSize {
			continue
		}
		if targetFormat != "" && v.Format != targetFormat {
			continue
		}

		// When format is explicitly requested, match is exact
		if targetFormat != "" {
			return v
		}

		// When only size is requested, prioritize original format
		if v.Format == defaultFormat {
			return v
		}
		if candidate == nil {
			candidate = v
		}
	}

	return candidate
}

func normalizeFormat(format string) string {
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "jpg" {
		return "jpeg"
	}
	return f
}
