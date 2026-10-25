package service

// Validation constants and business limits
const (
	// MaxFileSizeBytes represents the maximum allowed upload file size (10 MB)
	MaxFileSizeBytes int64 = 10 * 1024 * 1024

	// MinImageDimension defines the minimum allowed width/height in pixels
	MinImageDimension int = 10

	// MaxImageDimension defines the maximum allowed width/height in pixels to avoid decompression bombs
	MaxImageDimension int = 8192
)
