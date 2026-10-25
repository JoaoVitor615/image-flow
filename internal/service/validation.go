package service

import (
	"fmt"
	"io"
)

// UploadInput contains the uploaded file payload and metadata
type UploadInput struct {
	Filename string
	Size     int64
	Body     io.Reader
}

// ValidationRule defines the signature for a validation check on an uploaded image
type ValidationRule func(input *UploadInput) error

// ValidateFileSize checks if the uploaded file exceeds MaxFileSizeBytes
func ValidateFileSize(input *UploadInput) error {
	if input.Size > MaxFileSizeBytes {
		return fmt.Errorf("%w: maximum allowed is %d bytes, got %d", ErrFileTooLarge, MaxFileSizeBytes, input.Size)
	}
	return nil
}
