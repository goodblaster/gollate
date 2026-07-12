package tesseract

import (
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	input := `{
  "words": [
    {
      "text": "Hello",
      "line_num": 0,
      "left": 100,
      "top": 200,
      "width": 50,
      "height": 20,
      "conf": 95.5
    },
    {
      "text": "World",
      "line_num": 0,
      "left": 160,
      "top": 200,
      "width": 55,
      "height": 20,
      "conf": 92.3
    },
    {
      "text": "Test",
      "line_num": 1,
      "left": 100,
      "top": 230,
      "width": 45,
      "height": 20,
      "conf": 98.1
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
	if blocks[0].LineNum != 0 {
		t.Errorf("Expected LineNum 0, got %d", blocks[0].LineNum)
	}
	if blocks[0].Left != 100 {
		t.Errorf("Expected Left 100, got %d", blocks[0].Left)
	}
	if blocks[0].Top != 200 {
		t.Errorf("Expected Top 200, got %d", blocks[0].Top)
	}
	if blocks[0].Confidence != 95 {
		t.Errorf("Expected Confidence 95, got %d", blocks[0].Confidence)
	}

	// Check engine name
	if blocks[0].Engine() != "tesseract" {
		t.Errorf("Expected engine 'tesseract', got '%s'", blocks[0].Engine())
	}

	// Check second block is on same line
	if blocks[1].LineNum != 0 {
		t.Errorf("Expected second block on line 0, got %d", blocks[1].LineNum)
	}

	// Check third block is on different line
	if blocks[2].LineNum != 1 {
		t.Errorf("Expected third block on line 1, got %d", blocks[2].LineNum)
	}
}

func TestReadEmptyWords(t *testing.T) {
	input := `{
  "words": [
    {
      "text": "",
      "line_num": 0,
      "left": 100,
      "top": 200,
      "width": 50,
      "height": 20,
      "conf": 95.5
    },
    {
      "text": "Valid",
      "line_num": 0,
      "left": 160,
      "top": 200,
      "width": 55,
      "height": 20,
      "conf": 92.3
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

func TestReadInvalidJSON(t *testing.T) {
	input := `invalid json{`

	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestReadNoWords(t *testing.T) {
	input := `{"words": []}`

	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatal("Expected error for no words, got nil")
	}
}

func TestReadConfidenceConversion(t *testing.T) {
	input := `{
  "words": [
    {
      "text": "Test",
      "line_num": 0,
      "left": 100,
      "top": 200,
      "width": 50,
      "height": 20,
      "conf": 87.654
    }
  ]
}`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Confidence should be converted from float to int
	if blocks[0].Confidence != 87 {
		t.Errorf("Expected Confidence 87 (truncated), got %d", blocks[0].Confidence)
	}
}
