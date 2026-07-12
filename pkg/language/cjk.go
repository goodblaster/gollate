package language

// CJK implements Handler for Chinese, Japanese, and Korean languages.
type CJK struct{}

func (h *CJK) Name() string {
	return "CJK"
}

func (h *CJK) DetectScript(text string) float64 {
	if len(text) == 0 {
		return 0.0
	}

	// Count CJK characters
	cjkCount := 0
	totalRunes := 0

	for _, r := range text {
		totalRunes++
		if isCJKChar(r) {
			cjkCount++
		}
	}

	if totalRunes == 0 {
		return 0.0
	}

	return float64(cjkCount) / float64(totalRunes)
}

func (h *CJK) ReadingOrder() ReadingOrder {
	// Modern CJK typically uses horizontal left-to-right, top-to-bottom
	// (Traditional vertical text would need a different handler)
	return ReadingOrder{
		Primary:       Horizontal,
		HorizontalDir: LeftToRight,
		Secondary:     Vertical,
		VerticalDir:   TopToBottom,
	}
}

func (h *CJK) NeedsSpaceBetween(current, next string) bool {
	// No spaces between CJK characters
	// But add space if mixing with non-CJK (numbers, English)
	currentIsCJK := containsCJK(current)
	nextIsCJK := containsCJK(next)

	// Both are CJK - no space
	if currentIsCJK && nextIsCJK {
		return false
	}

	// At least one is not CJK - add space
	return true
}

func (h *CJK) OCRSettings() OCRSettings {
	return OCRSettings{
		LanguageCodes:     []string{"zh-Hant", "zh-Hans", "ja-JP", "ko-KR"},
		RecognitionLevel:  "accurate",
		RequiresCharSplit: true,
	}
}

func (h *CJK) Tokenize(text string) []string {
	// CJK text tokenizes into individual characters (runes)
	// Each character is treated as a separate "word" for pathfinding
	if text == "" {
		return nil
	}

	tokens := []string{}
	for _, r := range text {
		// Skip whitespace
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		tokens = append(tokens, string(r))
	}

	return tokens
}

// isCJKChar checks if a rune is a CJK character
func isCJKChar(r rune) bool {
	return (r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) // Katakana
}

// containsCJK checks if text contains any CJK characters
func containsCJK(text string) bool {
	for _, r := range text {
		if isCJKChar(r) {
			return true
		}
	}
	return false
}
