package language

import "strings"

// ForLanguage returns the handler for a caller-supplied language name
// (the same names sorters.ConfigForLanguage accepts), falling back to
// script detection on the text for names it has no fixed handler for.
// A stated language is stronger evidence than a character count: Chinese
// or Japanese text stays per-character tokenized however many numerals
// and Latin terms it contains.
func ForLanguage(name string, texts ...string) Handler {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "chinese", "japanese":
		return &CJK{}
	}
	return Detect(texts...)
}

// Detect analyzes text and returns the most appropriate language handler.
// Returns a MixedContent handler if no single script dominates (>80% confidence).
func Detect(texts ...string) Handler {
	// Combine all text for analysis
	allText := strings.Join(texts, " ")

	if allText == "" {
		// Default to English for empty input
		return &English{}
	}

	// Try all handlers
	handlers := []Handler{
		&CJK{},
		&English{},
	}

	maxConfidence := 0.0
	var bestHandler Handler

	for _, h := range handlers {
		conf := h.DetectScript(allText)
		if conf > maxConfidence {
			maxConfidence = conf
			bestHandler = h
		}
	}

	// If no clear winner (confidence < 80%), use mixed content handler
	if maxConfidence < 0.8 {
		return &MixedContent{handlers: handlers}
	}

	return bestHandler
}

// MixedContent handles documents with multiple scripts (e.g., English + Chinese).
type MixedContent struct {
	handlers []Handler
}

func (h *MixedContent) Name() string {
	return "MixedContent"
}

func (h *MixedContent) DetectScript(text string) float64 {
	// Return highest confidence among sub-handlers
	maxConfidence := 0.0
	for _, handler := range h.handlers {
		if conf := handler.DetectScript(text); conf > maxConfidence {
			maxConfidence = conf
		}
	}
	return maxConfidence
}

func (h *MixedContent) ReadingOrder() ReadingOrder {
	// Default to horizontal LTR, top-to-bottom
	// Could be made smarter based on detected primary script
	return ReadingOrder{
		Primary:       Horizontal,
		HorizontalDir: LeftToRight,
		Secondary:     Vertical,
		VerticalDir:   TopToBottom,
	}
}

func (h *MixedContent) NeedsSpaceBetween(current, next string) bool {
	// Use heuristic: if both are CJK, no space; otherwise space
	currentIsCJK := containsCJK(current)
	nextIsCJK := containsCJK(next)

	if currentIsCJK && nextIsCJK {
		return false
	}

	return true
}

func (h *MixedContent) OCRSettings() OCRSettings {
	// Include all language codes, use accurate recognition
	return OCRSettings{
		LanguageCodes:     []string{"en-US", "zh-Hant", "zh-Hans", "ja-JP", "ko-KR"},
		RecognitionLevel:  "accurate",
		RequiresCharSplit: true, // Needed for CJK content
	}
}

func (h *MixedContent) Tokenize(text string) []string {
	// Mixed content tokenizes on whitespace, same as English and CJK
	// Spacing rules (NeedsSpaceBetween) handle the differences
	if text == "" {
		return nil
	}

	tokens := []string{}
	start := -1

	for i, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if start >= 0 {
				tokens = append(tokens, text[start:i])
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
	}

	// Add final token if exists
	if start >= 0 {
		tokens = append(tokens, text[start:])
	}

	return tokens
}
