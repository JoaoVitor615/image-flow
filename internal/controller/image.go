package controller

import (
	"errors"
	"net/http"

	"github.com/JoaoVitor615/image-flow/internal/service"
)

// ImageController handles HTTP requests for image-related endpoints
type ImageController struct {
	imageService *service.ImageService
}

// NewImageController creates a new ImageController instance
func NewImageController(imageService *service.ImageService) *ImageController {
	return &ImageController{
		imageService: imageService,
	}
}

// Upload handles POST /images multipart upload requests
func (c *ImageController) Upload(w http.ResponseWriter, r *http.Request) {
	// 10 MB in-memory buffer limit before writing overflow to temp files
	const maxMemory = 10 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		SendError(w, http.StatusBadRequest, "invalid multipart form data")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		SendError(w, http.StatusBadRequest, "missing 'image' file field in request")
		return
	}
	defer file.Close()

	input := &service.UploadInput{
		Filename: header.Filename,
		Size:     header.Size,
		Body:     file,
	}

	if err := c.imageService.ValidateUpload(r.Context(), input); err != nil {
		switch {
		case errors.Is(err, service.ErrFileTooLarge):
			SendError(w, http.StatusRequestEntityTooLarge, err.Error())
		case errors.Is(err, service.ErrUnsupportedFormat):
			SendError(w, http.StatusUnsupportedMediaType, err.Error())
		case errors.Is(err, service.ErrInvalidImage), errors.Is(err, service.ErrInvalidDimensions):
			SendError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			SendError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	SendSuccess(w, http.StatusOK, map[string]any{
		"message": "image accepted",
		"format":  input.DetectedFormat,
		"width":   input.Width,
		"height":  input.Height,
	})
}
