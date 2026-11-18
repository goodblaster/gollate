package sorters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/logger"
)

func TestHyphensWithInterleavedColumns(t *testing.T) {
	canonical := []string{"A helpful disposition makes working pleasant.", "Fresh consideration keeps everyone governed fairly."}
	// Engine emits row 1 left/right, then row 2 left/right. Both columns
	// contain split words, and the same suffix occurs in both columns.
	rows := []struct {
		text string
		x, y float64
	}{
		{"A helpful disposi-", .05, .1}, {"Fresh considera-", .55, .1},
		{"tion makes work-", .05, .15}, {"tion keeps everyone gov-", .55, .15},
		{"ing pleasant.", .05, .2}, {"erned fairly.", .55, .2},
	}
	var blocks []Block
	for row, r := range rows {
		x := r.x
		for _, text := range strings.Fields(r.text) {
			b := word(text, x, r.y, .012*float64(len(text)))
			b.Index, b.LineId = len(blocks), fmt.Sprint(row)
			blocks = append(blocks, b)
			x += b.Width() + .01
		}
	}
	for _, ids := range []bool{true, false} {
		input := append([]Block(nil), blocks...)
		if !ids {
			for i := range input {
				input[i].LineId = ""
			}
		}
		s := NewOcrSorterWithConfig(input, canonical, logger.Noop{}, ConfigForLanguage("english"))
		if _, err := s.Sort(); err != nil {
			t.Fatal(err)
		}
		doc := s.Document()
		if doc.Text != strings.Join(canonical, "\n") {
			t.Fatalf("line IDs %v: %q", ids, doc.Text)
		}
		joined := 0
		for _, p := range doc.Paragraphs {
			for _, tok := range p.Tokens {
				text := doc.Text[tok.Span.Start:tok.Span.End]
				if tok.Canonical == nil {
					t.Fatalf("unlinked %q", text)
				}
				if tok.Continuation != nil {
					joined++
					if (tok.Bounds.Left < .5) != (tok.Continuation.Bounds.Left < .5) {
						t.Fatalf("joined across columns: %q", text)
					}
				}
			}
		}
		if joined != 4 {
			t.Fatalf("joined %d words, want 4", joined)
		}
	}
}

func TestWrappedHyphenDoesNotSkipInterveningLine(t *testing.T) {
	blocks := []Block{word("A", .1, .1, .02), word("con-", .2, .1, .06), word("different", .1, .14, .1), word("text", .1, .18, .05)}
	for i := range blocks {
		blocks[i].Index = i
		blocks[i].LineId = fmt.Sprint(i)
	}
	out := normalizeOcrBlocks(blocks, true, ParseLines([]string{"A context different text"}))
	for _, b := range out {
		if b.Continuation != nil {
			t.Fatalf("skipped a physical line: %+v", b)
		}
	}
}

func TestHyphenContextDisambiguatesCommonParts(t *testing.T) {
	for _, tc := range []struct {
		canonical string
		join      bool
	}{
		{"Difficult because rain fell. It may be cause for concern.", true},
		{"Difficult because rain fell. Difficult be cause rain fell.", false},
	} {
		blocks := []Block{word("Difficult", .1, .1, .1), word("be-", .21, .1, .05), word("cause", .1, .14, .06), word("rain", .17, .14, .05), word("fell.", .23, .14, .05)}
		for i := range blocks {
			blocks[i].Index = i
			blocks[i].LineId = "first"
			if i >= 2 {
				blocks[i].LineId = "second"
			}
		}
		out := normalizeOcrBlocks(blocks, true, ParseLines([]string{tc.canonical}))
		joined := false
		for _, b := range out {
			if b.Text == "because" && b.Continuation != nil {
				joined = true
			}
		}
		if joined != tc.join {
			t.Fatalf("joined=%v, want %v for %q", joined, tc.join, tc.canonical)
		}
	}
}

func TestHyphenatedCompoundKeepsBothParts(t *testing.T) {
	blocks := []Block{word("great-un-", .7, .1, .14), word("cle", .1, .14, .04)}
	blocks[0].LineId, blocks[1].LineId = "a", "b"
	out := normalizeOcrBlocks(blocks, true, ParseLines([]string{"great-uncle"}))
	if len(out) != 2 || out[0].Text != "great" || out[0].HyphenAfter != "-" || out[1].Text != "uncle" || out[1].Continuation == nil || out[1].Continuation.Bounds != blocks[1].BoundingBox {
		t.Fatalf("lost compound or continuation: %+v", out)
	}
}

func TestMisreadContinuationDoesNotStealOtherColumnSuffix(t *testing.T) {
	blocks := []Block{
		word("Fresh", .1, .1, .08), word("considera-", .19, .1, .14),
		word("tion", .55, .1, .05), // emitted next, but in the other column
		word("ti0n", .1, .14, .05), word("matters", .16, .14, .08),
	}
	for i := range blocks {
		blocks[i].Index = i
		blocks[i].LineId = "left-top"
		if i == 2 {
			blocks[i].LineId = "right-top"
		}
		if i >= 3 {
			blocks[i].LineId = "left-next"
		}
	}
	out := normalizeOcrBlocks(blocks, true, ParseLines([]string{"Fresh consideration matters."}))
	for _, b := range out {
		if b.Continuation != nil {
			t.Fatalf("borrowed an unrelated suffix: %+v", b)
		}
	}
}
