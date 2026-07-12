package api

import (
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		expected string
	}{
		{
			name: "with value",
			err: ValidationError{
				Field:   "engine",
				Value:   "invalid-engine",
				Message: "unsupported engine",
			},
			expected: "invalid engine (value: invalid-engine): unsupported engine",
		},
		{
			name: "without value",
			err: ValidationError{
				Field:   "lines",
				Value:   nil,
				Message: "at least one line required",
			},
			expected: "invalid lines: at least one line required",
		},
		{
			name: "with numeric value",
			err: ValidationError{
				Field:   "page_width",
				Value:   -100,
				Message: "must be greater than 0",
			},
			expected: "invalid page_width (value: -100): must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message '%s', got '%s'", tt.expected, tt.err.Error())
			}
		})
	}
}

func TestSortRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     SortRequest
		expectError bool
		errorField  string
	}{
		{
			name: "valid tesseract request",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      []string{"line 1", "line 2"},
				InputJson:  `{"words":[]}`,
				PageWidth:  1920,
				PageHeight: 1080,
			},
			expectError: false,
		},
		{
			name: "valid easyocr request",
			request: SortRequest{
				Engine:     "easyocr",
				Lines:      []string{"test"},
				InputJson:  `{"results":[]}`,
				PageWidth:  1000,
				PageHeight: 1000,
			},
			expectError: false,
		},
		{
			name: "valid apple request",
			request: SortRequest{
				Engine:     "apple",
				Lines:      []string{"test"},
				InputJson:  `[]`,
				PageWidth:  2000,
				PageHeight: 1500,
			},
			expectError: false,
		},
		{
			name: "missing engine",
			request: SortRequest{
				Lines:      []string{"test"},
				InputJson:  `{}`,
				PageWidth:  1920,
				PageHeight: 1080,
			},
			expectError: true,
			errorField:  "engine",
		},
		{
			name: "unsupported engine",
			request: SortRequest{
				Engine:     "invalid-engine",
				Lines:      []string{"test"},
				InputJson:  `{}`,
				PageWidth:  1920,
				PageHeight: 1080,
			},
			expectError: true,
			errorField:  "engine",
		},
		{
			name: "zero page width",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      []string{"test"},
				InputJson:  `{}`,
				PageWidth:  0,
				PageHeight: 1080,
			},
			expectError: true,
			errorField:  "page_width",
		},
		{
			name: "negative page width",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      []string{"test"},
				InputJson:  `{}`,
				PageWidth:  -100,
				PageHeight: 1080,
			},
			expectError: true,
			errorField:  "page_width",
		},
		{
			name: "zero page height",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      []string{"test"},
				InputJson:  `{}`,
				PageWidth:  1920,
				PageHeight: 0,
			},
			expectError: true,
			errorField:  "page_height",
		},
		{
			name: "negative page height",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      []string{"test"},
				InputJson:  `{}`,
				PageWidth:  1920,
				PageHeight: -500,
			},
			expectError: true,
			errorField:  "page_height",
		},
		{
			name: "empty lines",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      []string{},
				InputJson:  `{}`,
				PageWidth:  1920,
				PageHeight: 1080,
			},
			expectError: true,
			errorField:  "lines",
		},
		{
			name: "nil lines",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      nil,
				InputJson:  `{}`,
				PageWidth:  1920,
				PageHeight: 1080,
			},
			expectError: true,
			errorField:  "lines",
		},
		{
			name: "empty input json",
			request: SortRequest{
				Engine:     "tesseract",
				Lines:      []string{"test"},
				InputJson:  "",
				PageWidth:  1920,
				PageHeight: 1080,
			},
			expectError: true,
			errorField:  "input_json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error for field '%s', got nil", tt.errorField)
					return
				}

				valErr, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("Expected ValidationError, got %T", err)
					return
				}

				if valErr.Field != tt.errorField {
					t.Errorf("Expected error for field '%s', got '%s'", tt.errorField, valErr.Field)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestSortRequest_Parse_Tesseract(t *testing.T) {
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
    }
  ]
}`

	request := SortRequest{
		Engine:     "tesseract",
		Lines:      []string{"Hello World"},
		InputJson:  input,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	err := request.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	blocks := request.Blocks()
	if len(blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].Text != "Hello" {
		t.Errorf("Expected text 'Hello', got '%s'", blocks[0].Text)
	}
	if blocks[1].Text != "World" {
		t.Errorf("Expected text 'World', got '%s'", blocks[1].Text)
	}

	// Check LineId was set
	if blocks[0].LineId != "0" {
		t.Errorf("Expected LineId '0', got '%s'", blocks[0].LineId)
	}
}

func TestSortRequest_Parse_EasyOCR(t *testing.T) {
	input := `{
  "results": [
    {
      "bbox": [[10, 20], [60, 20], [60, 40], [10, 40]],
      "text": "Test",
      "confidence": 0.95
    }
  ]
}`

	request := SortRequest{
		Engine:     "easyocr",
		Lines:      []string{"Test"},
		InputJson:  input,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	err := request.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	blocks := request.Blocks()
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Text != "Test" {
		t.Errorf("Expected text 'Test', got '%s'", blocks[0].Text)
	}
}

func TestSortRequest_Parse_Apple(t *testing.T) {
	input := `[
  {
    "text": "Apple Vision",
    "confidence": 0.92,
    "rect": {
      "top": 0.1,
      "left": 0.2,
      "width": 0.3,
      "height": 0.05
    },
    "words": [
      {
        "text": "Apple",
        "top": 0.1,
        "left": 0.2,
        "width": 0.15,
        "height": 0.05,
        "confidence": 0.93
      },
      {
        "text": "Vision",
        "top": 0.1,
        "left": 0.36,
        "width": 0.14,
        "height": 0.05,
        "confidence": 0.91
      }
    ]
  }
]`

	request := SortRequest{
		Engine:     "apple",
		Lines:      []string{"Apple Vision"},
		InputJson:  input,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	err := request.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	blocks := request.Blocks()
	if len(blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].Text != "Apple" {
		t.Errorf("Expected text 'Apple', got '%s'", blocks[0].Text)
	}
	if blocks[1].Text != "Vision" {
		t.Errorf("Expected text 'Vision', got '%s'", blocks[1].Text)
	}
}

func TestSortRequest_Parse_InvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		engine  string
		input   string
		errText string
	}{
		{
			name:    "tesseract invalid json",
			engine:  "tesseract",
			input:   `invalid json{`,
			errText: "failed to parse Tesseract OCR JSON",
		},
		{
			name:    "easyocr invalid json",
			engine:  "easyocr",
			input:   `invalid json{`,
			errText: "failed to parse EasyOCR JSON",
		},
		{
			name:    "apple invalid json",
			engine:  "apple",
			input:   `invalid json{`,
			errText: "failed to parse Apple Vision OCR JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := SortRequest{
				Engine:     tt.engine,
				Lines:      []string{"test"},
				InputJson:  tt.input,
				PageWidth:  1920,
				PageHeight: 1080,
			}

			err := request.Parse()
			if err == nil {
				t.Fatal("Expected error for invalid JSON, got nil")
			}

			if !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("Expected error containing '%s', got: %v", tt.errText, err)
			}
		})
	}
}

func TestSortRequest_Parse_EmptyText(t *testing.T) {
	// Test that blocks with empty text are filtered out
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

	request := SortRequest{
		Engine:     "tesseract",
		Lines:      []string{"Valid"},
		InputJson:  input,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	err := request.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	blocks := request.Blocks()
	// Should only have 1 block (empty text filtered out)
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block (empty filtered), got %d", len(blocks))
	}

	if blocks[0].Text != "Valid" {
		t.Errorf("Expected text 'Valid', got '%s'", blocks[0].Text)
	}
}

func TestSortRequest_WithLogger(t *testing.T) {
	request := SortRequest{
		Engine:     "tesseract",
		Lines:      []string{"test"},
		InputJson:  `{"words":[]}`,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	// Test that WithLogger returns the request (for chaining)
	returned := request.WithLogger(nil)
	if returned != &request {
		t.Error("WithLogger should return pointer to the same request")
	}
}

func TestSortRequest_Parse_ValidationError(t *testing.T) {
	// Test that Parse calls Validate and returns validation errors
	request := SortRequest{
		Engine:     "", // Invalid - missing engine
		Lines:      []string{"test"},
		InputJson:  `{}`,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	err := request.Parse()
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	_, ok := err.(*ValidationError)
	if !ok {
		t.Errorf("Expected ValidationError from Parse(), got %T", err)
	}
}

func TestSortRequest_Parse_InvalidEngine(t *testing.T) {
	// This should be caught by Validate, but test the default case
	request := SortRequest{
		Engine:     "unknown-engine",
		Lines:      []string{"test"},
		InputJson:  `{}`,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	err := request.Parse()
	if err == nil {
		t.Fatal("Expected error for unsupported engine, got nil")
	}
}

func TestSortRequest_Blocks(t *testing.T) {
	request := SortRequest{}

	// Test that Blocks() returns nil slice before parsing (internal field not initialized)
	blocks := request.Blocks()
	if len(blocks) != 0 {
		t.Errorf("Expected 0 blocks before parsing, got %d", len(blocks))
	}
}
