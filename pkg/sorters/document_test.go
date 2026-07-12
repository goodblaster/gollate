package sorters

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

func documentTestSorter(t *testing.T) *Sorter {
	t.Helper()
	canonical := []string{"aurora desk lamp", "", "cooling foam día"}
	mk := func(text string, index int, top, left float64) ocr.Block {
		return ocr.Block{
			Text: text, NormedText: NormalizeText(text), Extractor: "test", Index: index,
			PageWidth: 1000, PageHeight: 1000,
			BoundingBox: ocr.BoundingBox{Top: top, Left: left, Width: 0.06, Height: 0.02},
		}
	}
	blocks := []ocr.Block{
		mk("aurora", 0, 0.10, 0.05),
		mk("desk", 1, 0.10, 0.12),
		mk("lamp", 2, 0.10, 0.19),
		mk("cooling", 3, 0.15, 0.05),
		mk("foam", 4, 0.15, 0.12),
		mk("día", 5, 0.15, 0.19),
	}
	config := DefaultConfig()
	config.MinWordsForEarlyPasses = 3
	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	return sorter
}

func TestDocumentTextAndSpans(t *testing.T) {
	doc := documentTestSorter(t).Document()

	want := "aurora desk lamp\ncooling foam día"
	if doc.Text != want {
		t.Fatalf("Text = %q, want %q", doc.Text, want)
	}
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("got %d paragraphs, want 2", len(doc.Paragraphs))
	}

	// Every span must slice Text to exactly its own content.
	for pi, p := range doc.Paragraphs {
		if got := doc.Text[p.Span.Start:p.Span.End]; got != p.String() {
			t.Errorf("paragraph %d: span slice %q != String() %q", pi, got, p.String())
		}
		for ti, tok := range p.Tokens {
			got := doc.Text[tok.Span.Start:tok.Span.End]
			if got != tok.String() || strings.Contains(got, " ") || got == "" {
				t.Errorf("paragraph %d token %d: span slices to %q", pi, ti, got)
			}
		}
	}

	// Multi-byte safety: the last token is "día" (4 bytes, 3 runes).
	last := doc.Paragraphs[1].Tokens[2]
	if last.String() != "día" || last.Span.End-last.Span.Start != len("día") {
		t.Errorf("multi-byte token: String()=%q span width=%d", last.String(), last.Span.End-last.Span.Start)
	}
}

func TestDocumentParagraphBounds(t *testing.T) {
	doc := documentTestSorter(t).Document()

	b := doc.Paragraphs[0].Bounds
	if b.Top != 0.10 || b.Left != 0.05 {
		t.Errorf("paragraph bounds origin = (%v, %v), want (0.10, 0.05)", b.Top, b.Left)
	}
	// Three 0.06-wide words starting at 0.05, 0.12, 0.19: right edge 0.25.
	if right := b.Left + b.Width; right < 0.249 || right > 0.251 {
		t.Errorf("paragraph right edge = %v, want 0.25", right)
	}
}

func TestDocumentJSONHasNoTextDuplication(t *testing.T) {
	doc := documentTestSorter(t).Document()

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	// The blob appears once; tokens and paragraphs reference spans only,
	// so no token word may appear as a JSON string value elsewhere.
	if got := strings.Count(string(data), "aurora"); got != 1 {
		t.Errorf("token text appears %d times in JSON, want exactly 1 (in the blob)", got)
	}

	// Round-trip: spans still address the blob correctly.
	var back Document
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	tok := back.Paragraphs[0].Tokens[0]
	if back.Text[tok.Span.Start:tok.Span.End] != "aurora" {
		t.Errorf("round-tripped span does not address the blob")
	}
}

func TestDocumentBeforeSortIsEmpty(t *testing.T) {
	sorter := NewOcrSorterWithConfig(nil, []string{"x"}, nil, DefaultConfig())
	doc := sorter.Document()
	if doc.Text != "" || len(doc.Paragraphs) != 0 {
		t.Errorf("expected empty document before Sort, got %+v", doc)
	}
}
