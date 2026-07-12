//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodblaster/gollate/pkg/ocr/apple"
	"github.com/goodblaster/gollate/pkg/slicing"
)

func main() {
	defaults := slicing.DefaultConfig()

	// Define flags
	var langFlag string
	flag.StringVar(&langFlag, "lang", "", "Language codes (comma-separated, e.g., 'zh-Hans,zh-Hant' for Chinese)")
	sliceEnabled := flag.Bool("slice", defaults.Enabled, "Slice tall images before OCR (Apple Vision downscales tall images, losing small text)")
	sliceThreshold := flag.Int("slice-threshold", defaults.HeightThreshold, "Only slice images taller than this many pixels")
	sliceHeight := flag.Int("slice-height", defaults.TargetHeight, "Target slice height in pixels")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <image-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nRuns Apple Vision OCR on an image and saves JSON output.\n")
		fmt.Fprintf(os.Stderr, "Output file: {basename}-ocr.json\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nLanguage codes:\n")
		fmt.Fprintf(os.Stderr, "  en-US     English (included by default)\n")
		fmt.Fprintf(os.Stderr, "  zh-Hans   Simplified Chinese\n")
		fmt.Fprintf(os.Stderr, "  zh-Hant   Traditional Chinese\n")
		fmt.Fprintf(os.Stderr, "  ja-JP     Japanese\n")
		fmt.Fprintf(os.Stderr, "  ko-KR     Korean\n")
		os.Exit(1)
	}

	imagePath := flag.Arg(0)

	// Verify image exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Image file not found: %s\n", imagePath)
		os.Exit(1)
	}

	img, err := slicing.LoadImage(imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading image: %v\n", err)
		os.Exit(1)
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()

	// Parse language flags
	var langs []string
	if langFlag != "" {
		langs = strings.Split(langFlag, ",")
		for i, lang := range langs {
			langs[i] = strings.TrimSpace(lang)
		}
	}

	fmt.Printf("Processing: %s (%dx%d)\n", imagePath, width, height)
	if len(langs) > 0 {
		fmt.Printf("Languages: %s\n", strings.Join(langs, ", "))
	}

	cfg := slicing.Config{
		Enabled:         *sliceEnabled,
		HeightThreshold: *sliceThreshold,
		TargetHeight:    *sliceHeight,
		MinHeight:       *sliceHeight * 2 / 3,
	}

	lines, err := runOCR(imagePath, img, langs, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error performing OCR: %v\n", err)
		os.Exit(1)
	}

	// Count words
	totalWords := 0
	for _, line := range lines {
		totalWords += len(line.Words)
	}

	// Save JSON
	outputPath := strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + "-ocr.json"
	jsonData, err := json.MarshalIndent(lines, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Saved: %s\n", outputPath)
	fmt.Printf("  Lines: %d\n", len(lines))
	fmt.Printf("  Words: %d\n", totalWords)

	// Print average confidence
	if len(lines) > 0 {
		totalConfidence := 0.0
		for _, line := range lines {
			totalConfidence += line.Confidence
		}
		avgConfidence := totalConfidence / float64(len(lines))
		fmt.Printf("  Avg Confidence: %.2f%%\n", avgConfidence*100)
	}
}

// runOCR performs Apple Vision OCR, slicing tall images first and merging
// per-slice results back into full-page normalized coordinates.
func runOCR(imagePath string, img image.Image, langs []string, cfg slicing.Config) ([]apple.Line, error) {
	engine := &apple.Engine{}

	slices, err := slicing.SliceImage(img, cfg)
	if err != nil {
		return nil, err
	}

	// Unsliced: OCR the original file bytes directly (no re-encode).
	if len(slices) == 1 {
		return engine.ParseFile(imagePath, langs)
	}

	fmt.Printf("  Sliced into %d strips for OCR\n", len(slices))

	pageHeight := float64(img.Bounds().Dy())
	var all []apple.Line
	for i, slice := range slices {
		var buf bytes.Buffer
		if err := png.Encode(&buf, slice.Image); err != nil {
			return nil, fmt.Errorf("encoding slice %d: %w", i, err)
		}
		lines, err := engine.ParseBytes(buf.Bytes(), langs)
		if err != nil {
			return nil, fmt.Errorf("OCR on slice %d: %w", i, err)
		}

		// Rescale normalized coordinates from slice space to page space.
		// Slices span the full page width, so only Y needs adjusting.
		scale := float64(slice.Image.Bounds().Dy()) / pageHeight
		offset := float64(slice.OffsetY) / pageHeight
		for l := range lines {
			lines[l].Rect.Top = lines[l].Rect.Top*scale + offset
			lines[l].Rect.Height *= scale
			for w := range lines[l].Words {
				lines[l].Words[w].Top = lines[l].Words[w].Top*scale + offset
				lines[l].Words[w].Height *= scale
			}
		}
		all = append(all, lines...)
	}
	return all, nil
}
