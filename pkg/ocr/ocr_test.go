package ocr

import (
	"testing"

	"github.com/goodblaster/gollate/pkg/engines/apple"
	"github.com/goodblaster/gollate/pkg/engines/easyocr"
	"github.com/goodblaster/gollate/pkg/engines/tesseract"
)

func TestFromTesseract(t *testing.T) {
	tessBlock := tesseract.Block{
		Text:       "Hello",
		LineNum:    0,
		Left:       100,
		Top:        200,
		Width:      50,
		Height:     20,
		Confidence: 95,
	}

	block := FromTesseract(tessBlock, 1920, 1080)

	// Check text
	if block.Text != "Hello" {
		t.Errorf("Expected text 'Hello', got '%s'", block.Text)
	}

	// Check bounding box normalization (0-1 range)
	expectedLeft := 100.0 / 1920.0
	if block.BoundingBox.Left != expectedLeft {
		t.Errorf("Expected Left %f, got %f", expectedLeft, block.BoundingBox.Left)
	}

	expectedTop := 200.0 / 1080.0
	if block.BoundingBox.Top != expectedTop {
		t.Errorf("Expected Top %f, got %f", expectedTop, block.BoundingBox.Top)
	}

	expectedWidth := 50.0 / 1920.0
	if block.BoundingBox.Width != expectedWidth {
		t.Errorf("Expected Width %f, got %f", expectedWidth, block.BoundingBox.Width)
	}

	expectedHeight := 20.0 / 1080.0
	if block.BoundingBox.Height != expectedHeight {
		t.Errorf("Expected Height %f, got %f", expectedHeight, block.BoundingBox.Height)
	}

	// Check confidence normalization (95 / 100 = 0.95)
	expectedConf := 0.95
	if block.Confidence != expectedConf {
		t.Errorf("Expected Confidence %f, got %f", expectedConf, block.Confidence)
	}

	// Check engine name
	if block.Extractor != "tesseract" {
		t.Errorf("Expected Extractor 'tesseract', got '%s'", block.Extractor)
	}

	// Check page dimensions
	if block.PageWidth != 1920 {
		t.Errorf("Expected PageWidth 1920, got %d", block.PageWidth)
	}
	if block.PageHeight != 1080 {
		t.Errorf("Expected PageHeight 1080, got %d", block.PageHeight)
	}
}

func TestFromEasyOCR(t *testing.T) {
	easyBlock := easyocr.Block{
		Text: "World",
		Boxes: [][2]int{
			{10, 20}, // Top-left
			{60, 20}, // Top-right
			{60, 40}, // Bottom-right
			{10, 40}, // Bottom-left
		},
		Confidence: 0.92,
	}

	block := FromEasyOCR(easyBlock, 1920, 1080)

	// Check text
	if block.Text != "World" {
		t.Errorf("Expected text 'World', got '%s'", block.Text)
	}

	// Check bounding box calculation with offsets
	// Left: (10 + 2) / 1920 = 12 / 1920
	expectedLeft := 12.0 / 1920.0
	if block.BoundingBox.Left != expectedLeft {
		t.Errorf("Expected Left %f, got %f", expectedLeft, block.BoundingBox.Left)
	}

	// Top: (20 + 2) / 1080 = 22 / 1080
	expectedTop := 22.0 / 1080.0
	if block.BoundingBox.Top != expectedTop {
		t.Errorf("Expected Top %f, got %f", expectedTop, block.BoundingBox.Top)
	}

	// Width: (60 - 10 - 5) / 1920 = 45 / 1920
	expectedWidth := 45.0 / 1920.0
	if block.BoundingBox.Width != expectedWidth {
		t.Errorf("Expected Width %f, got %f", expectedWidth, block.BoundingBox.Width)
	}

	// Height: (40 - 20 - 5) / 1080 = 15 / 1080
	expectedHeight := 15.0 / 1080.0
	if block.BoundingBox.Height != expectedHeight {
		t.Errorf("Expected Height %f, got %f", expectedHeight, block.BoundingBox.Height)
	}

	// Check confidence (already 0-1 scale)
	if block.Confidence != 0.92 {
		t.Errorf("Expected Confidence 0.92, got %f", block.Confidence)
	}

	// Check engine name
	if block.Extractor != "easyocr" {
		t.Errorf("Expected Extractor 'easyocr', got '%s'", block.Extractor)
	}
}

