package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goodblaster/gollate/pkg/engines/tesseract"
	"github.com/goodblaster/gollate/pkg/slicing"
)

func main() {
	defaults := slicing.DefaultConfig()

	// Define flags
	var langFlag string
	var psmFlag string
	flag.StringVar(&langFlag, "lang", "", "Language codes (comma-separated, e.g., 'jpn,eng' for Japanese+English)")
	flag.StringVar(&psmFlag, "psm", "", "Tesseract page segmentation mode (e.g. 5 = single uniform block of vertical text)")
	sliceEnabled := flag.Bool("slice", defaults.Enabled, "Slice tall images before OCR")
	sliceThreshold := flag.Int("slice-threshold", defaults.HeightThreshold, "Only slice images taller than this many pixels")
	sliceHeight := flag.Int("slice-height", defaults.TargetHeight, "Target slice height in pixels")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <image-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nRuns Tesseract OCR and outputs JSON file\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nLanguage codes:\n")
		fmt.Fprintf(os.Stderr, "  eng       English\n")
		fmt.Fprintf(os.Stderr, "  jpn       Japanese\n")
		fmt.Fprintf(os.Stderr, "  chi_sim   Simplified Chinese\n")
		fmt.Fprintf(os.Stderr, "  chi_tra   Traditional Chinese\n")
		fmt.Fprintf(os.Stderr, "  ara       Arabic\n")
		fmt.Fprintf(os.Stderr, "  hin       Hindi\n")
		fmt.Fprintf(os.Stderr, "  spa       Spanish\n")
		os.Exit(1)
	}

	imagePath := flag.Arg(0)

	// Check if tesseract is installed
	if _, err := exec.LookPath("tesseract"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: tesseract not found in PATH\n")
		fmt.Fprintf(os.Stderr, "Install with: brew install tesseract\n")
		os.Exit(1)
	}

	img, err := slicing.LoadImage(imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading image: %v\n", err)
		os.Exit(1)
	}

	cfg := slicing.Config{
		Enabled:         *sliceEnabled,
		HeightThreshold: *sliceThreshold,
		TargetHeight:    *sliceHeight,
		MinHeight:       *sliceHeight * 2 / 3,
	}
	slices, err := slicing.SliceImage(img, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error slicing image: %v\n", err)
		os.Exit(1)
	}

	if langFlag != "" {
		fmt.Printf("Running Tesseract OCR on %s (languages: %s)...\n", imagePath, langFlag)
	} else {
		fmt.Printf("Running Tesseract OCR on %s...\n", imagePath)
	}

	var words []tesseract.TesseractWord
	if len(slices) == 1 {
		// Unsliced: run on the original file directly (no re-encode).
		words, err = runTesseract(imagePath, langFlag, psmFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("  Sliced into %d strips for OCR\n", len(slices))
		words, err = ocrSlices(slices, langFlag, psmFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Create JSON document
	doc := tesseract.TesseractDocument{
		Words: words,
	}

	// Generate output filename
	ext := filepath.Ext(imagePath)
	baseName := strings.TrimSuffix(imagePath, ext)
	outputPath := baseName + "-ocr.json"

	// Write JSON file
	jsonData, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Created %s (%d words)\n", outputPath, len(words))
}

// runTesseract runs the tesseract CLI on one image file and parses its
// word-level TSV output.
func runTesseract(imagePath, langFlag, psmFlag string) ([]tesseract.TesseractWord, error) {
	tempTSV, err := os.CreateTemp("", "tesseract-*.tsv")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tempTSVPath := tempTSV.Name()
	tempTSV.Close()
	defer os.Remove(tempTSVPath)

	// Output to temp file base name (tesseract adds the .tsv extension)
	tempBase := strings.TrimSuffix(tempTSVPath, ".tsv")
	args := []string{imagePath, tempBase}
	if langFlag != "" {
		args = append(args, "-l", langFlag)
	}
	if psmFlag != "" {
		args = append(args, "--psm", psmFlag)
	}
	args = append(args, "tsv")

	cmd := exec.Command("tesseract", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("running tesseract: %w\n%s", err, string(output))
	}

	tsvData, err := os.ReadFile(tempTSVPath)
	if err != nil {
		return nil, fmt.Errorf("reading TSV file: %w", err)
	}

	words, err := parseTesseractTSV(string(tsvData))
	if err != nil {
		return nil, fmt.Errorf("parsing TSV: %w", err)
	}
	return words, nil
}

// ocrSlices runs tesseract on each slice and merges the words back into
// full-page pixel coordinates. Line numbers are re-based per slice so lines
// from different slices never collide.
func ocrSlices(slices []slicing.Slice, langFlag, psmFlag string) ([]tesseract.TesseractWord, error) {
	tempDir, err := os.MkdirTemp("", "tesseract-slices-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var all []tesseract.TesseractWord
	lineBase := 0
	for i, slice := range slices {
		slicePath := filepath.Join(tempDir, fmt.Sprintf("slice-%03d.png", i))
		f, err := os.Create(slicePath)
		if err != nil {
			return nil, fmt.Errorf("creating slice file: %w", err)
		}
		err = png.Encode(f, slice.Image)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("encoding slice %d: %w", i, err)
		}

		words, err := runTesseract(slicePath, langFlag, psmFlag)
		if err != nil {
			return nil, fmt.Errorf("OCR on slice %d: %w", i, err)
		}

		maxLine := 0
		for _, w := range words {
			if w.LineNum > maxLine {
				maxLine = w.LineNum
			}
			w.Top += slice.OffsetY
			w.LineNum += lineBase
			all = append(all, w)
		}
		lineBase += maxLine + 1
	}
	return all, nil
}

func parseTesseractTSV(tsvData string) ([]tesseract.TesseractWord, error) {
	reader := csv.NewReader(strings.NewReader(tsvData))
	reader.Comma = '\t'

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Build column index map
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[col] = i
	}

	// Parse rows
	var words []tesseract.TesseractWord

	for {
		row, err := reader.Read()
		if err != nil {
			break // EOF or error
		}

		// Level 5 = word level in Tesseract output
		if len(row) <= colMap["level"] {
			continue
		}

		level, _ := strconv.Atoi(row[colMap["level"]])
		if level != 5 {
			continue
		}

		// Get text
		text := ""
		if colMap["text"] < len(row) {
			text = strings.TrimSpace(row[colMap["text"]])
		}
		if text == "" {
			continue
		}

		// Parse coordinates
		left, _ := strconv.Atoi(row[colMap["left"]])
		top, _ := strconv.Atoi(row[colMap["top"]])
		width, _ := strconv.Atoi(row[colMap["width"]])
		height, _ := strconv.Atoi(row[colMap["height"]])
		conf, _ := strconv.ParseFloat(row[colMap["conf"]], 64)

		// Get line number (use block_num for line grouping)
		lineNum := 0
		if colMap["line_num"] < len(row) && row[colMap["line_num"]] != "" {
			lineNum, _ = strconv.Atoi(row[colMap["line_num"]])
		} else if colMap["block_num"] < len(row) {
			lineNum, _ = strconv.Atoi(row[colMap["block_num"]])
		}

		word := tesseract.TesseractWord{
			Text:       text,
			LineNum:    lineNum,
			Left:       left,
			Top:        top,
			Width:      width,
			Height:     height,
			Confidence: conf,
		}

		words = append(words, word)
	}

	return words, nil
}
