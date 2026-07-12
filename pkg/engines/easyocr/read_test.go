package easyocr

import (
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	input := `{
  "results": [
    {
      "bbox": [[10, 20], [60, 20], [60, 40], [10, 40]],
      "text": "Hello",
      "confidence": 0.95
    },
    {
      "bbox": [[70, 20], [130, 20], [130, 40], [70, 40]],
      "text": "World",
      "confidence": 0.92
    },
    {
      "bbox": [[10, 50], [80, 50], [80, 70], [10, 70]],
      "text": "Test",
      "confidence": 0.98
    }
  ]
}`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(blocks) != 3 {
		t.Fatalf("Expected 3 blocks, got %d", len(blocks))
	}

	// Check first block
	if blocks[0].Text != "Hello" {
		t.Errorf("Expected text 'Hello', got '%s'", blocks[0].Text)
	}
	if blocks[0].Confidence != 0.95 {
		t.Errorf("Expected Confidence 0.95, got %f", blocks[0].Confidence)
	}

	// Check bbox conversion
	if len(blocks[0].Boxes) != 4 {
		t.Fatalf("Expected 4 bbox points, got %d", len(blocks[0].Boxes))
	}
	if blocks[0].Boxes[0][0] != 10 || blocks[0].Boxes[0][1] != 20 {
		t.Errorf("Expected first point [10,20], got [%d,%d]",
			blocks[0].Boxes[0][0], blocks[0].Boxes[0][1])
	}
	if blocks[0].Boxes[2][0] != 60 || blocks[0].Boxes[2][1] != 40 {
		t.Errorf("Expected third point [60,40], got [%d,%d]",
			blocks[0].Boxes[2][0], blocks[0].Boxes[2][1])
	}

	// Check engine name
	if blocks[0].Engine() != "easyocr" {
		t.Errorf("Expected engine 'easyocr', got '%s'", blocks[0].Engine())
	}
}

func TestReadEmptyText(t *testing.T) {
	input := `{
  "results": [
    {
      "bbox": [[10, 20], [60, 20], [60, 40], [10, 40]],
      "text": "",
      "confidence": 0.95
    },
    {
      "bbox": [[70, 20], [130, 20], [130, 40], [70, 40]],
      "text": "Valid",
      "confidence": 0.92
    }
  ]
}`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Empty text blocks should be filtered out
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block (empty filtered), got %d", len(blocks))
	}

	if blocks[0].Text != "Valid" {
		t.Errorf("Expected text 'Valid', got '%s'", blocks[0].Text)
	}
}

func TestReadInvalidBBox(t *testing.T) {
	input := `{
  "results": [
    {
      "bbox": [[10, 20], [60, 20]],
      "text": "Invalid",
      "confidence": 0.95
    },
    {
      "bbox": [[70, 20], [130, 20], [130, 40], [70, 40]],
      "text": "Valid",
      "confidence": 0.92
    }
  ]
}`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Invalid bbox (only 2 points) should be filtered out
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block (invalid bbox filtered), got %d", len(blocks))
	}

	if blocks[0].Text != "Valid" {
		t.Errorf("Expected text 'Valid', got '%s'", blocks[0].Text)
	}
}

func TestReadInvalidJSON(t *testing.T) {
	input := `invalid json{`

	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestReadNoResults(t *testing.T) {
	input := `{"results": []}`

	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatal("Expected error for no results, got nil")
	}
}

func TestReadFloatCoordinates(t *testing.T) {
	input := `{
  "results": [
    {
      "bbox": [[10.5, 20.8], [60.2, 20.1], [60.9, 40.3], [10.1, 40.7]],
      "text": "Test",
      "confidence": 0.87
    }
  ]
}`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Float coordinates should be converted to int
	if blocks[0].Boxes[0][0] != 10 || blocks[0].Boxes[0][1] != 20 {
		t.Errorf("Expected first point [10,20] (truncated), got [%d,%d]",
			blocks[0].Boxes[0][0], blocks[0].Boxes[0][1])
	}
}
