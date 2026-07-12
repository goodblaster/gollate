package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goodblaster/gollate/pkg/engines"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// ValidationError represents a request validation error.
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("invalid %s (value: %v): %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

// SortRequest contains the parameters needed to sort OCR blocks using canonical text.
//
// The sorting algorithm uses spatial proximity to reconstruct reading order by matching
// OCR-extracted text blocks to expected text lines.
type SortRequest struct {
	// Engine specifies which OCR engine produced the InputJson.
	// Built-in: "apple", "tesseract", "easyocr", and "blocks" (the
	// engine-neutral pre-normalized format). Custom engines added via
	// engines.Register are addressed by their registered name.
	Engine string `json:"engine"`

	// Lines contains the canonical text in expected reading order.
	// Each line should be a complete sentence or logical text unit.
	Lines []string `json:"lines"`

	// InputJson is the raw JSON output from the OCR engine.
	// Format varies by engine - see engine-specific documentation.
	InputJson string `json:"input_json"`

	// PageWidth is the image width in pixels.
	// Required for normalizing coordinates to 0-1 range.
	PageWidth int `json:"page_width"`

	// PageHeight is the image height in pixels.
	// Required for normalizing coordinates to 0-1 range.
	PageHeight int `json:"page_height"`

	// TraceParent is an optional distributed tracing header.
	TraceParent string `json:"trace_parent"`

	// Meta contains optional metadata to pass through to the response.
	Meta json.RawMessage `json:"meta"`

	// internal use
	blocks []ocr.Block
	logger logger.Logger
}

// WithLogger sets the logger for this request.
// If not set, logging will be disabled during parsing.
func (req *SortRequest) WithLogger(log logger.Logger) *SortRequest {
	req.logger = log
	return req
}

// Blocks returns the parsed OCR blocks.
// This is only available after calling Parse().
func (req *SortRequest) Blocks() []ocr.Block {
	return req.blocks
}

// Validate checks that the request has valid parameters before parsing.
// This allows catching errors early without performing the expensive parse operation.
func (req *SortRequest) Validate() error {
	if req.Engine == "" {
		return &ValidationError{Field: "engine", Message: "engine is required"}
	}

	if !engines.Supported(req.Engine) {
		return &ValidationError{
			Field:   "engine",
			Value:   req.Engine,
			Message: "unsupported engine (registered: " + strings.Join(engines.Names(), ", ") + ")",
		}
	}

	if req.PageWidth <= 0 {
		return &ValidationError{
			Field:   "page_width",
			Value:   req.PageWidth,
			Message: "must be greater than 0",
		}
	}

	if req.PageHeight <= 0 {
		return &ValidationError{
			Field:   "page_height",
			Value:   req.PageHeight,
			Message: "must be greater than 0",
		}
	}

	if len(req.Lines) == 0 {
		return &ValidationError{
			Field:   "lines",
			Message: "at least one canonical text line is required",
		}
	}

	if req.InputJson == "" {
		return &ValidationError{
			Field:   "input_json",
			Message: "OCR input JSON is required",
		}
	}

	return nil
}

// Parse validates the request and parses the OCR JSON into normalized blocks.
func (req *SortRequest) Parse() error {
	// Use default logger if none provided
	if req.logger == nil {
		req.logger = logger.Default()
	}

	// Validate first
	if err := req.Validate(); err != nil {
		return err
	}

	blocks, err := engines.Read(req.Engine, strings.NewReader(req.InputJson), req.PageWidth, req.PageHeight)
	if err != nil {
		return err
	}
	req.blocks = blocks
	return nil
}

// SortResponse contains the results of sorting OCR blocks.
type SortResponse struct {
	// Document is the primary output format (see sorters.Document): all
	// sorted text as one readable blob, plus paragraph/token layout that
	// references byte spans of it.
	Document *sorters.Document `json:"document,omitempty"`

	// Unhandled lists canonical text lines that couldn't be matched to any
	// OCR blocks (hidden elements, dynamic content, or OCR failures), one
	// readable line each, whitespace trimmed.
	Unhandled []string `json:"unhandled"`

	// Meta carries statistics and provenance about the sort.
	Meta *Meta `json:"meta,omitempty"`

	// SortedBlocks is the legacy flat block list, with empty blocks marking
	// line breaks. NewSortResponse does not populate it; it remains for
	// library consumers that assemble it explicitly.
	//
	// Deprecated: prefer Document.
	SortedBlocks []ocr.Block `json:"sorted_blocks,omitempty"`
}

// Meta is response metadata: what the sort did (Stats), what was sorted and
// where it came from (Source), and any caller-supplied pass-through (Extra).
type Meta struct {
	Stats  *Stats          `json:"stats,omitempty"`
	Source *Source         `json:"source,omitempty"`
	Extra  json.RawMessage `json:"extra,omitempty"`
}

// Stats summarizes the outcome of a sort - a success/failure snapshot.
type Stats struct {
	CanonicalLines  int   `json:"canonical_lines"`
	InputBlocks     int   `json:"input_blocks"`
	LinesFound      int   `json:"lines_found"`
	LinesUnhandled  int   `json:"lines_unhandled"`
	LinesSplit      int   `json:"lines_split"`
	LinesReconciled int   `json:"lines_reconciled"`
	LineRepairs     int   `json:"line_repairs"`
	HolesBridged    int   `json:"holes_bridged"`
	HolesFilled     int   `json:"holes_filled"`
	LeftoverBlocks  int   `json:"leftover_blocks"`
	Passes          int   `json:"passes"`
	ElapsedMs       int64 `json:"elapsed_ms"`
}

// Source describes what was sorted: the OCR engine, page geometry, how many
// slices the image was cut into for OCR, and the input files. Fields are
// populated by the caller (the CLI does this); the library never sees the
// original image.
type Source struct {
	Engine    string    `json:"engine,omitempty"`
	Language  string    `json:"language,omitempty"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	Slices    int       `json:"slices,omitempty"`
	ImageFile *FileInfo `json:"image_file,omitempty"`
	OCRFile   *FileInfo `json:"ocr_file,omitempty"`
	TextFile  *FileInfo `json:"text_file,omitempty"`
}

// FileInfo is a file's path and size in bytes.
type FileInfo struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// NewSortResponse assembles the standard response from a completed sort: the
// document, whitespace-trimmed unhandled lines, and sort statistics. Call
// after sorter.Sort(). Callers may then enrich Meta (Source, Extra).
func NewSortResponse(s *sorters.Sorter) *SortResponse {
	unhandled := s.UnhandledText()
	m := s.Metrics()
	return &SortResponse{
		Document:  s.Document(),
		Unhandled: unhandled,
		Meta: &Meta{
			Stats: &Stats{
				CanonicalLines:  s.LineCount(),
				InputBlocks:     len(s.InputBlocks()),
				LinesFound:      m.LinesFound,
				LinesUnhandled:  len(unhandled),
				LinesSplit:      m.LinesSplit,
				LinesReconciled: m.LinesReconciled,
				LineRepairs:     m.LineRepairs,
				HolesBridged:    m.HolesBridged,
				HolesFilled:     m.HolesFilled,
				LeftoverBlocks:  m.LeftoverBlocks,
				Passes:          m.PassesCompleted,
				ElapsedMs:       m.ElapsedTime.Milliseconds(),
			},
		},
	}
}
