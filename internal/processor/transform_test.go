package processor_test

import (
	"image"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/processor"
)

func TestCalculateDimensions(t *testing.T) {
	tests := []struct {
		name      string
		origW     int
		origH     int
		maxW      int
		maxH      int
		expectedW int
		expectedH int
	}{
		{
			name:      "should return 0x0 for zero or negative dimensions",
			origW:     0,
			origH:     100,
			maxW:      200,
			maxH:      200,
			expectedW: 0,
			expectedH: 0,
		},
		{
			name:      "should not upscale if original fits within constraints",
			origW:     150,
			origH:     100,
			maxW:      200,
			maxH:      200,
			expectedW: 150,
			expectedH: 100,
		},
		{
			name:      "should scale landscape image preserving aspect ratio for thumbnail",
			origW:     1000,
			origH:     500,
			maxW:      200,
			maxH:      200,
			expectedW: 200,
			expectedH: 100,
		},
		{
			name:      "should scale portrait image preserving aspect ratio for thumbnail",
			origW:     500,
			origH:     1000,
			maxW:      200,
			maxH:      200,
			expectedW: 100,
			expectedH: 200,
		},
		{
			name:      "should scale square image preserving 1:1 aspect ratio",
			origW:     600,
			origH:     600,
			maxW:      200,
			maxH:      200,
			expectedW: 200,
			expectedH: 200,
		},
		{
			name:      "should scale width-only constraint for catalog",
			origW:     1600,
			origH:     1200,
			maxW:      800,
			maxH:      0,
			expectedW: 800,
			expectedH: 600,
		},
		{
			name:      "should not scale catalog if original width is smaller than 800",
			origW:     600,
			origH:     400,
			maxW:      800,
			maxH:      0,
			expectedW: 600,
			expectedH: 400,
		},
		{
			name:      "should scale height-only constraint",
			origW:     800,
			origH:     1600,
			maxW:      0,
			maxH:      400,
			expectedW: 200,
			expectedH: 400,
		},
		{
			name:      "should return original dimensions when both maxW and maxH are zero",
			origW:     500,
			origH:     300,
			maxW:      0,
			maxH:      0,
			expectedW: 500,
			expectedH: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := processor.CalculateDimensions(tt.origW, tt.origH, tt.maxW, tt.maxH)
			if gotW != tt.expectedW || gotH != tt.expectedH {
				t.Fatalf("CalculateDimensions(%d, %d, %d, %d) = (%d, %d), expected (%d, %d)",
					tt.origW, tt.origH, tt.maxW, tt.maxH, gotW, gotH, tt.expectedW, tt.expectedH)
			}
		})
	}
}

func TestEncodeImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))

	t.Run("should successfully encode PNG", func(t *testing.T) {
		data, err := processor.EncodeImage(img, "png")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	})

	t.Run("should successfully encode JPEG", func(t *testing.T) {
		data, err := processor.EncodeImage(img, "jpeg")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	})

	t.Run("should successfully encode JPG alias", func(t *testing.T) {
		data, err := processor.EncodeImage(img, "jpg")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	})

	t.Run("should successfully encode WebP", func(t *testing.T) {
		data, err := processor.EncodeImage(img, "webp")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	})

	t.Run("should return error for unsupported format", func(t *testing.T) {
		_, err := processor.EncodeImage(img, "gif")
		if err == nil {
			t.Fatal("expected error for unsupported format GIF, got nil")
		}
	})
}
