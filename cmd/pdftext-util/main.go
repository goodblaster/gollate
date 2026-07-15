package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/pdftext"
)

// outBlock is the engine-neutral blocks schema (see pkg/engines/blocks.go);
// index, page dimensions, etc. are filled in by the blocks engine on read.
type outBlock struct {
	Text   string          `json:"text"`
	Bounds ocr.BoundingBox `json:"bounds"`
	Conf   float64         `json:"normalized_conf"`
	Engine string          `json:"engine"`
	LineId string          `json:"line_id,omitempty"`
}

func main() {
	backendFlag := flag.String("backend", "auto",
		"Extraction backend: auto, pdfkit (macOS), poppler (requires pdftotext)")
	langFlag := flag.String("lang", "",
		"Document language hint for auto backend selection (e.g. hindi prefers poppler)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <pdf-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExtracts positioned words from a PDF's embedded text layer and saves\n")
		fmt.Fprintf(os.Stderr, "them in gollate's engine-neutral blocks JSON, for use with --engine\n")
		fmt.Fprintf(os.Stderr, "blocks. Not OCR: scanned PDFs without a text layer yield no words\n")
		fmt.Fprintf(os.Stderr, "(use ocr-util/tesseract-util on the rasterized image instead).\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nBackends (auto picks the first available):\n")
		for _, b := range pdftext.Backends() {
			status := "available"
			if !b.Available() {
				status = "not available"
			}
			fmt.Fprintf(os.Stderr, "  %-8s %s\n", b.Name(), status)
		}
		fmt.Fprintf(os.Stderr, "\nOutput file: {basename}-pdftext.json\n")
		fmt.Fprintf(os.Stderr, "Multi-page PDFs: {basename}-{page}-pdftext.json per page,\n")
		fmt.Fprintf(os.Stderr, "matching scripts/pdf-to-png.sh raster naming.\n")
		os.Exit(1)
	}

	pdfPath := flag.Arg(0)
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: PDF file not found: %s\n", pdfPath)
		os.Exit(1)
	}

	backend, err := pdftext.Select(*backendFlag, *langFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	pages, err := backend.Extract(pdfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting PDF text: %v\n", err)
		os.Exit(1)
	}
	if len(pages) == 0 {
		fmt.Fprintf(os.Stderr, "Error: PDF has no pages\n")
		os.Exit(1)
	}

	base := strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath))
	fmt.Printf("Processing: %s (%d page(s), backend: %s)\n", pdfPath, len(pages), backend.Name())

	emptyPages := 0
	for i, page := range pages {
		outputPath := base + "-pdftext.json"
		if len(pages) > 1 {
			outputPath = fmt.Sprintf("%s-%d-pdftext.json", base, i+1)
		}

		blocks := make([]outBlock, 0, len(page.Words))
		for _, b := range page.Blocks() {
			blocks = append(blocks, outBlock{
				Text:   b.Text,
				Bounds: b.BoundingBox,
				Conf:   b.Confidence,
				Engine: b.Extractor,
				LineId: b.LineId,
			})
		}

		jsonData, err := json.MarshalIndent(blocks, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Saved: %s\n", outputPath)
		fmt.Printf("  Page: %.0fx%.0f pt (pass --width %.0f --height %.0f)\n",
			page.Width, page.Height, page.Width, page.Height)
		fmt.Printf("  Words: %d\n", len(page.Words))
		if len(page.Words) == 0 {
			emptyPages++
		}
	}

	if emptyPages > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d page(s) have no text layer (scanned?). "+
			"Rasterize (scripts/pdf-to-png.sh) and OCR those pages instead.\n", emptyPages)
	}
}
