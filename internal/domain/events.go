package domain

import (
	"time"

	"github.com/google/uuid"
)

// ImageUploadedEvent represents the event payload published when an original image is uploaded and ready for processing
type ImageUploadedEvent struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	Version        string    `json:"version"`
	ImageID        uuid.UUID `json:"image_id"`
	JobID          uuid.UUID `json:"job_id"`
	OriginalPath   string    `json:"original_path"`
	OriginalFormat string    `json:"original_format"`
	CreatedAt      time.Time `json:"created_at"`
}
