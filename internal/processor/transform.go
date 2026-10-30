package processor

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"strings"

	webp "github.com/SeriousBug/webp-go-pure/std"
	"golang.org/x/image/draw"
)

// CalculateDimensions computes target dimensions preserving aspect ratio without upscaling
func CalculateDimensions(origWidth, origHeight, maxWidth, maxHeight int) (int, int) {
	if origWidth <= 0 || origHeight <= 0 {
		return 0, 0
	}

	// Do not upscale if original already fits within bounds
	fitsWidth := maxWidth == 0 || origWidth <= maxWidth
	fitsHeight := maxHeight == 0 || origHeight <= maxHeight
	if fitsWidth && fitsHeight {
		return origWidth, origHeight
	}

	var ratio float64
	if maxWidth > 0 && maxHeight > 0 {
		ratioW := float64(maxWidth) / float64(origWidth)
		ratioH := float64(maxHeight) / float64(origHeight)
		if ratioW < ratioH {
			ratio = ratioW
		} else {
			ratio = ratioH
		}
	} else if maxWidth > 0 {
		ratio = float64(maxWidth) / float64(origWidth)
	} else if maxHeight > 0 {
		ratio = float64(maxHeight) / float64(origHeight)
	} else {
		return origWidth, origHeight
	}

	newW := int(math.Round(float64(origWidth) * ratio))
	newH := int(math.Round(float64(origHeight) * ratio))

	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	return newW, newH
}

// Resize rescales a source image to the target dimensions using Catmull-Rom high-quality filtering
func Resize(src image.Image, targetWidth, targetHeight int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// EncodeImage encodes an image.Image into the requested target format (png, jpeg, webp)
func EncodeImage(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "jpg" {
		format = "jpeg"
	}

	switch format {
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	case "jpeg":
		opts := &jpeg.Options{Quality: 85}
		if err := jpeg.Encode(&buf, img, opts); err != nil {
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}
	case "webp":
		opts := &webp.Options{Quality: 80}
		if err := webp.Encode(&buf, img, opts); err != nil {
			return nil, fmt.Errorf("failed to encode WebP: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported target format: %s", format)
	}

	return buf.Bytes(), nil
}
