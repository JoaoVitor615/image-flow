package processor

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "github.com/SeriousBug/webp-go-pure/std"
)

// ProcessedVariant represents a single generated image derivative in memory
type ProcessedVariant struct {
	SizeType      string
	Format        string
	Width         int
	Height        int
	Data          []byte
	FileSizeBytes int64
}

// VariantSpec defines dimension constraints and target formats for a derivative category
type VariantSpec struct {
	SizeType  string
	MaxWidth  int
	MaxHeight int
	Formats   []string
}

// DefaultVariantSpecs specifies the 6 derivative requirements:
// - thumbnail: max 200x200 (preserving aspect ratio) in webp, jpeg, png
// - catalog: max width 800 (preserving aspect ratio) in webp, jpeg, png
var DefaultVariantSpecs = []VariantSpec{
	{
		SizeType:  "thumbnail",
		MaxWidth:  200,
		MaxHeight: 200,
		Formats:   []string{"webp", "jpeg", "png"},
	},
	{
		SizeType:  "catalog",
		MaxWidth:  800,
		MaxHeight: 0,
		Formats:   []string{"webp", "jpeg", "png"},
	},
}

// ImageProcessor handles image decoding, aspect-ratio-preserving resizing, and multi-format encoding
type ImageProcessor struct {
	specs []VariantSpec
}

// NewImageProcessor creates an ImageProcessor with the default 6-variant specification
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		specs: DefaultVariantSpecs,
	}
}

// NewImageProcessorWithSpecs allows injecting custom specifications (useful for testing)
func NewImageProcessorWithSpecs(specs []VariantSpec) *ImageProcessor {
	return &ImageProcessor{
		specs: specs,
	}
}

// ProcessVariants reads an original image stream and generates all configured derivatives
func (p *ImageProcessor) ProcessVariants(r io.Reader) ([]ProcessedVariant, error) {
	srcImg, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source image: %w", err)
	}

	bounds := srcImg.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	var variants []ProcessedVariant

	for _, spec := range p.specs {
		targetW, targetH := CalculateDimensions(origW, origH, spec.MaxWidth, spec.MaxHeight)

		var resized image.Image
		if targetW == origW && targetH == origH {
			resized = srcImg
		} else {
			resized = Resize(srcImg, targetW, targetH)
		}

		for _, format := range spec.Formats {
			data, err := EncodeImage(resized, format)
			if err != nil {
				return nil, fmt.Errorf("failed to encode variant %s/%s: %w", spec.SizeType, format, err)
			}

			variants = append(variants, ProcessedVariant{
				SizeType:      spec.SizeType,
				Format:        format,
				Width:         targetW,
				Height:        targetH,
				Data:          data,
				FileSizeBytes: int64(len(data)),
			})
		}
	}

	return variants, nil
}
