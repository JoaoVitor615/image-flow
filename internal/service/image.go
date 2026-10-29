package service

import (
	"context"
	"fmt"

	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/google/uuid"
)

// Repository defines data access methods required by ImageService
type Repository interface {
	CreateImage(ctx context.Context, img *domain.Image) error
	CreateJob(ctx context.Context, job *domain.Job) error
}

// UploadResult contains the generated identifiers and initial status of an upload
type UploadResult struct {
	ImageID uuid.UUID `json:"image_id"`
	JobID   uuid.UUID `json:"job_id"`
	Status  string    `json:"status"`
}

// ImageService orchestrates business logic, storage, and persistence for image uploads
type ImageService struct {
	storage storage.Storage
	repo    Repository
	rules   []ValidationRule
}

// NewImageService initializes an ImageService configured with storage, repository, and validation rules
func NewImageService(storage storage.Storage, repo Repository) *ImageService {
	return &ImageService{
		storage: storage,
		repo:    repo,
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

// Upload validates, saves original file to disk, and persists image and job records
func (s *ImageService) Upload(ctx context.Context, input *UploadInput) (*UploadResult, error) {
	if err := s.ValidateUpload(ctx, input); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%s.%s", uuid.New().String(), input.DetectedFormat)

	savedPath, err := s.storage.SaveOriginal(filename, input.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to save original image to storage: %w", err)
	}

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

	jobRecord := &domain.Job{
		ImageID:     imageRecord.ID,
		Status:      "pending",
		Attempts:    0,
		MaxAttempts: 3,
	}
	if err := s.repo.CreateJob(ctx, jobRecord); err != nil {
		return nil, fmt.Errorf("failed to persist processing job in database: %w", err)
	}

	return &UploadResult{
		ImageID: imageRecord.ID,
		JobID:   jobRecord.ID,
		Status:  jobRecord.Status,
	}, nil
}
