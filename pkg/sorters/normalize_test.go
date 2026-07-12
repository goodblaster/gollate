package sorters

import (
	"fmt"
	"testing"
)

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "no change",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "trim",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "newlines",
			input:    "hello\nworld",
			expected: "hello world",
		},
		{
			name:     "tabs",
			input:    "hello\tworld",
			expected: "hello world",
		},
		{
			name:     "multiple spaces",
			input:    "hello  world",
			expected: "hello world",
		},
		{
			name:     "leading space",
			input:    " hello world",
			expected: "hello world",
		},
		{
			name:     "trailing space",
			input:    "hello world ",
			expected: "hello world",
		},
		{
			name:     "leading and trailing space",
			input:    " hello world ",
			expected: "hello world",
		},
		{
			name:     "leading and trailing space with newlines",
			input:    " \nhello world\n ",
			expected: "hello world",
		},
		{
			name:     "inner apostrophe",
			input:    "Julie's",
			expected: "julies",
		},
		{
			name:     "outer apostrophe",
			input:    "'hello world'",
			expected: "hello world",
		},
		{
			name:     "inner hyphen",
			input:    "well-being",
			expected: "well being",
		},
		{
			name:     "underscore",
			input:    "hello_world",
			expected: "hello world",
		},
		{
			name:     "inner period",
			input:    "e.g.",
			expected: "e_g",
		},
		{
			name:     "complex 1",
			input:    "  This is Julie's well-meaning interpretation of Mr. Williams' lecture.  ",
			expected: "this is julies well meaning interpretation of mr williams lecture",
		},
		{
			name:     "complex 2",
			input:    "  This is Julie's well-meaning interpretation of Mr. Williams' lecture.  \n\n  It's a good one...  ",
			expected: "this is julies well meaning interpretation of mr williams lecture its a good one",
		},
		{
			name:     "unicode accents",
			input:    "Let's go to Café Juliet', mañana… ¿Está bien?",
			expected: "lets go to cafe juliet manana esta bien",
		},
		{
			name:     "chinese text with CJK punctuation",
			input:    "维基百科，自由的百科全书。欢迎大家！",
			expected: "维基百科 自由的百科全书 欢迎大家",
		},
		{
			name:     "mixed chinese and english",
			input:    "Apple公司（Apple Inc.）是一家科技公司。",
			expected: "apple公司 apple inc 是一家科技公司",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.name == "retain special characters" {
				fmt.Println("Test case: ", test.name)
			}
			actual := NormalizeText(test.input)
			if actual != test.expected {
				t.Errorf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}
