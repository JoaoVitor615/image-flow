package processor_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/processor"
)

// Helper to generate a solid color test PNG
func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// Helper to generate a solid color test JPEG
func createTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func TestProcessVariants(t *testing.T) {
	proc := processor.NewImageProcessor()

	t.Run("should generate all 6 variants from a valid PNG image", func(t *testing.T) {
		pngBytes := createTestPNG(1200, 800)
		variants, err := proc.ProcessVariants(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("expected successful variant generation, got: %v", err)
		}

		if len(variants) != 6 {
			t.Fatalf("expected exactly 6 variants, got %d", len(variants))
		}

		// Verify expected categories and formats
		expectedVariants := map[string]bool{
			"thumbnail/webp": false,
			"thumbnail/jpeg": false,
			"thumbnail/png":  false,
			"catalog/webp":   false,
			"catalog/jpeg":   false,
			"catalog/png":    false,
		}

		for _, v := range variants {
			key := v.SizeType + "/" + v.Format
			expectedVariants[key] = true

			if len(v.Data) == 0 {
				t.Fatalf("variant %s has empty data", key)
			}
			if v.FileSizeBytes != int64(len(v.Data)) {
				t.Fatalf("variant %s FileSizeBytes mismatch: %d != %d", key, v.FileSizeBytes, len(v.Data))
			}

			if v.SizeType == "thumbnail" {
				if v.Width > 200 || v.Height > 200 {
					t.Fatalf("thumbnail variant exceeds 200x200: got %dx%d", v.Width, v.Height)
				}
				// 1200x800 -> 3:2 ratio -> thumbnail should be 200x133
				if v.Width != 200 || v.Height != 133 {
					t.Fatalf("expected thumbnail 200x133, got %dx%d", v.Width, v.Height)
				}
			}

			if v.SizeType == "catalog" {
				// 1200x800 -> 3:2 ratio -> catalog should be 800x533
				if v.Width != 800 || v.Height != 533 {
					t.Fatalf("expected catalog 800x533, got %dx%d", v.Width, v.Height)
				}
			}
		}

		for key, found := range expectedVariants {
			if !found {
				t.Fatalf("expected variant %s was not generated", key)
			}
		}
	})

	t.Run("should generate all 6 variants from a valid JPEG image", func(t *testing.T) {
		jpegBytes := createTestJPEG(100, 100)
		variants, err := proc.ProcessVariants(bytes.NewReader(jpegBytes))
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if len(variants) != 6 {
			t.Fatalf("expected 6 variants, got %d", len(variants))
		}

		for _, v := range variants {
			// 100x100 is smaller than both thumbnail and catalog max bounds, should preserve 100x100
			if v.Width != 100 || v.Height != 100 {
				t.Fatalf("expected dimensions 100x100 preserved, got %dx%d", v.Width, v.Height)
			}
		}
	})

	t.Run("should return error when decoding corrupted image data", func(t *testing.T) {
		corrupted := []byte("not a valid image byte stream")
		_, err := proc.ProcessVariants(bytes.NewReader(corrupted))
		if err == nil {
			t.Fatal("expected decode error, got nil")
		}
	})

	t.Run("should support custom variant specifications", func(t *testing.T) {
		customSpecs := []processor.VariantSpec{
			{
				SizeType:  "icon",
				MaxWidth:  50,
				MaxHeight: 50,
				Formats:   []string{"png"},
			},
		}
		customProc := processor.NewImageProcessorWithSpecs(customSpecs)

		pngBytes := createTestPNG(200, 200)
		variants, err := customProc.ProcessVariants(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		if len(variants) != 1 {
			t.Fatalf("expected 1 variant, got %d", len(variants))
		}
		if variants[0].SizeType != "icon" || variants[0].Format != "png" {
			t.Fatalf("unexpected variant: %v", variants[0])
		}
		if variants[0].Width != 50 || variants[0].Height != 50 {
			t.Fatalf("expected 50x50, got %dx%d", variants[0].Width, variants[0].Height)
		}
	})
}
