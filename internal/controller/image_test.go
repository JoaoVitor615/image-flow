package controller_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/controller"
	"github.com/JoaoVitor615/image-flow/internal/service"
)

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
	imageService := service.NewImageService(nil, nil)
	ctrl := controller.NewImageController(imageService)

	t.Run("should return 200 OK when uploading a valid PNG image", func(t *testing.T) {
		pngBytes := generatePNGBytes(50, 50)
		body, contentType, err := createMultipartRequest("image", "avatar.png", pngBytes)
		if err != nil {
			t.Fatalf("failed to create multipart request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/images", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		ctrl.Upload(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var res controller.APIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}

		if !res.Success {
			t.Fatalf("expected success true, got false")
		}

		data := res.Data.(map[string]any)
		if data["format"] != "png" {
			t.Fatalf("expected format 'png', got '%v'", data["format"])
		}
		if int(data["width"].(float64)) != 50 || int(data["height"].(float64)) != 50 {
			t.Fatalf("expected dimensions 50x50, got %vx%v", data["width"], data["height"])
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
		// Field named 'wrong_field' instead of 'image'
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
}
