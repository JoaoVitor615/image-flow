package service

import "errors"

// Sentinel errors for image processing and validation
var (
	ErrFileTooLarge = errors.New("file size exceeds maximum allowed limit")
)