func TestFromApple(t *testing.T) {
	appleBlock := apple.Block{
		Text:       "Test",
		Top:        0.1,
		Left:       0.2,
		Width:      0.3,
		Height:     0.4,
		Confidence: 0.85,
		LineNum:    5,
	}

	block := FromApple(appleBlock, 1920, 1080)

	// Check text
	if block.Text != "Test" {
		t.Errorf("Expected text 'Test', got '%s'", block.Text)
	}

	// Check bounding box (already normalized 0-1)
	if block.BoundingBox.Top != 0.1 {
		t.Errorf("Expected Top 0.1, got %f", block.BoundingBox.Top)
	}
	if block.BoundingBox.Left != 0.2 {
		t.Errorf("Expected Left 0.2, got %f", block.BoundingBox.Left)
	}
	if block.BoundingBox.Width != 0.3 {
		t.Errorf("Expected Width 0.3, got %f", block.BoundingBox.Width)
	}
	if block.BoundingBox.Height != 0.4 {
		t.Errorf("Expected Height 0.4, got %f", block.BoundingBox.Height)
	}

	// Check confidence
	if block.Confidence != 0.85 {
		t.Errorf("Expected Confidence 0.85, got %f", block.Confidence)
	}

	// Check engine name
	if block.Extractor != "apple" {
		t.Errorf("Expected Extractor 'apple', got '%s'", block.Extractor)
	}

	// Check LineId
	if block.LineId != "5" {
		t.Errorf("Expected LineId '5', got '%s'", block.LineId)
	}

	// Check page dimensions
	if block.PageWidth != 1920 {
		t.Errorf("Expected PageWidth 1920, got %d", block.PageWidth)
	}
	if block.PageHeight != 1080 {
		t.Errorf("Expected PageHeight 1080, got %d", block.PageHeight)
	}
}

