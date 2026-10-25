package service

import "errors"

// Sentinel errors for image processing and validation
var (
	ErrFileTooLarge      = errors.New("file size exceeds maximum allowed limit")
	ErrUnsupportedFormat = errors.New("unsupported image format (only JPEG, PNG, and WebP are allowed)")
	ErrInvalidImage      = errors.New("corrupted or unreadable image file")
	ErrInvalidDimensions = errors.New("image dimensions are invalid or exceed allowed limits")
)
