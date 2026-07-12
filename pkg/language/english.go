package language

// English implements Handler for English and other Latin-script languages.
type English struct{}

func (h *English) Name() string {
	return "English"
}

func (h *English) DetectScript(text string) float64 {
	if len(text) == 0 {
		return 0.0
	}

	// Count ASCII letters and common punctuation
	latinCount := 0
	totalRunes := 0

	for _, r := range text {
		totalRunes++
		// ASCII letters, numbers, and common punctuation
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == ' ' || r == '.' || r == ',' {
			latinCount++
		}
	}

	if totalRunes == 0 {
		return 0.0
	}

	return float64(latinCount) / float64(totalRunes)
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
