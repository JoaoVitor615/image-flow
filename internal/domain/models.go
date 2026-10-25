package domain

import (
	"time"

	"github.com/google/uuid"
)

// Image represents the uploaded source image metadata
type Image struct {
	ID             uuid.UUID `json:"id"`
	OriginalPath   string    `json:"original_path"`
	OriginalFormat string    `json:"original_format"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	FileSizeBytes  int64     `json:"file_size_bytes"`
	CreatedAt      time.Time `json:"created_at"`
}

// Job represents the background processing task
type Job struct {
	ID           uuid.UUID `json:"id"`
	ImageID      uuid.UUID `json:"image_id"`
	Status       string    `json:"status"` // pending, processing, completed, failed
	Attempts     int       `json:"attempts"`
	MaxAttempts  int       `json:"max_attempts"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ImageVariant represents one of the 6 generated derivative files
type ImageVariant struct {
	ID            uuid.UUID `json:"id"`
	ImageID       uuid.UUID `json:"image_id"`
	SizeType      string    `json:"size_type"` // thumbnail, catalog
	Format        string    `json:"format"`    // webp, jpeg, png
	Path          string    `json:"path"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
}
