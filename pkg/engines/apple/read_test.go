package apple

import (
	"os"
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	input := `[
  {
    "text": "Hello World",
    "confidence": 0.95,
    "rect": {
      "top": 0.1,
      "left": 0.2,
      "width": 0.3,
      "height": 0.05
    },
    "words": [
      {
        "text": "Hello",
        "top": 0.1,
        "left": 0.2,
        "width": 0.15,
        "height": 0.05,
        "confidence": 0.96
      },
      {
        "text": "World",
        "top": 0.1,
        "left": 0.36,
        "width": 0.14,
        "height": 0.05,
        "confidence": 0.94
      }
    ]
  },
  {
    "text": "Second line",
    "confidence": 0.88,
    "rect": {
      "top": 0.2,
      "left": 0.2,
      "width": 0.25,
      "height": 0.05
    },
    "words": [
      {
        "text": "Second",
        "top": 0.2,
        "left": 0.2,
        "width": 0.12,
        "height": 0.05,
        "confidence": 0.89
      },
      {
        "text": "line",
        "top": 0.2,
        "left": 0.33,
        "width": 0.12,
        "height": 0.05,
        "confidence": 0.87
      }
    ]
  }
]`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Should have 4 words total (2 from each line)
	if len(blocks) != 4 {
		t.Fatalf("Expected 4 blocks, got %d", len(blocks))
	}

	// Check first word
	if blocks[0].Text != "Hello" {
		t.Errorf("Expected text 'Hello', got '%s'", blocks[0].Text)
	}
	if blocks[0].LineNum != 0 {
		t.Errorf("Expected LineNum 0, got %d", blocks[0].LineNum)
	}
	// Confidence should be from line level (0.95), not word level
	if blocks[0].Confidence != 0.95 {
		t.Errorf("Expected Confidence 0.95 (from line), got %f", blocks[0].Confidence)
	}
	if blocks[0].Top != 0.1 {
		t.Errorf("Expected Top 0.1, got %f", blocks[0].Top)
	}
	if blocks[0].Left != 0.2 {
		t.Errorf("Expected Left 0.2, got %f", blocks[0].Left)
	}

	// Check second word (still first line)
	if blocks[1].Text != "World" {
		t.Errorf("Expected text 'World', got '%s'", blocks[1].Text)
	}
	if blocks[1].LineNum != 0 {
		t.Errorf("Expected LineNum 0, got %d", blocks[1].LineNum)
	}
	if blocks[1].Confidence != 0.95 {
		t.Errorf("Expected Confidence 0.95 (from line), got %f", blocks[1].Confidence)
	}

	// Check third word (second line)
	if blocks[2].Text != "Second" {
		t.Errorf("Expected text 'Second', got '%s'", blocks[2].Text)
	}
	if blocks[2].LineNum != 1 {
		t.Errorf("Expected LineNum 1, got %d", blocks[2].LineNum)
	}
	if blocks[2].Confidence != 0.88 {
		t.Errorf("Expected Confidence 0.88 (from line), got %f", blocks[2].Confidence)
	}

	// Check fourth word (second line)
	if blocks[3].Text != "line" {
		t.Errorf("Expected text 'line', got '%s'", blocks[3].Text)
	}
	if blocks[3].LineNum != 1 {
		t.Errorf("Expected LineNum 1, got %d", blocks[3].LineNum)
	}
}

func TestReadEmptyLines(t *testing.T) {
	input := `[]`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(blocks) != 0 {
		t.Errorf("Expected 0 blocks for empty lines, got %d", len(blocks))
	}
}

func TestReadLineWithNoWords(t *testing.T) {
	input := `[
  {
    "text": "Empty line",
    "confidence": 0.90,
    "rect": {
      "top": 0.1,
      "left": 0.2,
      "width": 0.3,
      "height": 0.05
    },
    "words": []
  },
  {
    "text": "Has words",
    "confidence": 0.85,
    "rect": {
      "top": 0.2,
      "left": 0.2,
      "width": 0.25,
      "height": 0.05
    },
    "words": [
      {
        "text": "Has",
        "top": 0.2,
        "left": 0.2,
        "width": 0.1,
        "height": 0.05,
        "confidence": 0.84
      }
    ]
  }
]`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Should only have 1 word (from second line)
	if len(blocks) != 1 {
		t.Errorf("Expected 1 block (empty line skipped), got %d", len(blocks))
	}

	if blocks[0].Text != "Has" {
		t.Errorf("Expected text 'Has', got '%s'", blocks[0].Text)
	}
	if blocks[0].LineNum != 1 {
		t.Errorf("Expected LineNum 1 (second line), got %d", blocks[0].LineNum)
	}
}

