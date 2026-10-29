package controller

import (
	"errors"
	"io"
	"net/http"

	"github.com/JoaoVitor615/image-flow/internal/repository"
	"github.com/JoaoVitor615/image-flow/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

	result, err := c.imageService.Upload(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFileTooLarge):
			SendError(w, http.StatusRequestEntityTooLarge, err.Error())
		case errors.Is(err, service.ErrUnsupportedFormat):
			SendError(w, http.StatusUnsupportedMediaType, err.Error())
		case errors.Is(err, service.ErrInvalidImage), errors.Is(err, service.ErrInvalidDimensions):
			SendError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			SendError(w, http.StatusInternalServerError, "failed to process image upload")
		}
		return
	}

	// Return 202 Accepted with generated identifiers and pending status
	SendSuccess(w, http.StatusAccepted, result)
}

// GetImage handles GET /images/{id} returning original or pre-generated variant from disk
func (c *ImageController) GetImage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		SendError(w, http.StatusBadRequest, "invalid image ID format")
		return
	}

	targetFormat := r.URL.Query().Get("format")

	content, mimeType, err := c.imageService.GetImageFile(r.Context(), id, targetFormat)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound), errors.Is(err, service.ErrNotFound):
			SendError(w, http.StatusNotFound, "image not found")
		case errors.Is(err, service.ErrVariantNotFound):
			SendError(w, http.StatusNotFound, err.Error())
		default:
			SendError(w, http.StatusInternalServerError, "failed to retrieve image")
		}
		return
	}
	defer content.Close()

	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, content)
}
