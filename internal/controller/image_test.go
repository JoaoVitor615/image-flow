package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/controller"
	"github.com/JoaoVitor615/image-flow/internal/domain"
	"github.com/JoaoVitor615/image-flow/internal/repository"
	"github.com/JoaoVitor615/image-flow/internal/service"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type mockRepository struct {
	createImageErr error
	createJobErr   error
	getImageErr    error
	storedImage    *domain.Image
	variants       []domain.ImageVariant
}

func (m *mockRepository) CreateImage(ctx context.Context, img *domain.Image) error {
	if m.createImageErr != nil {
		return m.createImageErr
	}
	img.ID = uuid.New()
	return nil
}

func (m *mockRepository) CreateJob(ctx context.Context, job *domain.Job) error {
	if m.createJobErr != nil {
		return m.createJobErr
	}
	job.ID = uuid.New()
	return nil
}

func (m *mockRepository) GetImageByID(ctx context.Context, id uuid.UUID) (*domain.Image, error) {
	if m.getImageErr != nil {
		return nil, m.getImageErr
	}
	if m.storedImage != nil {
		return m.storedImage, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockRepository) GetVariantsByImageID(ctx context.Context, imageID uuid.UUID) ([]domain.ImageVariant, error) {
	return m.variants, nil
}

// helper to build a multipart request buffer with a specific field and file payload
func createMultipartRequest(fieldName, filename string, content []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, "", err
	}

	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		return nil, "", err
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}

// helper to generate raw PNG bytes
func generatePNGBytes(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestImageController_Upload(t *testing.T) {
	tempStorage, err := storage.NewDiskStorage(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create temp storage: %v", err)
	}

	repo := &mockRepository{}
	imageService := service.NewImageService(tempStorage, repo)
	ctrl := controller.NewImageController(imageService)

	t.Run("should return 202 Accepted when uploading a valid PNG image", func(t *testing.T) {
		pngBytes := generatePNGBytes(50, 50)
		body, contentType, err := createMultipartRequest("image", "avatar.png", pngBytes)
		if err != nil {
			t.Fatalf("failed to create multipart request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/images", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		ctrl.Upload(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected status 202, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var res controller.APIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}

		if !res.Success {
			t.Fatalf("expected success true, got false")
		}

		data := res.Data.(map[string]any)
		if data["image_id"] == "" || data["job_id"] == "" {
			t.Fatalf("expected non-empty image_id and job_id, got %v", data)
		}
		if data["status"] != "pending" {
			t.Fatalf("expected status 'pending', got '%v'", data["status"])
		}
	})

	t.Run("should return 400 Bad Request when multipart form is malformed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/images", bytes.NewReader([]byte("not-multipart")))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid_boundary")
		rec := httptest.NewRecorder()

		ctrl.Upload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("should return 400 Bad Request when image field is missing", func(t *testing.T) {
		pngBytes := generatePNGBytes(50, 50)
		body, contentType, err := createMultipartRequest("wrong_field", "avatar.png", pngBytes)
		if err != nil {
			t.Fatalf("failed to create multipart request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/images", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		ctrl.Upload(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("should return 415 Unsupported Media Type for non-image files", func(t *testing.T) {
		txtContent := []byte("plain text document")
		body, contentType, err := createMultipartRequest("image", "notes.txt", txtContent)
		if err != nil {
			t.Fatalf("failed to create multipart request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/images", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		ctrl.Upload(rec, req)

		if rec.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("expected status 415, got %d", rec.Code)
		}
	})

	t.Run("should return 422 Unprocessable Entity for images with invalid dimensions", func(t *testing.T) {
		tinyPNG := generatePNGBytes(5, 5) // smaller than MinImageDimension (10)
		body, contentType, err := createMultipartRequest("image", "tiny.png", tinyPNG)
		if err != nil {
			t.Fatalf("failed to create multipart request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/images", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		ctrl.Upload(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422, got %d", rec.Code)
		}
	})

	t.Run("should return 500 Internal Server Error when repository fails", func(t *testing.T) {
		failingRepo := &mockRepository{createImageErr: errors.New("db down")}
		failingSvc := service.NewImageService(tempStorage, failingRepo)
		failingCtrl := controller.NewImageController(failingSvc)

		pngBytes := generatePNGBytes(50, 50)
		body, contentType, err := createMultipartRequest("image", "avatar.png", pngBytes)
		if err != nil {
			t.Fatalf("failed to create multipart request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/images", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		failingCtrl.Upload(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})
}

func TestImageController_GetImage(t *testing.T) {
	tempStorage, err := storage.NewDiskStorage(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create temp storage: %v", err)
	}

	pngBytes := generatePNGBytes(40, 30)
	savedPath, err := tempStorage.SaveOriginal("sample.png", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("failed to save sample: %v", err)
	}

	webpBytes := []byte("fake webp bytes")
	variantPath, err := tempStorage.SaveVariant("thumb.webp", bytes.NewReader(webpBytes))
	if err != nil {
		t.Fatalf("failed to save variant: %v", err)
	}

	validID := uuid.New()
	sampleImage := &domain.Image{
		ID:             validID,
		OriginalPath:   savedPath,
		OriginalFormat: "png",
		Width:          40,
		Height:         30,
		FileSizeBytes:  int64(len(pngBytes)),
	}

	sampleVariants := []domain.ImageVariant{
		{
			ID:            uuid.New(),
			ImageID:       validID,
			SizeType:      "thumbnail",
			Format:        "webp",
			Path:          variantPath,
			Width:         40,
			Height:        30,
			FileSizeBytes: int64(len(webpBytes)),
		},
	}

	repo := &mockRepository{
		storedImage: sampleImage,
		variants:    sampleVariants,
	}
	svc := service.NewImageService(tempStorage, repo)
	ctrl := controller.NewImageController(svc)

	router := chi.NewRouter()
	router.Get("/images/{id}", ctrl.GetImage)

	t.Run("should return 200 OK with original image binary", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/images/"+validID.String(), nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "image/png" {
			t.Fatalf("expected image/png, got %s", contentType)
		}
		if rec.Body.Len() == 0 {
			t.Fatal("expected non-empty image content")
		}
	})

	t.Run("should return 200 OK with pre-generated variant", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/images/"+validID.String()+"?format=webp", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "image/webp" {
			t.Fatalf("expected image/webp, got %s", contentType)
		}
		if !bytes.Equal(rec.Body.Bytes(), webpBytes) {
			t.Fatalf("expected variant bytes, got %s", rec.Body.String())
		}
	})

	t.Run("should return 404 Not Found when requested format variant has not been generated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/images/"+validID.String()+"?format=jpeg", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("should return 400 Bad Request for invalid UUID parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/images/not-a-uuid", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("should return 404 Not Found when image does not exist", func(t *testing.T) {
		emptyRepo := &mockRepository{storedImage: nil}
		emptyCtrl := controller.NewImageController(service.NewImageService(tempStorage, emptyRepo))
		r := chi.NewRouter()
		r.Get("/images/{id}", emptyCtrl.GetImage)

		req := httptest.NewRequest(http.MethodGet, "/images/"+uuid.New().String(), nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}