func TestReadInvalidJSON(t *testing.T) {
	input := `invalid json{`

	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestReadSingleWord(t *testing.T) {
	input := `[
  {
    "text": "Solo",
    "confidence": 0.99,
    "rect": {
      "top": 0.5,
      "left": 0.5,
      "width": 0.1,
      "height": 0.05
    },
    "words": [
      {
        "text": "Solo",
        "top": 0.5,
        "left": 0.5,
        "width": 0.1,
        "height": 0.05,
        "confidence": 0.98
      }
    ]
  }
]`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Text != "Solo" {
		t.Errorf("Expected text 'Solo', got '%s'", blocks[0].Text)
	}
	if blocks[0].Confidence != 0.99 {
		t.Errorf("Expected Confidence 0.99 (from line), got %f", blocks[0].Confidence)
	}
}

func TestReadConfidenceOverride(t *testing.T) {
	input := `[
  {
    "text": "Line confidence",
    "confidence": 0.75,
    "rect": {
      "top": 0.1,
      "left": 0.2,
      "width": 0.3,
      "height": 0.05
    },
    "words": [
      {
        "text": "Test",
        "top": 0.1,
        "left": 0.2,
        "width": 0.1,
        "height": 0.05,
        "confidence": 0.99
      }
    ]
  }
]`

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Word confidence (0.99) should be overridden by line confidence (0.75)
	if blocks[0].Confidence != 0.75 {
		t.Errorf("Expected Confidence 0.75 (line confidence should override word), got %f", blocks[0].Confidence)
	}
}

func TestReadFile(t *testing.T) {
	// Create temporary test file
	content := `[
  {
    "text": "File test",
    "confidence": 0.92,
    "rect": {
      "top": 0.1,
      "left": 0.2,
      "width": 0.3,
      "height": 0.05
    },
    "words": [
      {
        "text": "File",
        "top": 0.1,
        "left": 0.2,
        "width": 0.1,
        "height": 0.05,
        "confidence": 0.91
      },
      {
        "text": "test",
        "top": 0.1,
        "left": 0.31,
        "width": 0.09,
        "height": 0.05,
        "confidence": 0.93
      }
    ]
  }
]`

	tmpfile, err := os.CreateTemp("", "apple-ocr-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	blocks, err := ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].Text != "File" {
		t.Errorf("Expected text 'File', got '%s'", blocks[0].Text)
	}
	if blocks[1].Text != "test" {
		t.Errorf("Expected text 'test', got '%s'", blocks[1].Text)
	}
}

func TestReadFileNotFound(t *testing.T) {
	_, err := ReadFile("/nonexistent/path/file.json")
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
}

func TestReadManyLines(t *testing.T) {
	// Test with many lines to ensure LineNum increments correctly
	var lines []string
	lines = append(lines, `[`)
	for i := 0; i < 10; i++ {
		if i > 0 {
			lines = append(lines, `,`)
		}
		lines = append(lines, `{
    "text": "Line",
    "confidence": 0.9,
    "rect": {"top": 0.1, "left": 0.2, "width": 0.3, "height": 0.05},
    "words": [
      {
        "text": "Word",
        "top": 0.1,
        "left": 0.2,
        "width": 0.1,
        "height": 0.05,
        "confidence": 0.9
      }
    ]
  }`)
	}
	lines = append(lines, `]`)
	input := strings.Join(lines, "\n")

	blocks, err := Read(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(blocks) != 10 {
		t.Fatalf("Expected 10 blocks, got %d", len(blocks))
	}

	// Check that LineNum increments correctly
	for i := 0; i < 10; i++ {
		if blocks[i].LineNum != i {
			t.Errorf("Block %d: expected LineNum %d, got %d", i, i, blocks[i].LineNum)
		}
	}
}
