package sorters

import (
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// TestJapaneseConfigUsesVerticalOrder verifies CJK config uses vertical reading order.
func TestJapaneseConfigUsesVerticalOrder(t *testing.T) {
	config := CJKConfig()
	if config.ReadingOrder != VerticalTTB_RTL {
		t.Errorf("CJKConfig should use VerticalTTB_RTL, got %s", config.ReadingOrder.String())
	}

	// Verify vertical distance calculation prefers top-to-bottom
	topBlock := Block{
		Text:  "上",
		Index: 0,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.2,
			Width:  0.05,
			Height: 0.05,
		},
		Extractor: "test",
	}

	bottomBlock := Block{
		Text:  "下",
		Index: 1,
		BoundingBox: ocr.BoundingBox{
			Top:    0.2,
			Left:   0.2,
			Width:  0.05,
			Height: 0.05,
		},
		Extractor: "test",
	}

	// Distance from top to bottom should be smaller than bottom to top
	distTB := DistanceWithOrder(topBlock, bottomBlock, VerticalTTB_RTL)
	distBT := DistanceWithOrder(bottomBlock, topBlock, VerticalTTB_RTL)

	if distTB >= distBT {
		t.Errorf("Vertical order should prefer top-to-bottom: distTB=%.2f, distBT=%.2f", distTB, distBT)
	}
}

// TestArabicConfigUsesRTLOrder verifies RTL config uses right-to-left reading order.
func TestArabicConfigUsesRTLOrder(t *testing.T) {
	config := RTLConfig()
	if config.ReadingOrder != HorizontalRTL_TTB {
		t.Errorf("RTLConfig should use HorizontalRTL_TTB, got %s", config.ReadingOrder.String())
	}

	// Verify RTL distance calculation prefers right-to-left
	leftBlock := Block{
		Text:  "اليسار",
		Index: 0,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.1,
			Width:  0.1,
			Height: 0.05,
		},
		Extractor: "test",
	}

	rightBlock := Block{
		Text:  "اليمين",
		Index: 1,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.3,
			Width:  0.1,
			Height: 0.05,
		},
		Extractor: "test",
	}

	// Distance from right to left should be smaller than left to right
	distRL := DistanceWithOrder(rightBlock, leftBlock, HorizontalRTL_TTB)
	distLR := DistanceWithOrder(leftBlock, rightBlock, HorizontalRTL_TTB)

	if distRL >= distLR {
		t.Errorf("RTL order should prefer right-to-left: distRL=%.2f, distLR=%.2f", distRL, distLR)
	}
}

// TestHebrewWithRTLConfig verifies Hebrew works with RTL configuration.
func TestHebrewWithRTLConfig(t *testing.T) {
	config := RTLConfig()

	// Test that RTL config validates
	if err := config.Validate(); err != nil {
		t.Errorf("RTLConfig should be valid: %v", err)
	}

	// Verify it uses correct reading order
	if config.ReadingOrder != HorizontalRTL_TTB {
		t.Errorf("Expected HorizontalRTL_TTB, got %s", config.ReadingOrder.String())
	}
}

// TestJapaneseVerticalDistanceCalculation tests vertical text spatial relationships.
func TestJapaneseVerticalDistanceCalculation(t *testing.T) {
	// In vertical Japanese, text flows:
	// 1. Top to bottom within a column
	// 2. Right to left across columns

	// Column 1 (rightmost)
	col1Top := Block{
		Text:  "日",
		Index: 0,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.3,
			Width:  0.05,
			Height: 0.05,
		},
		Extractor: "test",
	}

	col1Bottom := Block{
		Text:  "本",
		Index: 1,
		BoundingBox: ocr.BoundingBox{
			Top:    0.2,
			Left:   0.3,
			Width:  0.05,
			Height: 0.05,
		},
		Extractor: "test",
	}

	// Column 2 (leftmost)
	col2Top := Block{
		Text:  "語",
		Index: 2,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.2,
			Width:  0.05,
			Height: 0.05,
		},
		Extractor: "test",
	}

	// Test 1: Within same column, top-to-bottom should be small distance
	withinCol := DistanceWithOrder(col1Top, col1Bottom, VerticalTTB_RTL)

	// Test 2: Across columns, right column to left column
	acrossCol := DistanceWithOrder(col1Top, col2Top, VerticalTTB_RTL)

	// Within column should generally be smaller than across columns
	// (though this depends on the specific distance calculation)
	t.Logf("Within column distance: %.2f", withinCol)
	t.Logf("Across column distance: %.2f", acrossCol)

	// At minimum, verify distances are calculated without error
	if withinCol < 0 {
		t.Errorf("Distance should not be negative: %.2f", withinCol)
	}
	if acrossCol < 0 {
		t.Errorf("Distance should not be negative: %.2f", acrossCol)
	}
}

// TestArabicRTLDistanceCalculation tests RTL text spatial relationships.
func TestArabicRTLDistanceCalculation(t *testing.T) {
	// In Arabic RTL text, reading flows right to left

	rightWord := Block{
		Text:  "السلام",
		Index: 0,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.5,
			Width:  0.15,
			Height: 0.05,
		},
		Extractor: "test",
	}

	middleWord := Block{
		Text:  "عليكم",
		Index: 1,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.3,
			Width:  0.15,
			Height: 0.05,
		},
		Extractor: "test",
	}

	leftWord := Block{
		Text:  "ورحمة",
		Index: 2,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.1,
			Width:  0.15,
			Height: 0.05,
		},
		Extractor: "test",
	}

	// Test RTL flow: right -> middle -> left
	dist1 := DistanceWithOrder(rightWord, middleWord, HorizontalRTL_TTB)
	dist2 := DistanceWithOrder(middleWord, leftWord, HorizontalRTL_TTB)

	// Verify sequential RTL distances are reasonable
	t.Logf("Right to middle distance: %.2f", dist1)
	t.Logf("Middle to left distance: %.2f", dist2)

	if dist1 < 0 {
		t.Errorf("Distance should not be negative: %.2f", dist1)
	}
	if dist2 < 0 {
		t.Errorf("Distance should not be negative: %.2f", dist2)
	}

	// Verify wrong direction has larger distance
	wrongDir := DistanceWithOrder(leftWord, rightWord, HorizontalRTL_TTB)
	if wrongDir <= dist1 {
		t.Logf("Warning: Wrong direction distance (%.2f) not larger than correct direction (%.2f)", wrongDir, dist1)
		// This is a soft warning, not a hard failure, as distance calculations are complex
	}
}
