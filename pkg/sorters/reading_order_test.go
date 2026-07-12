package sorters

import (
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

func TestReadingOrderDistance(t *testing.T) {
	// Create two blocks side by side horizontally
	leftBlock := Block{
		Text:       "left",
		NormedText: "left",
		Index:      0,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.1,
			Width:  0.1,
			Height: 0.05,
		},
		Extractor: "test",
	}

	rightBlock := Block{
		Text:       "right",
		NormedText: "right",
		Index:      1,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.25,
			Width:  0.1,
			Height: 0.05,
		},
		Extractor: "test",
	}

	t.Run("HorizontalLTR prefers left-to-right", func(t *testing.T) {
		// Distance from left to right should be small (sequential)
		distLR := DistanceWithOrder(leftBlock, rightBlock, HorizontalLTR_TTB)
		// Distance from right to left should be large (wrong direction)
		distRL := DistanceWithOrder(rightBlock, leftBlock, HorizontalLTR_TTB)

		if distLR >= distRL {
			t.Errorf("HorizontalLTR should prefer left->right: got distLR=%.2f, distRL=%.2f", distLR, distRL)
		}
	})

	t.Run("HorizontalRTL prefers right-to-left", func(t *testing.T) {
		// Distance from right to left should be small (sequential)
		distRL := DistanceWithOrder(rightBlock, leftBlock, HorizontalRTL_TTB)
		// Distance from left to right should be large (wrong direction)
		distLR := DistanceWithOrder(leftBlock, rightBlock, HorizontalRTL_TTB)

		if distRL >= distLR {
			t.Errorf("HorizontalRTL should prefer right->left: got distRL=%.2f, distLR=%.2f", distRL, distLR)
		}
	})
}

func TestVerticalReadingOrder(t *testing.T) {
	// Create two blocks stacked vertically
	topBlock := Block{
		Text:       "top",
		NormedText: "top",
		Index:      0,
		BoundingBox: ocr.BoundingBox{
			Top:    0.1,
			Left:   0.1,
			Width:  0.05,
			Height: 0.1,
		},
		Extractor: "test",
	}

	bottomBlock := Block{
		Text:       "bottom",
		NormedText: "bottom",
		Index:      1,
		BoundingBox: ocr.BoundingBox{
			Top:    0.25,
			Left:   0.1,
			Width:  0.05,
			Height: 0.1,
		},
		Extractor: "test",
	}

	t.Run("VerticalTTB prefers top-to-bottom", func(t *testing.T) {
		// Distance from top to bottom should be small (sequential)
		distTB := DistanceWithOrder(topBlock, bottomBlock, VerticalTTB_RTL)
		// Distance from bottom to top should be large (wrong direction)
		distBT := DistanceWithOrder(bottomBlock, topBlock, VerticalTTB_RTL)

		if distTB >= distBT {
			t.Errorf("VerticalTTB should prefer top->bottom: got distTB=%.2f, distBT=%.2f", distTB, distBT)
		}
	})
}

func TestReadingOrderString(t *testing.T) {
	tests := []struct {
		order    ReadingOrder
		expected string
	}{
		{HorizontalLTR_TTB, "Horizontal LTR, Top-to-Bottom"},
		{HorizontalRTL_TTB, "Horizontal RTL, Top-to-Bottom"},
		{VerticalTTB_RTL, "Vertical Top-to-Bottom, Right-to-Left"},
		{VerticalTTB_LTR, "Vertical Top-to-Bottom, Left-to-Right"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if test.order.String() != test.expected {
				t.Errorf("ReadingOrder.String() = %q, want %q", test.order.String(), test.expected)
			}
		})
	}
}

