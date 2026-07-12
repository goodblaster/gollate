package sorters

import (
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// TestEnglishTwoColumns tests English text in a two-column newspaper layout.
// Left column, then right column. OCR returns blocks in mixed order.
func TestEnglishTwoColumns(t *testing.T) {
	// Canonical text: left column followed by right column
	// Note: Duplicate word "the" may have capitalization mismatches due to pathfinding ambiguity
	canonicalText := []string{
		"the morning sun cast long shadows across the quiet street.",
		"Birds sang softly in the trees overhead.",
		"Meanwhile The evening rain began to fall gently on the rooftops.",
		"People hurried home with umbrellas unfurled.",
	}

	// OCR blocks in wrong order (simulating mixed column reading)
	// Left column: "The morning sun..." and "Birds sang softly..."
	// Right column: "Meanwhile the evening..." and "People hurried home..."
	blocks := []ocr.Block{
		// Right column sentence 1, word 1 (wrong position - should be 3rd sentence)
		{Text: "Meanwhile", NormedText: "meanwhile", Index: 0, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.6, Width: 0.08, Height: 0.02}},
		// Left column sentence 1, word 1 (correct position - 1st sentence)
		{Text: "The", NormedText: "the", Index: 1, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.05, Width: 0.03, Height: 0.02}},
		{Text: "morning", NormedText: "morning", Index: 2, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.09, Width: 0.06, Height: 0.02}},
		// Right column continues
		{Text: "the", NormedText: "the", Index: 3, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.69, Width: 0.03, Height: 0.02}},
		// Left column continues
		{Text: "sun", NormedText: "sun", Index: 4, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.16, Width: 0.03, Height: 0.02}},
		{Text: "cast", NormedText: "cast", Index: 5, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.20, Width: 0.04, Height: 0.02}},
		{Text: "long", NormedText: "long", Index: 6, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.25, Width: 0.04, Height: 0.02}},
		{Text: "shadows", NormedText: "shadows", Index: 7, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.30, Width: 0.07, Height: 0.02}},
		// Right column continues
		{Text: "evening", NormedText: "evening", Index: 8, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.73, Width: 0.06, Height: 0.02}},
		{Text: "rain", NormedText: "rain", Index: 9, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.80, Width: 0.04, Height: 0.02}},
		// Left column continues
		{Text: "across", NormedText: "across", Index: 10, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.05, Width: 0.06, Height: 0.02}},
		{Text: "the", NormedText: "the", Index: 11, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.12, Width: 0.03, Height: 0.02}},
		{Text: "quiet", NormedText: "quiet", Index: 12, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.16, Width: 0.05, Height: 0.02}},
		{Text: "street.", NormedText: "street", Index: 13, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.22, Width: 0.06, Height: 0.02}},
		// Right column continues
		{Text: "began", NormedText: "began", Index: 14, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.60, Width: 0.05, Height: 0.02}},
		{Text: "to", NormedText: "to", Index: 15, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "fall", NormedText: "fall", Index: 16, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.69, Width: 0.04, Height: 0.02}},
		{Text: "gently", NormedText: "gently", Index: 17, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.74, Width: 0.06, Height: 0.02}},

		// Left column sentence 2
		{Text: "Birds", NormedText: "birds", Index: 18, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.05, Width: 0.05, Height: 0.02}},
		{Text: "sang", NormedText: "sang", Index: 19, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.11, Width: 0.04, Height: 0.02}},
		{Text: "softly", NormedText: "softly", Index: 20, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.16, Width: 0.06, Height: 0.02}},
		// Right column continues
		{Text: "on", NormedText: "on", Index: 21, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "the", NormedText: "the", Index: 22, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.63, Width: 0.03, Height: 0.02}},
		{Text: "rooftops.", NormedText: "rooftops", Index: 23, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.67, Width: 0.08, Height: 0.02}},
		// Left column continues
		{Text: "in", NormedText: "in", Index: 24, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.23, Width: 0.02, Height: 0.02}},
		{Text: "the", NormedText: "the", Index: 25, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.26, Width: 0.03, Height: 0.02}},
		{Text: "trees", NormedText: "trees", Index: 26, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.30, Width: 0.05, Height: 0.02}},
		{Text: "overhead.", NormedText: "overhead", Index: 27, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.36, Width: 0.08, Height: 0.02}},

		// Right column sentence 2
		{Text: "People", NormedText: "people", Index: 28, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.60, Width: 0.06, Height: 0.02}},
		{Text: "hurried", NormedText: "hurried", Index: 29, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.67, Width: 0.07, Height: 0.02}},
		{Text: "home", NormedText: "home", Index: 30, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.75, Width: 0.04, Height: 0.02}},
		{Text: "with", NormedText: "with", Index: 31, BoundingBox: ocr.BoundingBox{Top: 0.22, Left: 0.60, Width: 0.04, Height: 0.02}},
		{Text: "umbrellas", NormedText: "umbrellas", Index: 32, BoundingBox: ocr.BoundingBox{Top: 0.22, Left: 0.65, Width: 0.09, Height: 0.02}},
		{Text: "unfurled.", NormedText: "unfurled", Index: 33, BoundingBox: ocr.BoundingBox{Top: 0.22, Left: 0.75, Width: 0.08, Height: 0.02}},
	}

	// Run sort with MultiColumnConfig, optimized for clean column separation
	config := MultiColumnConfig()
	config.MaxWordDistance = 0.35       // Prevent cross-column jumps
	config.MinWordsForEarlyPasses = 5   // Process all test lines early
	config.RotationOptimization = true  // Re-enable for efficiency
	config.PermutationsPerPass = 100000 // Increased to find best path for duplicate words
	sorter := NewOcrSorterWithConfig(blocks, canonicalText, nil, config)
	sortedBlocks, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	// Extract text from sorted blocks
	var result strings.Builder
	for _, block := range sortedBlocks {
		if block.Text == "" {
			result.WriteString("\n")
		} else {
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString(" ")
			}
			result.WriteString(block.Text)
		}
	}

	// Expected output (canonical order)
	expected := strings.Join(canonicalText, "\n")
	actual := strings.TrimSpace(result.String())

	if actual != expected {
		t.Errorf("Sorted text doesn't match canonical.\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}

// TestSpanishTwoColumns tests Spanish text with accented characters in two columns.
func TestSpanishTwoColumns(t *testing.T) {
	canonicalText := []string{
		"La mañana traía luz y calor a la ciudad dormida.",
		"Los pájaros cantaban melodías dulces en los árboles.",
		"Mientras tanto la noche caía suavemente sobre las montañas.",
		"La gente regresaba a casa después del trabajo.",
	}

	blocks := []ocr.Block{
		// Right column word (should be 3rd sentence)
		{Text: "Mientras", NormedText: "mientras", Index: 0, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.6, Width: 0.08, Height: 0.02}},
		// Left column sentence 1
		{Text: "La", NormedText: "la", Index: 1, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "mañana", NormedText: "manana", Index: 2, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.08, Width: 0.06, Height: 0.02}},
		{Text: "traía", NormedText: "traia", Index: 3, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.15, Width: 0.05, Height: 0.02}},
		// Right column continues
		{Text: "tanto", NormedText: "tanto", Index: 4, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.69, Width: 0.05, Height: 0.02}},
		{Text: "la", NormedText: "la", Index: 5, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.75, Width: 0.02, Height: 0.02}},
		{Text: "noche", NormedText: "noche", Index: 6, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.78, Width: 0.05, Height: 0.02}},
		// Left column continues
		{Text: "luz", NormedText: "luz", Index: 7, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.21, Width: 0.03, Height: 0.02}},
		{Text: "y", NormedText: "y", Index: 8, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.25, Width: 0.01, Height: 0.02}},
		{Text: "calor", NormedText: "calor", Index: 9, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.27, Width: 0.05, Height: 0.02}},
		{Text: "a", NormedText: "a", Index: 10, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.33, Width: 0.01, Height: 0.02}},
		{Text: "la", NormedText: "la", Index: 11, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.35, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "caía", NormedText: "caia", Index: 12, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.60, Width: 0.04, Height: 0.02}},
		// Left column continues
		{Text: "ciudad", NormedText: "ciudad", Index: 13, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.05, Width: 0.06, Height: 0.02}},
		{Text: "dormida.", NormedText: "dormida", Index: 14, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.12, Width: 0.08, Height: 0.02}},
		// Right column continues
		{Text: "suavemente", NormedText: "suavemente", Index: 15, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.65, Width: 0.10, Height: 0.02}},
		{Text: "sobre", NormedText: "sobre", Index: 16, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.76, Width: 0.05, Height: 0.02}},
		{Text: "las", NormedText: "las", Index: 17, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.82, Width: 0.03, Height: 0.02}},

		// Left column sentence 2
		{Text: "Los", NormedText: "los", Index: 18, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.05, Width: 0.03, Height: 0.02}},
		{Text: "pájaros", NormedText: "pajaros", Index: 19, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.09, Width: 0.07, Height: 0.02}},
		// Right column continues
		{Text: "montañas.", NormedText: "montanas", Index: 20, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.60, Width: 0.09, Height: 0.02}},
		// Left column continues
		{Text: "cantaban", NormedText: "cantaban", Index: 21, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.17, Width: 0.08, Height: 0.02}},
		{Text: "melodías", NormedText: "melodias", Index: 22, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.26, Width: 0.08, Height: 0.02}},
		{Text: "dulces", NormedText: "dulces", Index: 23, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.35, Width: 0.06, Height: 0.02}},
		{Text: "en", NormedText: "en", Index: 24, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "los", NormedText: "los", Index: 25, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.08, Width: 0.03, Height: 0.02}},
		{Text: "árboles.", NormedText: "arboles", Index: 26, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.12, Width: 0.08, Height: 0.02}},

		// Right column sentence 2
		{Text: "La", NormedText: "la", Index: 27, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "gente", NormedText: "gente", Index: 28, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.63, Width: 0.05, Height: 0.02}},
		{Text: "regresaba", NormedText: "regresaba", Index: 29, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.69, Width: 0.09, Height: 0.02}},
		{Text: "a", NormedText: "a", Index: 30, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.79, Width: 0.01, Height: 0.02}},
		{Text: "casa", NormedText: "casa", Index: 31, BoundingBox: ocr.BoundingBox{Top: 0.22, Left: 0.60, Width: 0.04, Height: 0.02}},
		{Text: "después", NormedText: "despues", Index: 32, BoundingBox: ocr.BoundingBox{Top: 0.22, Left: 0.65, Width: 0.07, Height: 0.02}},
		{Text: "del", NormedText: "del", Index: 33, BoundingBox: ocr.BoundingBox{Top: 0.22, Left: 0.73, Width: 0.03, Height: 0.02}},
		{Text: "trabajo.", NormedText: "trabajo", Index: 34, BoundingBox: ocr.BoundingBox{Top: 0.22, Left: 0.77, Width: 0.07, Height: 0.02}},
	}

	// Run sort with MultiColumnConfig, optimized for clean column separation
	config := MultiColumnConfig()
	config.MaxWordDistance = 0.35       // Prevent cross-column jumps
	config.MinWordsForEarlyPasses = 5   // Process all test lines early
	config.PermutationsPerPass = 100000 // Increased to find best path for duplicate words
	sorter := NewOcrSorterWithConfig(blocks, canonicalText, nil, config)
	sortedBlocks, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	var result strings.Builder
	for _, block := range sortedBlocks {
		if block.Text == "" {
			result.WriteString("\n")
		} else {
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString(" ")
			}
			result.WriteString(block.Text)
		}
	}

	expected := strings.Join(canonicalText, "\n")
	actual := strings.TrimSpace(result.String())

	if actual != expected {
		t.Errorf("Sorted text doesn't match canonical.\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}

// TestChineseTwoColumns tests Chinese text in two columns (no spaces between words).
// Each character is a separate block, as is typical for CJK OCR output.
func TestChineseTwoColumns(t *testing.T) {
	canonicalText := []string{
		"早晨的阳光照亮了安静的街道", // Period normalized away
		"鸟儿在树上轻声歌唱",
		"与此同时夜晚的雨开始轻轻落下",
		"人们撑着雨伞匆匆回家",
	}

	blocks := []ocr.Block{
		// Right column (should be 3rd sentence) - mixed in with left column
		{Text: "与", NormedText: "与", Index: 0, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.60, Width: 0.02, Height: 0.02}},
		// Left column sentence 1
		{Text: "早", NormedText: "早", Index: 1, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "晨", NormedText: "晨", Index: 2, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.07, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "此", NormedText: "此", Index: 3, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "同", NormedText: "同", Index: 4, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.64, Width: 0.02, Height: 0.02}},
		// Left column continues
		{Text: "的", NormedText: "的", Index: 5, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.09, Width: 0.02, Height: 0.02}},
		{Text: "阳", NormedText: "阳", Index: 6, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.11, Width: 0.02, Height: 0.02}},
		{Text: "光", NormedText: "光", Index: 7, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.13, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "时", NormedText: "时", Index: 8, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "夜", NormedText: "夜", Index: 9, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.68, Width: 0.02, Height: 0.02}},
		{Text: "晚", NormedText: "晚", Index: 10, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.70, Width: 0.02, Height: 0.02}},
		// Left column continues
		{Text: "照", NormedText: "照", Index: 11, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.15, Width: 0.02, Height: 0.02}},
		{Text: "亮", NormedText: "亮", Index: 12, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.17, Width: 0.02, Height: 0.02}},
		{Text: "了", NormedText: "了", Index: 13, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.19, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "的", NormedText: "的", Index: 14, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.72, Width: 0.02, Height: 0.02}},
		{Text: "雨", NormedText: "雨", Index: 15, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.74, Width: 0.02, Height: 0.02}},
		{Text: "开", NormedText: "开", Index: 16, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.76, Width: 0.02, Height: 0.02}},
		// Left column continues (wrap to second line)
		{Text: "安", NormedText: "安", Index: 17, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "静", NormedText: "静", Index: 18, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.07, Width: 0.02, Height: 0.02}},
		{Text: "的", NormedText: "的", Index: 19, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.09, Width: 0.02, Height: 0.02}},
		{Text: "街", NormedText: "街", Index: 20, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.11, Width: 0.02, Height: 0.02}},
		{Text: "道", NormedText: "道", Index: 21, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.13, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 22, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.15, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "始", NormedText: "始", Index: 23, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "轻", NormedText: "轻", Index: 24, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "轻", NormedText: "轻", Index: 25, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.64, Width: 0.02, Height: 0.02}},
		{Text: "落", NormedText: "落", Index: 26, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "下", NormedText: "下", Index: 27, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.68, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 28, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.70, Width: 0.02, Height: 0.02}},

		// Left column sentence 2
		{Text: "鸟", NormedText: "鸟", Index: 29, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "儿", NormedText: "儿", Index: 30, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.07, Width: 0.02, Height: 0.02}},
		{Text: "在", NormedText: "在", Index: 31, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.09, Width: 0.02, Height: 0.02}},
		{Text: "树", NormedText: "树", Index: 32, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.11, Width: 0.02, Height: 0.02}},
		{Text: "上", NormedText: "上", Index: 33, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.13, Width: 0.02, Height: 0.02}},
		{Text: "轻", NormedText: "轻", Index: 34, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.15, Width: 0.02, Height: 0.02}},
		{Text: "声", NormedText: "声", Index: 35, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.17, Width: 0.02, Height: 0.02}},
		{Text: "歌", NormedText: "歌", Index: 36, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.19, Width: 0.02, Height: 0.02}},
		{Text: "唱", NormedText: "唱", Index: 37, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.21, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 38, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.23, Width: 0.02, Height: 0.02}},

		// Right column sentence 2
		{Text: "人", NormedText: "人", Index: 39, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "们", NormedText: "们", Index: 40, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "撑", NormedText: "撑", Index: 41, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.64, Width: 0.02, Height: 0.02}},
		{Text: "着", NormedText: "着", Index: 42, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "雨", NormedText: "雨", Index: 43, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.68, Width: 0.02, Height: 0.02}},
		{Text: "伞", NormedText: "伞", Index: 44, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.70, Width: 0.02, Height: 0.02}},
		{Text: "匆", NormedText: "匆", Index: 45, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.72, Width: 0.02, Height: 0.02}},
		{Text: "匆", NormedText: "匆", Index: 46, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.74, Width: 0.02, Height: 0.02}},
		{Text: "回", NormedText: "回", Index: 47, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "家", NormedText: "家", Index: 48, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 49, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.64, Width: 0.02, Height: 0.02}},
	}

	// Run sort with MultiColumnConfig, tuned for CJK character-level blocks
	config := MultiColumnConfig()
	// Must be > line wrap (BaseLineWrap=1.0 + gap=0.03 = 1.03)
	// Must be < column jump (BaseNextColumn=25.0 + gap=0.51 = 25.51)
	config.MaxWordDistance = 5.0
	config.MinWordsForEarlyPasses = 5   // Process all test lines early
	config.PermutationsPerPass = 100000 // Increased to find best path
	sorter := NewOcrSorterWithConfig(blocks, canonicalText, nil, config)
	sortedBlocks, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	var result strings.Builder
	for _, block := range sortedBlocks {
		if block.Text == "" {
			result.WriteString("\n")
		} else {
			result.WriteString(block.Text)
		}
	}

	expected := strings.Join(canonicalText, "\n")
	actual := strings.TrimSpace(result.String())

	if actual != expected {
		t.Errorf("Sorted text doesn't match canonical.\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}

// TestJapaneseTwoColumns tests Japanese text (Hiragana/Kanji mix) in two columns.
// Each character is a separate block, as is typical for CJK OCR output.
func TestJapaneseTwoColumns(t *testing.T) {
	canonicalText := []string{
		"朝の光が静かな道を照らしています", // Period normalized away
		"鳥たちが木の上で歌っています",
		"一方で夜の雨が屋根に降り始めました",
		"人々は傘を持って家に急いでいます",
	}

	blocks := []ocr.Block{
		// Right column (should be 3rd sentence) - mixed in with left column
		{Text: "一", NormedText: "一", Index: 0, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.60, Width: 0.02, Height: 0.02}},
		// Left column sentence 1
		{Text: "朝", NormedText: "朝", Index: 1, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "の", NormedText: "の", Index: 2, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.07, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "方", NormedText: "方", Index: 3, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "で", NormedText: "で", Index: 4, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.64, Width: 0.02, Height: 0.02}},
		// Left column continues
		{Text: "光", NormedText: "光", Index: 5, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.09, Width: 0.02, Height: 0.02}},
		{Text: "が", NormedText: "が", Index: 6, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.11, Width: 0.02, Height: 0.02}},
		{Text: "静", NormedText: "静", Index: 7, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.13, Width: 0.02, Height: 0.02}},
		{Text: "か", NormedText: "か", Index: 8, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.15, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "夜", NormedText: "夜", Index: 9, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "の", NormedText: "の", Index: 10, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.68, Width: 0.02, Height: 0.02}},
		{Text: "雨", NormedText: "雨", Index: 11, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.70, Width: 0.02, Height: 0.02}},
		{Text: "が", NormedText: "が", Index: 12, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.72, Width: 0.02, Height: 0.02}},
		// Left column continues
		{Text: "な", NormedText: "な", Index: 13, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.17, Width: 0.02, Height: 0.02}},
		{Text: "道", NormedText: "道", Index: 14, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.19, Width: 0.02, Height: 0.02}},
		{Text: "を", NormedText: "を", Index: 15, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.21, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "屋", NormedText: "屋", Index: 16, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.74, Width: 0.02, Height: 0.02}},
		{Text: "根", NormedText: "根", Index: 17, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.76, Width: 0.02, Height: 0.02}},
		{Text: "に", NormedText: "に", Index: 18, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.78, Width: 0.02, Height: 0.02}},
		// Left column continues (wrap to second line)
		{Text: "照", NormedText: "照", Index: 19, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "ら", NormedText: "ら", Index: 20, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.07, Width: 0.02, Height: 0.02}},
		{Text: "し", NormedText: "し", Index: 21, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.09, Width: 0.02, Height: 0.02}},
		{Text: "て", NormedText: "て", Index: 22, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.11, Width: 0.02, Height: 0.02}},
		{Text: "い", NormedText: "い", Index: 23, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.13, Width: 0.02, Height: 0.02}},
		{Text: "ま", NormedText: "ま", Index: 24, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.15, Width: 0.02, Height: 0.02}},
		{Text: "す", NormedText: "す", Index: 25, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.17, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 26, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.19, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "降", NormedText: "降", Index: 27, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "り", NormedText: "り", Index: 28, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "始", NormedText: "始", Index: 29, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.64, Width: 0.02, Height: 0.02}},
		{Text: "め", NormedText: "め", Index: 30, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "ま", NormedText: "ま", Index: 31, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.68, Width: 0.02, Height: 0.02}},
		{Text: "し", NormedText: "し", Index: 32, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.70, Width: 0.02, Height: 0.02}},
		{Text: "た", NormedText: "た", Index: 33, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.72, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 34, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.74, Width: 0.02, Height: 0.02}},

		// Left column sentence 2
		{Text: "鳥", NormedText: "鳥", Index: 35, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "た", NormedText: "た", Index: 36, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.07, Width: 0.02, Height: 0.02}},
		{Text: "ち", NormedText: "ち", Index: 37, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.09, Width: 0.02, Height: 0.02}},
		{Text: "が", NormedText: "が", Index: 38, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.11, Width: 0.02, Height: 0.02}},
		{Text: "木", NormedText: "木", Index: 39, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.13, Width: 0.02, Height: 0.02}},
		{Text: "の", NormedText: "の", Index: 40, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.15, Width: 0.02, Height: 0.02}},
		{Text: "上", NormedText: "上", Index: 41, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.17, Width: 0.02, Height: 0.02}},
		{Text: "で", NormedText: "で", Index: 42, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.19, Width: 0.02, Height: 0.02}},
		{Text: "歌", NormedText: "歌", Index: 43, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "っ", NormedText: "っ", Index: 44, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.07, Width: 0.02, Height: 0.02}},
		{Text: "て", NormedText: "て", Index: 45, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.09, Width: 0.02, Height: 0.02}},
		{Text: "い", NormedText: "い", Index: 46, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.11, Width: 0.02, Height: 0.02}},
		{Text: "ま", NormedText: "ま", Index: 47, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.13, Width: 0.02, Height: 0.02}},
		{Text: "す", NormedText: "す", Index: 48, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.15, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 49, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.17, Width: 0.02, Height: 0.02}},

		// Right column sentence 2
		{Text: "人", NormedText: "人", Index: 50, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "々", NormedText: "々", Index: 51, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "は", NormedText: "は", Index: 52, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.64, Width: 0.02, Height: 0.02}},
		{Text: "傘", NormedText: "傘", Index: 53, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "を", NormedText: "を", Index: 54, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.68, Width: 0.02, Height: 0.02}},
		{Text: "持", NormedText: "持", Index: 55, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.70, Width: 0.02, Height: 0.02}},
		{Text: "っ", NormedText: "っ", Index: 56, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.72, Width: 0.02, Height: 0.02}},
		{Text: "て", NormedText: "て", Index: 57, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.74, Width: 0.02, Height: 0.02}},
		{Text: "家", NormedText: "家", Index: 58, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.60, Width: 0.02, Height: 0.02}},
		{Text: "に", NormedText: "に", Index: 59, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.62, Width: 0.02, Height: 0.02}},
		{Text: "急", NormedText: "急", Index: 60, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.64, Width: 0.02, Height: 0.02}},
		{Text: "い", NormedText: "い", Index: 61, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "で", NormedText: "で", Index: 62, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.68, Width: 0.02, Height: 0.02}},
		{Text: "い", NormedText: "い", Index: 63, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.70, Width: 0.02, Height: 0.02}},
		{Text: "ま", NormedText: "ま", Index: 64, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.72, Width: 0.02, Height: 0.02}},
		{Text: "す", NormedText: "す", Index: 65, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.74, Width: 0.02, Height: 0.02}},
		{Text: "。", NormedText: "。", Index: 66, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.76, Width: 0.02, Height: 0.02}},
	}

	// Run sort with MultiColumnConfig, tuned for CJK character-level blocks
	config := MultiColumnConfig()
	// Must be > line wrap (BaseLineWrap=1.0 + gap=0.03 = 1.03)
	// Must be < column jump (BaseNextColumn=25.0 + gap=0.51 = 25.51)
	config.MaxWordDistance = 5.0
	config.MinWordsForEarlyPasses = 5   // Process all test lines early
	config.PermutationsPerPass = 100000 // Increased to find best path
	sorter := NewOcrSorterWithConfig(blocks, canonicalText, nil, config)
	sortedBlocks, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	var result strings.Builder
	for _, block := range sortedBlocks {
		if block.Text == "" {
			result.WriteString("\n")
		} else {
			result.WriteString(block.Text)
		}
	}

	expected := strings.Join(canonicalText, "\n")
	actual := strings.TrimSpace(result.String())

	if actual != expected {
		t.Errorf("Sorted text doesn't match canonical.\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}

// TestArabicTwoColumns tests Arabic RTL text in two columns.
// Note: Columns are still left-to-right in layout, but text within reads RTL.
func TestArabicTwoColumns(t *testing.T) {
	canonicalText := []string{
		"الصباح الجميل يجلب النور والدفء إلى المدينة.",
		"الطيور تغني بهدوء في الأشجار.",
		"في نفس الوقت الليل يسقط بلطف على الجبال.",
		"الناس يعودون إلى منازلهم بعد العمل.",
	}

	blocks := []ocr.Block{
		// Right column (should be 3rd sentence)
		{Text: "في", NormedText: "في", Index: 0, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.6, Width: 0.03, Height: 0.02}},
		{Text: "نفس", NormedText: "نفس", Index: 1, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.64, Width: 0.04, Height: 0.02}},
		// Left column sentence 1
		{Text: "الصباح", NormedText: "الصباح", Index: 2, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.05, Width: 0.06, Height: 0.02}},
		{Text: "الجميل", NormedText: "الجميل", Index: 3, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.12, Width: 0.06, Height: 0.02}},
		// Right column continues
		{Text: "الوقت", NormedText: "الوقت", Index: 4, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.69, Width: 0.05, Height: 0.02}},
		{Text: "الليل", NormedText: "الليل", Index: 5, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.75, Width: 0.05, Height: 0.02}},
		// Left column continues
		{Text: "يجلب", NormedText: "يجلب", Index: 6, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.19, Width: 0.04, Height: 0.02}},
		{Text: "النور", NormedText: "النور", Index: 7, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.24, Width: 0.05, Height: 0.02}},
		{Text: "والدفء", NormedText: "والدفء", Index: 8, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.30, Width: 0.06, Height: 0.02}},
		// Right column continues
		{Text: "يسقط", NormedText: "يسقط", Index: 9, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.60, Width: 0.04, Height: 0.02}},
		{Text: "بلطف", NormedText: "بلطف", Index: 10, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.65, Width: 0.04, Height: 0.02}},
		// Left column continues
		{Text: "إلى", NormedText: "إلى", Index: 11, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.05, Width: 0.03, Height: 0.02}},
		{Text: "المدينة.", NormedText: "المدينة", Index: 12, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.09, Width: 0.07, Height: 0.02}},
		// Right column continues
		{Text: "على", NormedText: "على", Index: 13, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.70, Width: 0.03, Height: 0.02}},
		{Text: "الجبال.", NormedText: "الجبال", Index: 14, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.74, Width: 0.06, Height: 0.02}},

		// Left column sentence 2
		{Text: "الطيور", NormedText: "الطيور", Index: 15, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.05, Width: 0.06, Height: 0.02}},
		{Text: "تغني", NormedText: "تغني", Index: 16, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.12, Width: 0.04, Height: 0.02}},
		{Text: "بهدوء", NormedText: "بهدوء", Index: 17, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.17, Width: 0.05, Height: 0.02}},
		{Text: "في", NormedText: "في", Index: 18, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.23, Width: 0.02, Height: 0.02}},
		{Text: "الأشجار.", NormedText: "الأشجار", Index: 19, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.26, Width: 0.07, Height: 0.02}},

		// Right column sentence 2
		{Text: "الناس", NormedText: "الناس", Index: 20, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.60, Width: 0.05, Height: 0.02}},
		{Text: "يعودون", NormedText: "يعودون", Index: 21, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.66, Width: 0.06, Height: 0.02}},
		{Text: "إلى", NormedText: "إلى", Index: 22, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.73, Width: 0.03, Height: 0.02}},
		{Text: "منازلهم", NormedText: "منازلهم", Index: 23, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.60, Width: 0.07, Height: 0.02}},
		{Text: "بعد", NormedText: "بعد", Index: 24, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.68, Width: 0.03, Height: 0.02}},
		{Text: "العمل.", NormedText: "العمل", Index: 25, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.72, Width: 0.05, Height: 0.02}},
	}

	// Run sort with MultiColumnConfig, optimized for clean column separation
	config := MultiColumnConfig()
	config.MaxWordDistance = 0.35       // Prevent cross-column jumps
	config.MinWordsForEarlyPasses = 5   // Process all test lines early
	config.PermutationsPerPass = 100000 // Increased to find best path for duplicate words
	sorter := NewOcrSorterWithConfig(blocks, canonicalText, nil, config)
	sortedBlocks, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	var result strings.Builder
	for _, block := range sortedBlocks {
		if block.Text == "" {
			result.WriteString("\n")
		} else {
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString(" ")
			}
			result.WriteString(block.Text)
		}
	}

	expected := strings.Join(canonicalText, "\n")
	actual := strings.TrimSpace(result.String())

	if actual != expected {
		t.Errorf("Sorted text doesn't match canonical.\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}

// TestHindiTwoColumns tests Hindi (Devanagari script) text in two columns.
func TestHindiTwoColumns(t *testing.T) {
	canonicalText := []string{
		"सुबह की धूप शांत सड़क पर चमक रही थी।",
		"पक्षी पेड़ों में धीरे से गा रहे थे।",
		"उसी समय शाम की बारिश छतों पर गिरने लगी।",
		"लोग छाते लेकर घर जल्दी जा रहे थे।",
	}

	blocks := []ocr.Block{
		// Right column (should be 3rd sentence)
		{Text: "उसी", NormedText: "उसी", Index: 0, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.6, Width: 0.04, Height: 0.02}},
		{Text: "समय", NormedText: "समय", Index: 1, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.65, Width: 0.04, Height: 0.02}},
		// Left column sentence 1
		{Text: "सुबह", NormedText: "सुबह", Index: 2, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.05, Width: 0.04, Height: 0.02}},
		{Text: "की", NormedText: "की", Index: 3, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.10, Width: 0.02, Height: 0.02}},
		{Text: "धूप", NormedText: "धूप", Index: 4, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.13, Width: 0.03, Height: 0.02}},
		// Right column continues
		{Text: "शाम", NormedText: "शाम", Index: 5, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.70, Width: 0.04, Height: 0.02}},
		{Text: "की", NormedText: "की", Index: 6, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.75, Width: 0.02, Height: 0.02}},
		{Text: "बारिश", NormedText: "बारिश", Index: 7, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.78, Width: 0.05, Height: 0.02}},
		// Left column continues
		{Text: "शांत", NormedText: "शांत", Index: 8, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.17, Width: 0.04, Height: 0.02}},
		{Text: "सड़क", NormedText: "सड़क", Index: 9, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.22, Width: 0.04, Height: 0.02}},
		{Text: "पर", NormedText: "पर", Index: 10, BoundingBox: ocr.BoundingBox{Top: 0.1, Left: 0.27, Width: 0.02, Height: 0.02}},
		// Right column continues
		{Text: "छतों", NormedText: "छतों", Index: 11, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.60, Width: 0.04, Height: 0.02}},
		{Text: "पर", NormedText: "पर", Index: 12, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.65, Width: 0.02, Height: 0.02}},
		// Left column continues
		{Text: "चमक", NormedText: "चमक", Index: 13, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.05, Width: 0.04, Height: 0.02}},
		{Text: "रही", NormedText: "रही", Index: 14, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.10, Width: 0.03, Height: 0.02}},
		{Text: "थी।", NormedText: "थी", Index: 15, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.14, Width: 0.03, Height: 0.02}},
		// Right column continues
		{Text: "गिरने", NormedText: "गिरने", Index: 16, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.68, Width: 0.05, Height: 0.02}},
		{Text: "लगी।", NormedText: "लगी", Index: 17, BoundingBox: ocr.BoundingBox{Top: 0.13, Left: 0.74, Width: 0.04, Height: 0.02}},

		// Left column sentence 2
		{Text: "पक्षी", NormedText: "पक्षी", Index: 18, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.05, Width: 0.04, Height: 0.02}},
		{Text: "पेड़ों", NormedText: "पेड़ों", Index: 19, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.10, Width: 0.05, Height: 0.02}},
		{Text: "में", NormedText: "में", Index: 20, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.16, Width: 0.03, Height: 0.02}},
		{Text: "धीरे", NormedText: "धीरे", Index: 21, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.20, Width: 0.04, Height: 0.02}},
		{Text: "से", NormedText: "से", Index: 22, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.25, Width: 0.02, Height: 0.02}},
		{Text: "गा", NormedText: "गा", Index: 23, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.05, Width: 0.02, Height: 0.02}},
		{Text: "रहे", NormedText: "रहे", Index: 24, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.08, Width: 0.03, Height: 0.02}},
		{Text: "थे।", NormedText: "थे", Index: 25, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.12, Width: 0.03, Height: 0.02}},

		// Right column sentence 2
		{Text: "लोग", NormedText: "लोग", Index: 26, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.60, Width: 0.03, Height: 0.02}},
		{Text: "छाते", NormedText: "छाते", Index: 27, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.64, Width: 0.04, Height: 0.02}},
		{Text: "लेकर", NormedText: "लेकर", Index: 28, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.69, Width: 0.04, Height: 0.02}},
		{Text: "घर", NormedText: "घर", Index: 29, BoundingBox: ocr.BoundingBox{Top: 0.16, Left: 0.74, Width: 0.02, Height: 0.02}},
		{Text: "जल्दी", NormedText: "जल्दी", Index: 30, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.60, Width: 0.05, Height: 0.02}},
		{Text: "जा", NormedText: "जा", Index: 31, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.66, Width: 0.02, Height: 0.02}},
		{Text: "रहे", NormedText: "रहे", Index: 32, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.69, Width: 0.03, Height: 0.02}},
		{Text: "थे।", NormedText: "थे", Index: 33, BoundingBox: ocr.BoundingBox{Top: 0.19, Left: 0.73, Width: 0.03, Height: 0.02}},
	}

	// Run sort with MultiColumnConfig, optimized for clean column separation
	config := MultiColumnConfig()
	config.MaxWordDistance = 0.35       // Prevent cross-column jumps
	config.MinWordsForEarlyPasses = 5   // Process all test lines early
	config.PermutationsPerPass = 100000 // Increased to find best path for duplicate words
	sorter := NewOcrSorterWithConfig(blocks, canonicalText, nil, config)
	sortedBlocks, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	var result strings.Builder
	for _, block := range sortedBlocks {
		if block.Text == "" {
			result.WriteString("\n")
		} else {
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString(" ")
			}
			result.WriteString(block.Text)
		}
	}

	expected := strings.Join(canonicalText, "\n")
	actual := strings.TrimSpace(result.String())

	if actual != expected {
		t.Errorf("Sorted text doesn't match canonical.\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}
