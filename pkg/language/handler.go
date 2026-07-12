package language

// Handler encapsulates language-specific sorting and formatting rules.
// This allows the core algorithm to remain language-agnostic.
type Handler interface {
	// Name returns the handler name for debugging/logging
	Name() string

	// DetectScript returns confidence score (0.0-1.0) that text belongs to this script
	DetectScript(text string) float64

	// ReadingOrder defines how blocks should be spatially sorted
	ReadingOrder() ReadingOrder

	// NeedsSpaceBetween determines if space should be added between two blocks in output
	NeedsSpaceBetween(current, next string) bool

	// OCRSettings provides hints for OCR engines
	OCRSettings() OCRSettings

	// Tokenize splits text into matchable units (words for English, characters for CJK)
	Tokenize(text string) []string
}

// ReadingOrder defines primary and secondary reading directions
type ReadingOrder struct {
	Primary   Direction // Horizontal or Vertical
	Secondary Direction // Perpendicular to primary

	// For primary horizontal
	HorizontalDir HorizontalDirection // LeftToRight or RightToLeft

	// For primary vertical
	VerticalDir VerticalDirection // TopToBottom or BottomToTop
}

type Direction int

const (
	Horizontal Direction = iota
	Vertical
)

type HorizontalDirection int

const (
	LeftToRight HorizontalDirection = iota
	RightToLeft
)

type VerticalDirection int

const (
	TopToBottom VerticalDirection = iota
	BottomToTop
)

// OCRSettings provides language-specific hints for OCR engines
type OCRSettings struct {
	LanguageCodes     []string // e.g., ["en-US"], ["zh-Hans", "zh-Hant"]
	RecognitionLevel  string   // "fast" or "accurate"
	RequiresCharSplit bool     // Whether to tokenize into characters
}