func TestReadingOrderHelpers(t *testing.T) {
	t.Run("IsHorizontal", func(t *testing.T) {
		if !HorizontalLTR_TTB.IsHorizontal() {
			t.Error("HorizontalLTR_TTB should be horizontal")
		}
		if !HorizontalRTL_TTB.IsHorizontal() {
			t.Error("HorizontalRTL_TTB should be horizontal")
		}
		if VerticalTTB_RTL.IsHorizontal() {
			t.Error("VerticalTTB_RTL should not be horizontal")
		}
	})

	t.Run("IsVertical", func(t *testing.T) {
		if VerticalTTB_RTL.IsVertical() != true {
			t.Error("VerticalTTB_RTL should be vertical")
		}
		if VerticalTTB_LTR.IsVertical() != true {
			t.Error("VerticalTTB_LTR should be vertical")
		}
		if HorizontalLTR_TTB.IsVertical() {
			t.Error("HorizontalLTR_TTB should not be vertical")
		}
	})

	t.Run("IsLeftToRight", func(t *testing.T) {
		if !HorizontalLTR_TTB.IsLeftToRight() {
			t.Error("HorizontalLTR_TTB should be left-to-right")
		}
		if !VerticalTTB_LTR.IsLeftToRight() {
			t.Error("VerticalTTB_LTR should be left-to-right")
		}
		if HorizontalRTL_TTB.IsLeftToRight() {
			t.Error("HorizontalRTL_TTB should not be left-to-right")
		}
	})

	t.Run("IsRightToLeft", func(t *testing.T) {
		if !HorizontalRTL_TTB.IsRightToLeft() {
			t.Error("HorizontalRTL_TTB should be right-to-left")
		}
		if !VerticalTTB_RTL.IsRightToLeft() {
			t.Error("VerticalTTB_RTL should be right-to-left")
		}
		if HorizontalLTR_TTB.IsRightToLeft() {
			t.Error("HorizontalLTR_TTB should not be right-to-left")
		}
	})
}

