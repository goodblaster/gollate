package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/goodblaster/errors"
	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/engines"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/multipage"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// multipageResponse is the JSON output for a multi-page sort. It mirrors
// api.SortResponse, with the multi-page document in place of the
// single-page one.
type multipageResponse struct {
	Document  *multipage.Document `json:"document"`
	Unhandled []string            `json:"unhandled"`
	Meta      multipageMeta       `json:"meta"`
}

type multipageMeta struct {
	Stats  multipageStats `json:"stats"`
	Source api.Source     `json:"source"`
}

type multipageStats struct {
	Pages          int   `json:"pages"`
	CanonicalLines int   `json:"canonical_lines"`
	InputBlocks    int   `json:"input_blocks"`
	Paragraphs     int   `json:"paragraphs"`
	Straddles      int   `json:"straddles"`       // paragraphs joined across a page break
	FurnitureRuns  int   `json:"furniture_runs"`  // page numbers / running heads set aside
	LinesUnhandled int   `json:"lines_unhandled"` // canonical lines matched on no page
	ElapsedMs      int64 `json:"elapsed_ms"`
}

// runMultipage sorts the pages named by the repeated --ocr-file flag, in
// the order given, against the whole canonical text. --width and --height
// apply to every page.
func runMultipage(log logger.Logger) error {
	if *debugDir != "" || *imagePath != "" {
		fmt.Fprintln(os.Stderr, "Warning: --debug-dir and --image are not supported for multi-page documents; ignoring")
	}

	textData, err := os.ReadFile(*textFile)
	if err != nil {
		return errors.Wrap(err, "failed to read text file")
	}
	canonical := strings.Split(string(textData), "\n")

	var pages []multipage.Page
	blocks := 0
	for i, path := range ocrFiles {
		f, err := os.Open(path)
		if err != nil {
			return errors.Wrapf(err, "failed to open page %d", i+1)
		}
		pageBlocks, err := engines.Read(*engine, f, *width, *height)
		f.Close()
		if err != nil {
			return errors.Wrapf(err, "failed to parse page %d (%s)", i+1, path)
		}
		blocks += len(pageBlocks)
		pages = append(pages, multipage.Page{Blocks: pageBlocks, Width: float64(*width), Height: float64(*height)})
	}

	if !sorters.IsTunedLanguage(*langName) {
		fmt.Fprintf(os.Stderr, "Warning: no tuned configuration for language %q; using defaults (tuned: %s)\n",
			*langName, strings.Join(sorters.TunedLanguages(), ", "))
	}
	config := sorters.ConfigForLanguage(*langName)
	if *columnJumpPenalty > 0 {
		config.ColumnJumpPenalty = *columnJumpPenalty
	}

	start := time.Now()
	result, err := multipage.Sort(pages, canonical, config, log)
	if err != nil {
		return err
	}

	stats := multipageStats{
		Pages:          len(pages),
		CanonicalLines: len(canonical),
		InputBlocks:    blocks,
		Paragraphs:     len(result.Document.Paragraphs),
		LinesUnhandled: len(result.Unhandled),
		ElapsedMs:      time.Since(start).Milliseconds(),
	}
	for _, a := range result.Assignments {
		if a.Straddle {
			stats.Straddles++
		}
	}
	for _, page := range result.Document.Pages {
		stats.FurnitureRuns += len(page.Furniture)
	}

	var writer io.Writer = os.Stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			return errors.Wrap(err, "failed to create output file")
		}
		defer f.Close()
		writer = f
	}
	if *format == "text" {
		_, err := fmt.Fprintln(writer, result.Document.Text)
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(multipageResponse{
		Document:  result.Document,
		Unhandled: result.Unhandled,
		Meta: multipageMeta{
			Stats: stats,
			Source: api.Source{
				Engine:    *engine,
				Language:  *langName,
				Width:     *width,
				Height:    *height,
				ImageFile: fileInfo(*sourceFile),
				TextFile:  fileInfo(*textFile),
			},
		},
	})
}
