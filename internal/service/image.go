package service

import (
	"context"
)

// ImageService orchestrates business logic and validation for image uploads
type ImageService struct {
	rules []ValidationRule
}

// NewImageService initializes an ImageService configured with default validation rules
func NewImageService() *ImageService {
	return &ImageService{
		rules: []ValidationRule{
			ValidateFileSize,
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
