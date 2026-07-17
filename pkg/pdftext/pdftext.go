// Package pdftext extracts positioned words from a PDF's embedded text
// layer. A text layer is effectively a perfect OCR engine — words with
// exact coordinates and no misreads — so the output is meant to be fed
// to the sorter through the engine-neutral "blocks" format as an input
// block source.
//
// PDF text must never be used as canonical text: text layers carry
// positions but not the sentence/paragraph structure canonical text
// exists to provide — that missing structure is the problem gollate
// solves.
//
// Extraction is pluggable: the "pdfkit" backend uses Apple PDFKit
// (macOS only) and the "poppler" backend shells out to pdftotext.
// Neither wins everywhere — PDFKit mangles Devanagari, poppler's boxed
// output needs RTL repair and is weak on vertical CJK — so Select
// prefers per-script (see backend docs and CLAUDE.md measurements).
package pdftext

import (
	"slices"
	"strings"

	"github.com/goodblaster/errors"
	"github.com/goodblaster/gollate/pkg/ocr"
	"golang.org/x/text/unicode/norm"
)

// Extractor is the engine name stamped on emitted blocks.
const Extractor = "pdftext"

// Word is a single text-layer token with its bounding box as 0-1
// fractions of the page media box, top-left origin.
type Word struct {
	Text   string  `json:"text"`
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	// LineId identifies the extractor's own line grouping when the
	// backend provides one (poppler does; PDFKit does not). Feeds
	// ocr.Block.LineId, which line repair uses for Latin/Hindi.
	LineId string `json:"line_id,omitempty"`
}

// Page holds one PDF page's text-layer words plus the media box size in
// points (useful as --width/--height since block coordinates are
// fractions; only the aspect ratio matters to the sorter).
type Page struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Words  []Word  `json:"words"`
}

// Blocks converts the page's words to engine-neutral blocks suitable for
// the "blocks" engine (word granularity, 0-1 coordinates, emit order).
func (p Page) Blocks() []ocr.Block {
	blocks := make([]ocr.Block, 0, len(p.Words))
	for _, w := range p.Words {
		blocks = append(blocks, ocr.Block{
			Text: w.Text,
			BoundingBox: ocr.BoundingBox{
				Top:    w.Top,
				Left:   w.Left,
				Width:  w.Width,
				Height: w.Height,
			},
			Confidence: 1.0,
			Extractor:  Extractor,
			LineId:     w.LineId,
		})
	}
	return blocks
}

// Backend is one way of reading a PDF text layer.
type Backend interface {
	Name() string
	// Available reports whether the backend can run on this machine
	// (platform support, required binaries installed).
	Available() bool
	// Extract reads the text layer of every page. Pages without a text
	// layer (e.g. scanned documents) return zero words.
	Extract(path string) ([]Page, error)
}

// backends is the registry; backend files append via init(). Preference
// is decided by defaultOrder, not registration order.
var backends []Backend

// defaultOrder is the auto-selection preference: PDFKit wins or ties on
// every measured script except Devanagari (see CLAUDE.md), where Select
// flips to poppler via indicLanguages.
var defaultOrder = []string{"pdfkit", "poppler"}

// Backends returns all registered backends in default preference order.
func Backends() []Backend { return ordered(defaultOrder[0]) }

// indicLanguages are scripts PDFKit is known to mangle (dropped/reordered
// matras) but poppler round-trips correctly.
var indicLanguages = map[string]bool{
	"hindi": true,
}

// Select picks an extraction backend. An explicit name ("pdfkit",
// "poppler") is honored or errors if that backend is unavailable.
// Empty/"auto" picks the first available backend, preferring poppler
// for Indic language hints (lang may be empty) and PDFKit otherwise.
func Select(name, lang string) (Backend, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name != "" && name != "auto" {
		for _, b := range backends {
			if b.Name() != name {
				continue
			}
			if !b.Available() {
				return nil, errors.Newf("backend %q is not available on this machine%s", name, installHint(name))
			}
			return b, nil
		}
		return nil, errors.Newf("unknown backend %q (have: %s)", name, strings.Join(backendNames(), ", "))
	}

	first := defaultOrder[0]
	if indicLanguages[strings.ToLower(lang)] {
		first = "poppler"
	}
	for _, b := range ordered(first) {
		if b.Available() {
			return b, nil
		}
	}
	return nil, errors.Newf("no PDF text extraction backend available (have: %s); install poppler (pdftotext) or run on macOS",
		strings.Join(backendNames(), ", "))
}

// ExtractFile reads the PDF text layer using the default backend
// selection (equivalent to Select("auto", "")).
func ExtractFile(path string) ([]Page, error) {
	b, err := Select("auto", "")
	if err != nil {
		return nil, err
	}
	return b.Extract(path)
}

func backendNames() []string {
	names := make([]string, 0, len(backends))
	for _, b := range backends {
		names = append(names, b.Name())
	}
	return names
}

// ordered returns the registry sorted by defaultOrder, with first
// promoted to the front. Backends missing from defaultOrder (custom
// registrations) keep registration order at the end.
func ordered(first string) []Backend {
	rank := func(b Backend) int {
		if b.Name() == first {
			return -1
		}
		for i, n := range defaultOrder {
			if b.Name() == n {
				return i
			}
		}
		return len(defaultOrder)
	}
	out := slices.Clone(backends)
	slices.SortStableFunc(out, func(a, b Backend) int { return rank(a) - rank(b) })
	return out
}

func installHint(name string) string {
	switch name {
	case "poppler":
		return " — install poppler (e.g. `brew install poppler`)"
	case "pdfkit":
		return " — PDFKit requires macOS"
	}
	return ""
}

// foldText normalizes extracted token text. Text layers carry
// typographic glyphs OCR never emits — ligatures (ﬀ, ﬁ), full-width
// compatibility forms — which would fail exact matching against
// canonical text. NFKC folds them to their plain equivalents.
func foldText(pages []Page) {
	for p := range pages {
		for w := range pages[p].Words {
			pages[p].Words[w].Text = norm.NFKC.String(pages[p].Words[w].Text)
		}
	}
}
