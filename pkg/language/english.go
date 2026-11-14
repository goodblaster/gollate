package language

import "unicode"

// English implements Handler for English and other Latin-script languages.
type English struct{}

func (h *English) Name() string {
	return "English"
}

func (h *English) DetectScript(text string) float64 {
	if len(text) == 0 {
		return 0.0
	}

	// Share of the text's letters that are Latin script (accented letters
	// included). Digits, punctuation and whitespace are ignored - see
	// CJK.DetectScript.
	latinCount := 0
	letters := 0

	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.Is(unicode.Latin, r) {
			latinCount++
		}
	}

	if letters == 0 {
		return 0.0
	}

	return float64(latinCount) / float64(letters)
}

func (h *English) ReadingOrder() ReadingOrder {
	return ReadingOrder{
		Primary:       Horizontal,
		HorizontalDir: LeftToRight,
		Secondary:     Vertical,
		VerticalDir:   TopToBottom,
	}
}

func (h *English) NeedsSpaceBetween(current, next string) bool {
	// Always add space between English words
	return true
}

func (h *English) OCRSettings() OCRSettings {
	return OCRSettings{
		LanguageCodes:     []string{"en-US"},
		RecognitionLevel:  "fast",
		RequiresCharSplit: false,
	}
}

func (h *English) Tokenize(text string) []string {
	// English tokenizes on whitespace (space-separated words)
	if text == "" {
		return nil
	}

	// Use FieldsFunc to split on any whitespace
	tokens := []string{}
	start := -1

	for i, r := range text {
		if isWhitespace(r) {
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

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
