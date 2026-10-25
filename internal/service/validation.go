package service

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	_ "golang.org/x/image/webp"
)

// UploadInput contains the uploaded file payload, stream, and populated metadata
type UploadInput struct {
	Filename       string
	Size           int64
	Body           io.ReadSeeker
	DetectedFormat string
	Width          int
	Height         int
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

// ValidateContentType inspects magic bytes (content sniffing) to verify supported image formats
func ValidateContentType(input *UploadInput) error {
	// Read the first 512 bytes for MIME type sniffing
	buffer := make([]byte, 512)
	n, err := io.ReadFull(input.Body, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("failed to read file header: %w", err)
	}

	// Always rewind stream so subsequent steps start from byte 0
	if _, err := input.Body.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind file stream: %w", err)
	}

	sniffed := buffer[:n]
	mimeType := http.DetectContentType(sniffed)

	// Check supported formats (JPEG, PNG, WebP)
	switch {
	case mimeType == "image/jpeg":
		input.DetectedFormat = "jpeg"
	case mimeType == "image/png":
		input.DetectedFormat = "png"
	case mimeType == "image/webp" || isWebP(sniffed):
		input.DetectedFormat = "webp"
	default:
		return fmt.Errorf("%w: detected %s", ErrUnsupportedFormat, mimeType)
	}

	return nil
}

// ValidateImageDimensions decodes image configuration to verify integrity and dimension bounds
func ValidateImageDimensions(input *UploadInput) error {
	cfg, _, err := image.DecodeConfig(input.Body)
	if err != nil {
		// Rewind in case of failure
		_, _ = input.Body.Seek(0, io.SeekStart)
		return fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}

	// Always rewind stream back to the beginning
	if _, err := input.Body.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind file stream: %w", err)
	}

	// Validate dimension boundaries
	if cfg.Width < MinImageDimension || cfg.Height < MinImageDimension {
		return fmt.Errorf("%w: dimensions %dx%d smaller than minimum %dx%d",
			ErrInvalidDimensions, cfg.Width, cfg.Height, MinImageDimension, MinImageDimension)
	}

	if cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return fmt.Errorf("%w: dimensions %dx%d exceed maximum allowed %dx%d",
			ErrInvalidDimensions, cfg.Width, cfg.Height, MaxImageDimension, MaxImageDimension)
	}

	input.Width = cfg.Width
	input.Height = cfg.Height

	return nil
}

// isWebP verifies RIFF/WEBP magic bytes
func isWebP(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	return bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP"
}