func TestBlockMethods(t *testing.T) {
	block := Block{
		Text: "Test",
		BoundingBox: BoundingBox{
			Top:    0.1,
			Left:   0.2,
			Width:  0.3,
			Height: 0.4,
		},
		Confidence: 0.95,
		Extractor:  "test-engine",
		PageWidth:  1000,
		PageHeight: 500,
	}

	// Test String()
	if block.String() != "Test" {
		t.Errorf("Expected String() to return 'Test', got '%s'", block.String())
	}

	// Test Engine()
	if block.Engine() != "test-engine" {
		t.Errorf("Expected Engine() to return 'test-engine', got '%s'", block.Engine())
	}

	// Test Top()
	if block.Top() != 0.1 {
		t.Errorf("Expected Top() 0.1, got %f", block.Top())
	}

	// Test Left()
	if block.Left() != 0.2 {
		t.Errorf("Expected Left() 0.2, got %f", block.Left())
	}

	// Test Width()
	if block.Width() != 0.3 {
		t.Errorf("Expected Width() 0.3, got %f", block.Width())
	}

	// Test Height()
	if block.Height() != 0.4 {
		t.Errorf("Expected Height() 0.4, got %f", block.Height())
	}

	// Test Right() = Left + Width = 0.2 + 0.3 = 0.5
	expectedRight := 0.5
	if block.Right() != expectedRight {
		t.Errorf("Expected Right() %f, got %f", expectedRight, block.Right())
	}

	// Test Bottom() = Top + Height = 0.1 + 0.4 = 0.5
	expectedBottom := 0.5
	if block.Bottom() != expectedBottom {
		t.Errorf("Expected Bottom() %f, got %f", expectedBottom, block.Bottom())
	}

	// Test Center() = (Left + Width/2, Top + Height/2) = (0.35, 0.3)
	expectedCenterX := 0.35
	expectedCenterY := 0.3
	centerX, centerY := block.Center()
	const epsilon = 0.0001
	if abs(centerX-expectedCenterX) > epsilon || abs(centerY-expectedCenterY) > epsilon {
		t.Errorf("Expected Center() (%f, %f), got (%f, %f)",
			expectedCenterX, expectedCenterY, centerX, centerY)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestBlockPixelMethods(t *testing.T) {
	block := Block{
		BoundingBox: BoundingBox{
			Top:    0.1, // 0.1 * 500 = 50
			Left:   0.2, // 0.2 * 1000 = 200
			Width:  0.3, // 0.3 * 1000 = 300
			Height: 0.4, // 0.4 * 500 = 200
		},
		PageWidth:  1000,
		PageHeight: 500,
	}

	// Test PixelTop()
	if block.PixelTop() != 50 {
		t.Errorf("Expected PixelTop() 50, got %d", block.PixelTop())
	}

	// Test PixelLeft()
	if block.PixelLeft() != 200 {
		t.Errorf("Expected PixelLeft() 200, got %d", block.PixelLeft())
	}

	// Test PixelWidth()
	if block.PixelWidth() != 300 {
		t.Errorf("Expected PixelWidth() 300, got %d", block.PixelWidth())
	}

	// Test PixelHeight()
	if block.PixelHeight() != 200 {
		t.Errorf("Expected PixelHeight() 200, got %d", block.PixelHeight())
	}

	// Test PixelRight() = PixelLeft + PixelWidth = 200 + 300 = 500
	if block.PixelRight() != 500 {
		t.Errorf("Expected PixelRight() 500, got %d", block.PixelRight())
	}

	// Test PixelBottom() = PixelTop + PixelHeight = 50 + 200 = 250
	if block.PixelBottom() != 250 {
		t.Errorf("Expected PixelBottom() 250, got %d", block.PixelBottom())
	}

	// Test PixelCenter() = (PixelLeft + PixelWidth/2, PixelTop + PixelHeight/2)
	// = (200 + 150, 50 + 100) = (350, 150)
	expectedCenterX := 350
	expectedCenterY := 150
	centerX, centerY := block.PixelCenter()
	if centerX != expectedCenterX || centerY != expectedCenterY {
		t.Errorf("Expected PixelCenter() (%d, %d), got (%d, %d)",
			expectedCenterX, expectedCenterY, centerX, centerY)
	}
}

func TestFromTesseractZeroConfidence(t *testing.T) {
	tessBlock := tesseract.Block{
		Text:       "LowConf",
		Confidence: 0,
	}

	block := FromTesseract(tessBlock, 1920, 1080)

	if block.Confidence != 0.0 {
		t.Errorf("Expected Confidence 0.0, got %f", block.Confidence)
	}
}

func TestFromEasyOCRSmallBounds(t *testing.T) {
	easyBlock := easyocr.Block{
		Text: "X",
		Boxes: [][2]int{
			{0, 0},
			{5, 0},
			{5, 5},
			{0, 5},
		},
		Confidence: 0.5,
	}

	block := FromEasyOCR(easyBlock, 100, 100)

	// Width: (5 - 0 - 5) / 100 = 0 / 100 = 0
	// This is an edge case - very small bounds
	if block.BoundingBox.Width != 0.0 {
		t.Errorf("Expected Width 0.0 for small bounds, got %f", block.BoundingBox.Width)
	}
}

func TestFromAppleLargePageDimensions(t *testing.T) {
	appleBlock := apple.Block{
		Text:       "Large",
		Top:        0.5,
		Left:       0.5,
		Width:      0.1,
		Height:     0.1,
		Confidence: 0.99,
		LineNum:    100,
	}

	block := FromApple(appleBlock, 10000, 10000)

	if block.PageWidth != 10000 {
		t.Errorf("Expected PageWidth 10000, got %d", block.PageWidth)
	}
	if block.PageHeight != 10000 {
		t.Errorf("Expected PageHeight 10000, got %d", block.PageHeight)
	}

	// Verify pixel calculations work with large dimensions
	if block.PixelWidth() != 1000 {
		t.Errorf("Expected PixelWidth 1000, got %d", block.PixelWidth())
	}
}
