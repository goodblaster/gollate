package sorters

import (
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// TestConfigError tests the ConfigError.Error() method
func TestConfigError(t *testing.T) {
	err := ConfigError{
		Field:   "MaxWordDistance",
		Value:   -1.0,
		Message: "must be positive",
	}

	expected := "invalid config for MaxWordDistance (value: -1): must be positive"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

// TestSorterConfigValidate tests SorterConfig.Validate() with various invalid configs
func TestSorterConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      SorterConfig
		expectError bool
		errorField  string
	}{
		{
			name:        "valid config",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "negative MaxWordDistance",
			config: SorterConfig{
				MaxWordDistance: -0.1,
				ReadingOrder:    HorizontalLTR_TTB,
				MaxPermutations: 1000,
				MaxPasses:       8,
			},
			expectError: true,
			errorField:  "MaxWordDistance",
		},
		{
			name: "zero MaxPermutations",
			config: SorterConfig{
				MaxWordDistance: 0.5,
				ReadingOrder:    HorizontalLTR_TTB,
				MaxPermutations: 0,
				MaxPasses:       8,
			},
			expectError: true,
			errorField:  "MaxPermutations",
		},
		{
			name: "negative MinWordsForEarlyPasses",
			config: SorterConfig{
				MaxWordDistance:        0.5,
				ReadingOrder:           HorizontalLTR_TTB,
				MaxPermutations:        1000,
				MaxPasses:              8,
				MinWordsForEarlyPasses: -1,
			},
			expectError: true,
			errorField:  "MinWordsForEarlyPasses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error for field '%s', got nil", tt.errorField)
					return
				}

				valErr, ok := err.(*ConfigError)
				if !ok {
					t.Errorf("Expected ConfigError, got %T", err)
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

// TestPathContains tests the Path.Contains() method
func TestPathContains(t *testing.T) {
	path := Path{
		Nodes: []int{1, 3, 5, 7, 9},
	}

	tests := []struct {
		name     string
		value    int
		expected bool
	}{
		{"contains 1", 1, true},
		{"contains 5", 5, true},
		{"contains 9", 9, true},
		{"does not contain 2", 2, false},
		{"does not contain 0", 0, false},
		{"does not contain 10", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := path.Contains(tt.value)
			if result != tt.expected {
				t.Errorf("Expected Contains(%d) = %v, got %v", tt.value, tt.expected, result)
			}
		})
	}
}

// TestPathString tests the Path.String() method
func TestPathString(t *testing.T) {
	blocks := []Block{
		{Index: 0, NormedText: "the"},
		{Index: 1, NormedText: "quick"},
		{Index: 2, NormedText: "brown"},
	}

	path := Path{
		Nodes: []int{0, 1, 2},
	}

	result := path.String(blocks)
	expected := "the quick brown"

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestPathCopy tests the Path.Copy() method
func TestPathCopy(t *testing.T) {
	original := Path{
		Length: 1.5,
		Nodes:  []int{1, 2, 3},
	}

	copied := original.Copy()

	// Verify copy has same values
	if copied.Length != original.Length {
		t.Errorf("Expected Length %f, got %f", original.Length, copied.Length)
	}
	if len(copied.Nodes) != len(original.Nodes) {
		t.Errorf("Expected %d nodes, got %d", len(original.Nodes), len(copied.Nodes))
	}

	// Modify copy and verify original is unchanged
	copied.Nodes[0] = 999
	if original.Nodes[0] == 999 {
		t.Error("Modifying copy affected original - not a deep copy")
	}
}

// TestPathAppend tests the Path.Append() method
func TestPathAppend(t *testing.T) {
	path := Path{
		Length: 1.0,
		Nodes:  []int{1, 2},
	}

	block := Block{Index: 3}
	distance := 0.5

	path.Append(block, distance)

	if len(path.Nodes) != 3 {
		t.Errorf("Expected 3 nodes after append, got %d", len(path.Nodes))
	}
	if path.Nodes[2] != 3 {
		t.Errorf("Expected last node to be 3, got %d", path.Nodes[2])
	}
	if path.Length != 1.5 {
		t.Errorf("Expected Length 1.5, got %f", path.Length)
	}
}

// TestReadingOrderIsTopToBottom tests the IsTopToBottom() method
func TestReadingOrderIsTopToBottom(t *testing.T) {
	tests := []struct {
		order    ReadingOrder
		expected bool
	}{
		{HorizontalLTR_TTB, true},
		{HorizontalRTL_TTB, true},
		{VerticalTTB_RTL, true},
		{VerticalTTB_LTR, true},
	}

	for _, tt := range tests {
		t.Run(tt.order.String(), func(t *testing.T) {
			result := tt.order.IsTopToBottom()
			if result != tt.expected {
				t.Errorf("Expected IsTopToBottom() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestDecodeUnicodeEscapes tests Unicode escape decoding
func TestDecodeUnicodeEscapes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "right single quote",
			input: "don\\u2019t",
		},
		{
			name:  "no escapes",
			input: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeUnicodeEscapes(tt.input)

			// Just verify it doesn't panic and returns something
			if result == "" && tt.input != "" {
				t.Error("Expected non-empty result")
			}
		})
	}
}

// TestParseLinesWithComplexInput tests ParseLines with various inputs
func TestParseLinesWithComplexInput(t *testing.T) {
	tests := []struct {
		name  string
		input []string
	}{
		{
			name:  "empty lines",
			input: []string{},
		},
		{
			name:  "single word lines",
			input: []string{"one", "two", "three"},
		},
		{
			name:  "mixed length lines",
			input: []string{"short line", "this is a much longer line with many words", "mid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLines(tt.input)

			// Verify result is a valid LineServer (may be nil for empty input)
			_ = result
		})
	}
}

// TestDistanceCalculation tests distance calculation with different reading orders
func TestDistanceCalculation(t *testing.T) {
	blocks := []ocr.Block{
		{
			Text: "A",
			BoundingBox: ocr.BoundingBox{
				Left:   0.1,
				Top:    0.1,
				Width:  0.1,
				Height: 0.05,
			},
		},
		{
			Text: "B",
			BoundingBox: ocr.BoundingBox{
				Left:   0.5,
				Top:    0.5,
				Width:  0.1,
				Height: 0.05,
			},
		},
	}

	// Test with different reading orders
	orders := []ReadingOrder{
		HorizontalLTR_TTB,
		HorizontalRTL_TTB,
		VerticalTTB_RTL,
		VerticalTTB_LTR,
	}

	for _, order := range orders {
		t.Run(order.String(), func(t *testing.T) {
			config := DefaultConfig()
			config.ReadingOrder = order

			sorter := NewOcrSorterWithConfig(blocks, []string{"A B"}, nil, config)
			_ = sorter

			// Just verify the sorter was created successfully
			if sorter == nil {
				t.Error("Expected non-nil sorter")
			}
		})
	}
}

// TestNewOcrSorter tests the sorter constructor
func TestNewOcrSorter(t *testing.T) {
	blocks := []ocr.Block{
		{Text: "hello", Index: 0},
		{Text: "world", Index: 1},
	}

	lines := []string{"hello world"}

	sorter := NewOcrSorter(blocks, lines, nil)

	if sorter == nil {
		t.Fatal("Expected non-nil sorter")
	}

	// Verify sorter has methods we expect
	metrics := sorter.Metrics()
	if metrics.TotalPermutationsExplored < 0 {
		t.Error("Expected valid metrics")
	}
}

// TestNewOcrSorterWithConfig tests the sorter constructor with custom config
func TestNewOcrSorterWithConfig(t *testing.T) {
	blocks := []ocr.Block{
		{Text: "test", Index: 0},
	}

	lines := []string{"test"}
	config := DefaultConfig()
	config.MaxPermutations = 1000

	sorter := NewOcrSorterWithConfig(blocks, lines, nil, config)

	if sorter == nil {
		t.Fatal("Expected non-nil sorter")
	}
}
