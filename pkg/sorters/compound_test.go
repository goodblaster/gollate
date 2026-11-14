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

// A visual line with a misread in it matches no canonical line exactly. It
// must stay between its neighbors rather than being emitted after
// everything else - the failure that made lightly noisy pages score below
// heavily noisy ones.
func TestMisreadLeftoverLineStaysInPlace(t *testing.T) {
	canonical := []string{
		"alpha bravo charlie delta echo",
		"foxtrot golf hotel india juliet",
		"kilo lima mike november oscar",
	}
	ocrLines := []string{
		"alpha bravo charlie delta echo",
		"foxtrot golf hotcl india juliet", // "hotel" misread
		"kilo lima mike november oscar",
	}
	var blocks []Block
	for row, line := range ocrLines {
		left := 0.05
		for _, text := range strings.Fields(line) {
			b := word(text, left, 0.10+0.05*float64(row), 0.08)
			b.Index = len(blocks)
			b.LineId = string(rune('a' + row))
			blocks = append(blocks, b)
			left += 0.09
		}
	}

	// DefaultConfig leaves these short lines to leftover assembly.
	sorter := NewOcrSorterWithConfig(blocks, canonical, logger.Noop{}, DefaultConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(sorter.SortedLines(), "\n")
	want := strings.Join(ocrLines, "\n")
	if strings.TrimSpace(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJoinLineBreakHyphens(t *testing.T) {
	lines := ParseLines([]string{"the vendors sell long-time favorites near the old well"})

	// "ven-" ends line a, "dors" starts line b.
	a1 := word("the", 0.70, 0.10, 0.04)
	a2 := word("ven‐", 0.75, 0.10, 0.05)
	b1 := word("dors", 0.10, 0.13, 0.05)
	b2 := word("sell", 0.16, 0.13, 0.04)
	// A real compound broken at its own hyphen: both halves are canonical
	// words, so it is left for compound handling.
	b3 := word("long-", 0.75, 0.13, 0.06)
	c1 := word("time", 0.10, 0.16, 0.05)
	// A hyphen in the middle of a line joins nothing.
	c2 := word("old-", 0.20, 0.16, 0.05)
	c3 := word("well", 0.26, 0.16, 0.05)

	out := normalizeOcrBlocks([]Block{a1, a2, b1, b2, b3, c1, c2, c3}, true, lines)
	var got []string
	for _, b := range out {
		got = append(got, b.Text+"="+b.NormedText)
	}
	want := []string{"the=the", "vendors=vendors", "sell=sell", "long-=long", "time=time", "old-=old", "well=well"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	joined := out[1]
	if joined.OriginalOcrText != "ven‐ dors" || joined.CorrectionType != "dehyphenated" {
		t.Errorf("provenance not recorded: %+v", joined)
	}
	if joined.BoundingBox.Left != 0.75 || joined.BoundingBox.Top != 0.10 {
		t.Errorf("joined block should keep the first part's position: %+v", joined.BoundingBox)
	}
}
