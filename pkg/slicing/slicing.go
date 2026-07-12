// Package slicing prepares large images for OCR by cutting them into
// horizontal slices.
//
// OCR engines degrade on very tall images: Apple Vision in particular
// downscales its input internally, so fine print on a tall page (e.g. a web
// page rendered to PDF) simply vanishes — measured on a 3848x17576 page,
// Vision found 3% of the fine-print words, versus 85% when handed just that
// region. Slicing restores full-resolution recognition.
//
// Cutting is delegated to github.com/goodblaster/vertigo/slicer, which only cuts through
// visually blank rows, so text lines are never severed and paragraph
// breaks are preferred.
//
// Callers OCR each slice independently and merge the results back into
// full-page coordinates using each slice's OffsetY.
package slicing

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/goodblaster/vertigo/slicer"
)

// LoadImage decodes a PNG or JPEG image from disk.
func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return img, nil
}

// Config controls when and how images are sliced before OCR.
type Config struct {
	// Enabled turns slicing on. When false, SliceImage always returns the
	// whole image as a single slice.
	Enabled bool

	// HeightThreshold is the image height in pixels above which slicing
	// kicks in. Images at or below the threshold pass through untouched.
	HeightThreshold int

	// TargetHeight is the desired slice height in pixels (soft limit).
	TargetHeight int

	// MinHeight is the minimum slice height in pixels (soft limit). The
	// range between MinHeight and TargetHeight is the window searched for a
	// clean cut.
	MinHeight int
}

// DefaultConfig returns the recommended slicing configuration.
//
// The threshold (4000px) sits between our standard test pages (2112px tall,
// which OCR handles fine) and the tall-page regime where Vision's internal
// downscaling destroys small text. Slice heights target the size range
// measured to OCR at full quality.
func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		HeightThreshold: 4000,
		TargetHeight:    1500,
		MinHeight:       1000,
	}
}

// Slice is one horizontal strip of a larger image.
type Slice struct {
	// Image is the strip itself. Its bounds may be offset within the parent
	// image's coordinate space; use OffsetY, not Bounds().Min.Y.
	Image image.Image

	// OffsetY is the strip's top edge in pixels from the top of the
	// original image.
	OffsetY int
}

// SliceImage cuts img into horizontal strips per cfg. Images that are not
// taller than cfg.HeightThreshold (or when slicing is disabled) are returned
// unmodified as a single slice with OffsetY 0, so callers can use one code
// path for both cases.
func SliceImage(img image.Image, cfg Config) ([]Slice, error) {
	if !cfg.Enabled || img.Bounds().Dy() <= cfg.HeightThreshold {
		return []Slice{{Image: img, OffsetY: 0}}, nil
	}

	slc := slicer.New(cfg.TargetHeight, cfg.MinHeight)
	images := slc.Slice(img)
	if len(images) == 0 {
		return nil, fmt.Errorf("slicing image: slicer returned no slices")
	}

	// Slices are contiguous full-width strips in top-to-bottom order, so
	// each strip's offset is the cumulative height of the strips above it.
	slices := make([]Slice, 0, len(images))
	offset := 0
	for _, si := range images {
		slices = append(slices, Slice{Image: si, OffsetY: offset})
		offset += si.Bounds().Dy()
	}
	return slices, nil
}
