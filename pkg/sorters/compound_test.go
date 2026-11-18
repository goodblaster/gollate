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

func TestJoinAmbiguousWordParts(t *testing.T) {
	for _, tt := range []struct {
		name, canonical, head, tail, want string
		sameLine                          bool
	}{
		{"another", "an example from another; other choices remain", "an-", "other;", "another;", false},
		{"unicode hyphen", "an example from another; other choices remain", "an‐", "other;", "another;", false},
		{"compound also has joined form", "a long-time friend and a longtime colleague", "long-", "time", "long- time", false},
		{"compound across canonical lines", "a long-\ntime friend and a longtime colleague", "long-", "time", "long- time", false},
		{"separate phrase also has joined form", "some body was found by somebody", "some-", "body", "some- body", false},
		{"joined form absent", "an example with other choices", "an-", "other", "an- other", false},
		{"no line break", "an example from another; other choices remain", "an-", "other;", "an- other;", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			head, tail := word(tt.head, .8, .1, .06), word(tt.tail, .1, .14, .08)
			head.LineId, tail.LineId = "first", "second"
			if tt.sameLine {
				tail = word(tt.tail, .87, .1, .08)
				tail.LineId = "first"
			}
			out := normalizeOcrBlocks([]Block{head, tail}, true, ParseLines(strings.Split(tt.canonical, "\n")))
			var text []string
			for _, b := range out {
				text = append(text, b.Text)
			}
			if got := strings.Join(text, " "); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Both halves are common words elsewhere in the same canonical paragraph.
// This reproduces Meditations' an-/other without depending on the download.
func TestLineBreakAnotherKeepsBothLocations(t *testing.T) {
	canonical := "An observer receives anything from another; other choices remain."
	var blocks []Block
	for row, line := range []string{"An observer receives anything from an-", "other; other choices remain."} {
		left := .1
		for _, text := range strings.Fields(line) {
			b := word(text, left, .1+.04*float64(row), .012*float64(len(text)))
			b.Index = len(blocks)
			b.LineId = string(rune('a' + row))
			blocks = append(blocks, b)
			left += b.Width() + .01
		}
	}
	head, tail := blocks[5].BoundingBox, blocks[6].BoundingBox
	s := NewOcrSorterWithConfig(blocks, []string{canonical}, logger.Noop{}, ConfigForLanguage("english"))
	if _, err := s.Sort(); err != nil {
		t.Fatal(err)
	}
	doc := s.Document()
	if doc.Text != canonical {
		t.Fatalf("got %q, want %q", doc.Text, canonical)
	}
	found := false
	for _, p := range doc.Paragraphs {
		for _, tok := range p.Tokens {
			if doc.Text[tok.Span.Start:tok.Span.End] != "another;" {
				continue
			}
			found = true
			if tok.Canonical == nil || canonical[tok.Canonical.Start:tok.Canonical.End] != "another;" {
				t.Fatalf("missing canonical span: %+v", tok)
			}
			if tok.Bounds != head || tok.Continuation == nil || tok.Continuation.Bounds != tail {
				t.Fatalf("lost source locations: %+v", tok)
			}
		}
	}
	if !found {
		t.Fatal("another was not emitted as one word")
	}
}

// Vertical text whose columns arrive left to right is put back in reading
// order (right to left), and each column becomes its own line.
func TestReverseLeftToRightColumns(t *testing.T) {
	column := func(text string, left float64) []Block {
		var out []Block
		for i, r := range []rune(text) {
			out = append(out, Block{
				Text: string(r), NormedText: string(r), Extractor: "test",
				BoundingBox: ocr.BoundingBox{Left: left, Top: 0.10 + 0.03*float64(i), Width: 0.03, Height: 0.025},
			})
		}
		return out
	}
	text := func(blocks []Block) string {
		var sb strings.Builder
		for _, b := range blocks {
			sb.WriteString(b.Text)
		}
		return sb.String()
	}
	number := func(blocks []Block) []Block {
		for i := range blocks {
			blocks[i].Index = i
		}
		return blocks
	}

	// Reading order is あいうえ / かきくけ / さしすせ / たちつて, rightmost first.
	cols := [][]Block{column("あいうえ", 0.70), column("かきくけ", 0.60), column("さしすせ", 0.50), column("たちつて", 0.40)}

	var reversed []Block // as a paginated PDF text layer emitted them
	for i := len(cols) - 1; i >= 0; i-- {
		reversed = append(reversed, cols[i]...)
	}
	got, changed := reverseLeftToRightColumns(number(reversed))
	if !changed || text(got) != "あいうえかきくけさしすせたちつて" {
		t.Errorf("changed=%v, text=%q", changed, text(got))
	}
	for i, b := range got {
		if b.Index != i {
			t.Fatalf("blocks not renumbered: %d has Index %d", i, b.Index)
		}
	}
	assignColumnLineIds(got)
	if got[0].LineId != got[3].LineId || got[3].LineId == got[4].LineId {
		t.Errorf("column line ids wrong: %q %q %q", got[0].LineId, got[3].LineId, got[4].LineId)
	}

	// Already right to left: left alone.
	var correct []Block
	for _, c := range cols {
		correct = append(correct, c...)
	}
	for i := range correct {
		correct[i].LineId = ""
	}
	if _, changed := reverseLeftToRightColumns(number(correct)); changed {
		t.Errorf("right-to-left columns were reversed")
	}
}
