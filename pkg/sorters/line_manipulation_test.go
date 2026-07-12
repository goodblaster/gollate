package sorters

import (
	"reflect"
	"testing"
)

func TestSplitParagraphQuotedSentenceBoundary(t *testing.T) {
	// The regression: a sentence ending in a closing quote must be a split
	// point, so quote-heavy legal text breaks into matchable sentences.
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "closing single quote",
			in:   "A 22-year-old document is 'ancient.' A 17-year-old is an 'infant.'",
			want: []string{"A 22-year-old document is 'ancient.'", "A 17-year-old is an 'infant.'"},
		},
		{
			name: "closing double quote",
			in:   `He said "stop." Then she left.`,
			want: []string{`He said "stop."`, "Then she left."},
		},
		{
			name: "plain boundary still works",
			in:   "First sentence. Second sentence.",
			want: []string{"First sentence.", "Second sentence."},
		},
		{
			name: "abbreviation is not a boundary",
			in:   "Dr. Smith arrived. He was late.",
			want: []string{"Dr. Smith arrived.", "He was late."},
		},
		{
			name: "no boundary returns whole",
			in:   "one two three four",
			want: []string{"one two three four"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitParagraph(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("SplitParagraph(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}

func TestEndsWithSentencePunctuationIgnoresClosingQuotes(t *testing.T) {
	yes := []string{"an 'infant.'", `he said "stop."`, "done.", "really?", "wow!'"}
	no := []string{"Dr.", "U.S.", "unfinished", "a 'quote'"}
	for _, s := range yes {
		if !endsWithSentencePunctuation(s) {
			t.Errorf("endsWithSentencePunctuation(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if endsWithSentencePunctuation(s) {
			t.Errorf("endsWithSentencePunctuation(%q) = true, want false", s)
		}
	}
}