// TestNewspaperColumnDetection tests that all reading orders correctly detect
// side-by-side column layouts (newspaper/magazine format).
func TestNewspaperColumnDetection(t *testing.T) {
	t.Run("HorizontalLTR_TTB newspaper columns", func(t *testing.T) {
		// Left column bottom
		leftColBottom := Block{
			Text:       "end",
			NormedText: "end",
			Index:      0,
			BoundingBox: ocr.BoundingBox{
				Top:    0.5, // Middle of page
				Left:   0.1, // Left column
				Width:  0.2,
				Height: 0.05,
			},
			Extractor: "test",
		}

		// Right column top (next word should be here)
		rightColTop := Block{
			Text:       "start",
			NormedText: "start",
			Index:      1,
			BoundingBox: ocr.BoundingBox{
				Top:    0.52, // Same vertical level (within tolerance)
				Left:   0.5,  // Right column (large gap = 0.2)
				Width:  0.2,
				Height: 0.05,
			},
			Extractor: "test",
		}

		// Should detect as column jump (not default huge penalty)
		if !isWrappedToNextColumn(leftColBottom, rightColTop, HorizontalLTR_TTB) {
			t.Error("HorizontalLTR_TTB should detect newspaper column jump to right")
		}

		// Distance should use column penalty, not default
		dist := DistanceWithOrder(leftColBottom, rightColTop, HorizontalLTR_TTB)
		if dist >= BaseDefault {
			t.Errorf("HorizontalLTR_TTB column jump should use column penalty (%.2f), got %.2f", BaseNextColumn, dist)
		}
	})

	t.Run("HorizontalRTL_TTB newspaper columns", func(t *testing.T) {
		// Right column bottom (start here in RTL)
		rightColBottom := Block{
			Text:       "end",
			NormedText: "end",
			Index:      0,
			BoundingBox: ocr.BoundingBox{
				Top:    0.5, // Middle of page
				Left:   0.6, // Right column
				Width:  0.2,
				Height: 0.05,
			},
			Extractor: "test",
		}

		// Left column top (next word in RTL should be here)
		leftColTop := Block{
			Text:       "start",
			NormedText: "start",
			Index:      1,
			BoundingBox: ocr.BoundingBox{
				Top:    0.52, // Same vertical level (within tolerance)
				Left:   0.2,  // Left column (large gap to left = 0.2)
				Width:  0.2,
				Height: 0.05,
			},
			Extractor: "test",
		}

		// Should detect as column jump (not default huge penalty)
		if !isWrappedToNextColumn(rightColBottom, leftColTop, HorizontalRTL_TTB) {
			t.Error("HorizontalRTL_TTB should detect newspaper column jump to left")
		}

		// Distance should use column penalty, not default
		dist := DistanceWithOrder(rightColBottom, leftColTop, HorizontalRTL_TTB)
		if dist >= BaseDefault {
			t.Errorf("HorizontalRTL_TTB column jump should use column penalty (%.2f), got %.2f", BaseNextColumn, dist)
		}
	})

	t.Run("VerticalTTB_RTL newspaper columns", func(t *testing.T) {
		// Right column bottom (start here in vertical RTL)
		rightColBottom := Block{
			Text:       "end",
			NormedText: "end",
			Index:      0,
			BoundingBox: ocr.BoundingBox{
				Top:    0.5, // Bottom of right column
				Left:   0.7, // Right column
				Width:  0.05,
				Height: 0.2,
			},
			Extractor: "test",
		}

		// Left column at same height (next in vertical RTL)
		leftColSameHeight := Block{
			Text:       "start",
			NormedText: "start",
			Index:      1,
			BoundingBox: ocr.BoundingBox{
				Top:    0.52, // Same horizontal level (within tolerance)
				Left:   0.3,  // Left column (large gap to left = 0.35)
				Width:  0.05,
				Height: 0.2,
			},
			Extractor: "test",
		}

		// Should detect as column jump (not default huge penalty)
		if !isWrappedToNextColumn(rightColBottom, leftColSameHeight, VerticalTTB_RTL) {
			t.Error("VerticalTTB_RTL should detect newspaper column jump to left")
		}

		// Distance should use column penalty, not default
		dist := DistanceWithOrder(rightColBottom, leftColSameHeight, VerticalTTB_RTL)
		if dist >= BaseDefault {
			t.Errorf("VerticalTTB_RTL column jump should use column penalty (%.2f), got %.2f", BaseNextColumn, dist)
		}
	})

	t.Run("VerticalTTB_LTR newspaper columns", func(t *testing.T) {
		// Left column bottom (start here in vertical LTR)
		leftColBottom := Block{
			Text:       "end",
			NormedText: "end",
			Index:      0,
			BoundingBox: ocr.BoundingBox{
				Top:    0.5, // Bottom of left column
				Left:   0.1, // Left column
				Width:  0.05,
				Height: 0.2,
			},
			Extractor: "test",
		}

		// Right column at same height (next in vertical LTR)
		rightColSameHeight := Block{
			Text:       "start",
			NormedText: "start",
			Index:      1,
			BoundingBox: ocr.BoundingBox{
				Top:    0.52, // Same horizontal level (within tolerance)
				Left:   0.5,  // Right column (large gap to right = 0.35)
				Width:  0.05,
				Height: 0.2,
			},
			Extractor: "test",
		}

		// Should detect as column jump (not default huge penalty)
		if !isWrappedToNextColumn(leftColBottom, rightColSameHeight, VerticalTTB_LTR) {
			t.Error("VerticalTTB_LTR should detect newspaper column jump to right")
		}

		// Distance should use column penalty, not default
		dist := DistanceWithOrder(leftColBottom, rightColSameHeight, VerticalTTB_LTR)
		if dist >= BaseDefault {
			t.Errorf("VerticalTTB_LTR column jump should use column penalty (%.2f), got %.2f", BaseNextColumn, dist)
		}
	})

	// Test that small gaps are NOT detected as column jumps
	t.Run("Small gaps should not trigger column detection", func(t *testing.T) {
		block1 := Block{
			Text:       "word1",
			NormedText: "word1",
			Index:      0,
			BoundingBox: ocr.BoundingBox{
				Top:    0.5,
				Left:   0.3,
				Width:  0.1,
				Height: 0.05,
			},
			Extractor: "test",
		}

		// Small gap (< 15% threshold)
		block2 := Block{
			Text:       "word2",
			NormedText: "word2",
			Index:      1,
			BoundingBox: ocr.BoundingBox{
				Top:    0.5,
				Left:   0.42, // Only 0.02 gap
				Width:  0.1,
				Height: 0.05,
			},
			Extractor: "test",
		}

		// Should NOT be detected as column jump for any reading order
		if isWrappedToNextColumn(block1, block2, HorizontalLTR_TTB) {
			t.Error("Small gap should not trigger HorizontalLTR_TTB column detection")
		}
		if isWrappedToNextColumn(block2, block1, HorizontalRTL_TTB) {
			t.Error("Small gap should not trigger HorizontalRTL_TTB column detection")
		}
	})
}
