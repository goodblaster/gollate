package sorters

import (
	"fmt"
	"strings"
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

// Ten or more URLs, emails or phone numbers in one string must each come
// back intact (placeholder 1 used to be a prefix of placeholder 10).
func TestNormalizeTextManyPlaceholders(t *testing.T) {
	var urls, emails, phones, wantPhones []string
	for i := 0; i < 12; i++ {
		urls = append(urls, fmt.Sprintf("https://example.com/page/%c", 'a'+i))
		emails = append(emails, fmt.Sprintf("user%c@example.com", 'a'+i))
		phones = append(phones, fmt.Sprintf("555-01%02d", i))
		wantPhones = append(wantPhones, fmt.Sprintf("555_01%02d", i))
	}
	for run := 0; run < 20; run++ { // map-order randomness made this flaky, not just wrong
		if got, want := NormalizeText(strings.Join(urls, " ")), strings.Join(urls, " "); got != want {
			t.Fatalf("urls:\n got %q\nwant %q", got, want)
		}
		if got, want := NormalizeText(strings.Join(emails, " ")), strings.Join(emails, " "); got != want {
			t.Fatalf("emails:\n got %q\nwant %q", got, want)
		}
		if got, want := NormalizeText(strings.Join(phones, " ")), strings.Join(wantPhones, " "); got != want {
			t.Fatalf("phones:\n got %q\nwant %q", got, want)
		}
	}

	// A URL that is a prefix of another must not clobber it.
	in := "https://example.com and https://example.com/more"
	if got := NormalizeText(in); got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}
