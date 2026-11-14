package sorters

import (
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
)

func vocab(words ...string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range words {
		m[w] = true
	}
	return m
}

func TestSplitCompoundText(t *testing.T) {
	tests := []struct {
		text  string
		vocab []string
		want  []string // "text|separator" per part; nil = not split
	}{
		{"four-block", []string{"four", "block"}, []string{"four|-", "block|"}},
		{"$4.2", []string{"$4", "2"}, []string{"$4|.", "2|"}},
		{"6:30", []string{"6", "30"}, []string{"6|:", "30|"}},
		{"12,000", []string{"12", "000"}, []string{"12|,", "000|"}},
		{"(street-closure)", []string{"street", "closure"}, []string{"(street|-", "closure)|"}},
		{"three-quarters,", []string{"three", "quarters"}, []string{"three|-", "quarters,|"}},
		// A part missing from canonical text: leave the block whole.
		{"four-block", []string{"four"}, nil},
	}
	for _, tt := range tests {
		tokens := strings.Fields(NormalizeText(tt.text))
		parts := splitCompoundText(tt.text, tokens, vocab(tt.vocab...))
		var got []string
		for _, p := range parts {
			got = append(got, p.text+"|"+p.separator)
		}
		if strings.Join(got, " ") != strings.Join(tt.want, " ") {
			t.Errorf("splitCompoundText(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

// Tokens NormalizeText keeps whole must never be split.
func TestCompoundSplitLeavesSingleTokensAlone(t *testing.T) {
	lines := ParseLines([]string{"call 1-800-555-1234 at 9 a.m. in the U.S. today"})
	for _, text := range []string{"1-800-555-1234", "a.m.", "U.S.", "today."} {
		blocks := normalizeOcrBlocks([]Block{{Text: text, Extractor: "test"}}, true, lines)
		if len(blocks) != 1 || blocks[0].Text != text {
			t.Errorf("%q was split or altered: %+v", text, blocks)
		}
	}
}

func word(text string, left, top, width float64) Block {
	return Block{
		Text: text, Extractor: "test", PageWidth: 1000, PageHeight: 1000,
		BoundingBox: ocr.BoundingBox{Left: left, Top: top, Width: width, Height: 0.02},
	}
}

// Compound words match token by token, and the output restores the
// separators that splitting removed.
func TestCompoundWordsMatchAndRoundTrip(t *testing.T) {
	texts := []string{"About", "$4.2", "million", "by", "6:30", "for", "the", "four-block", "plan"}
	var blocks []Block
	left := 0.05
	for i, text := range texts {
		width := 0.012 * float64(len(text))
		b := word(text, left, 0.10, width)
		b.Index = i
		blocks = append(blocks, b)
		left += width + 0.01
	}
	canonical := []string{"About $4.2 million by 6:30 for the four-block plan"}

	sorter := NewOcrSorterWithConfig(blocks, canonical, logger.Noop{}, ConfigForLanguage("english"))
	if _, err := sorter.Sort(); err != nil {
		t.Fatal(err)
	}
	if got := sorter.Document().Text; got != canonical[0] {
		t.Errorf("document text = %q, want %q", got, canonical[0])
	}
	if got := strings.Join(sorter.SortedLines(), "\n"); strings.TrimSpace(got) != canonical[0] {
		t.Errorf("sorted lines = %q, want %q", got, canonical[0])
	}
	if m := sorter.Metrics(); m.LinesFound != 1 || m.HolesBridged != 0 {
		t.Errorf("expected one clean match, got LinesFound=%d HolesBridged=%d", m.LinesFound, m.HolesBridged)
	}
}

func TestAttachPunctuation(t *testing.T) {
	// `downtown.` `"` `Her` - the closing quote sits flush against the word
	// before it and a space away from the word after.
	blocks := []Block{
		word("come", 0.10, 0.10, 0.04),
		word("downtown.", 0.15, 0.10, 0.09),
		word(`"`, 0.24, 0.10, 0.008),
		word("Her", 0.26, 0.10, 0.03),
		word("(", 0.30, 0.10, 0.005), // opening: joins the next word
		word("not", 0.305, 0.10, 0.03),
		word("—", 0.60, 0.50, 0.02), // stranded: no word nearby, dropped
	}
	out := normalizeOcrBlocks(blocks, false, nil)
	var got []string
	for _, b := range out {
		got = append(got, b.Text)
	}
	want := []string{"come", `downtown."`, "Her", "(not"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got %q, want %q", got, want)
	}
	// Geometry is untouched: the word keeps its own box.
	if out[1].BoundingBox.Width != 0.09 {
		t.Errorf("word box changed: %+v", out[1].BoundingBox)
	}
}
