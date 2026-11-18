package multipage

import (
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/language"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// layout places lines of words on a page, one visual line per string,
// left to right from the given top, and returns the blocks in emit order.
func layout(top float64, lines ...string) []ocr.Block {
	var blocks []ocr.Block
	for row, line := range lines {
		left := 0.10
		for _, text := range strings.Fields(line) {
			width := 0.012 * float64(len([]rune(text)))
			blocks = append(blocks, ocr.Block{
				Text: text, Extractor: "test", Index: len(blocks),
				BoundingBox: ocr.BoundingBox{Left: left, Top: top + 0.04*float64(row), Width: width, Height: 0.02},
			})
			left += width + 0.012
		}
	}
	return blocks
}

func TestPageBreakAnotherKeepsBothLocations(t *testing.T) {
	canonical := []string{"An observer receives anything from another; other choices remain."}
	pages := []Page{page(layout(.4, "An observer receives anything from an-")), page(layout(.1, "other; other choices remain."))}
	head := pages[0].Blocks[len(pages[0].Blocks)-1].BoundingBox
	tail := pages[1].Blocks[0].BoundingBox
	r, err := Sort(pages, canonical, sorters.ConfigForLanguage("english"), logger.Noop{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Document.Text != canonical[0] {
		t.Fatalf("got %q, want %q", r.Document.Text, canonical[0])
	}
	found := false
	for _, p := range r.Document.Paragraphs {
		for _, tok := range p.Tokens {
			if r.Document.Text[tok.Span.Start:tok.Span.End] != "another;" {
				continue
			}
			found = true
			if tok.Canonical == nil || canonical[0][tok.Canonical.Start:tok.Canonical.End] != "another;" {
				t.Fatalf("missing canonical span: %+v", tok)
			}
			if tok.Page != 0 || tok.Bounds != head || tok.Continuation == nil || tok.Continuation.Page != 1 || tok.Continuation.Bounds != tail {
				t.Fatalf("lost cross-page locations: %+v", tok)
			}
		}
	}
	if !found {
		t.Fatal("another was not emitted as one word")
	}
}

func TestPageBreakPreservesCanonicalCompound(t *testing.T) {
	canonical := []string{"A long-time friend met a longtime colleague."}
	pages := []Page{page(layout(.4, "A long-")), page(layout(.1, "time friend met a longtime colleague."))}
	r, err := Sort(pages, canonical, sorters.ConfigForLanguage("english"), logger.Noop{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Document.Text != canonical[0] {
		t.Fatalf("changed a real compound: got %q, want %q", r.Document.Text, canonical[0])
	}
}

// at places a single run of text at an exact position (page furniture).
func at(text string, left, top float64) []ocr.Block {
	var blocks []ocr.Block
	for _, word := range strings.Fields(text) {
		width := 0.012 * float64(len(word))
		blocks = append(blocks, ocr.Block{
			Text: word, Extractor: "test",
			BoundingBox: ocr.BoundingBox{Left: left, Top: top, Width: width, Height: 0.015},
		})
		left += width + 0.012
	}
	return blocks
}

func page(parts ...[]ocr.Block) Page {
	var blocks []ocr.Block
	for _, part := range parts {
		blocks = append(blocks, part...)
	}
	for i := range blocks {
		blocks[i].Index = i
	}
	return Page{Blocks: blocks, Width: 800, Height: 1200}
}

func paragraphTexts(doc *Document) []string {
	var out []string
	for _, p := range doc.Paragraphs {
		out = append(out, doc.Text[p.Span.Start:p.Span.End])
	}
	return out
}

func TestSortJoinsParagraphAcrossPageBreak(t *testing.T) {
	canonical := []string{
		"The harbor was quiet that morning and nobody spoke.",
		"",
		"Nora counted eleven vessels between dusk and dawn, wrote each name in the green ledger, and then closed it.",
		"",
		"By noon the ferry had gone.",
	}
	pages := []Page{
		page(
			at("Carrow Light", 0.40, 0.03),
			layout(0.10,
				"The harbor was quiet that morning and nobody spoke.",
				"Nora counted eleven vessels between dusk and dawn,"),
			at("1", 0.50, 0.95),
		),
		page(
			at("Carrow Light", 0.40, 0.03),
			layout(0.10,
				"wrote each name in the green ledger, and then closed it.",
				"By noon the ferry had gone."),
			at("2", 0.50, 0.95),
		),
	}

	result, err := Sort(pages, canonical, sorters.ConfigForLanguage("english"), logger.Noop{})
	if err != nil {
		t.Fatal(err)
	}
	got := paragraphTexts(result.Document)
	want := []string{canonical[0], canonical[2], canonical[4]}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("paragraphs:\n got %q\nwant %q", got, want)
	}

	// The straddling paragraph is one paragraph whose tokens change page
	// exactly once, at the break.
	straddler := result.Document.Paragraphs[1]
	if straddler.CanonicalLine == nil || *straddler.CanonicalLine != 2 {
		t.Fatalf("straddler canonical line = %v, want 2", straddler.CanonicalLine)
	}
	var pagesSeen []int
	for _, tok := range straddler.Tokens {
		if len(pagesSeen) == 0 || pagesSeen[len(pagesSeen)-1] != tok.Page {
			pagesSeen = append(pagesSeen, tok.Page)
		}
	}
	if len(pagesSeen) != 2 || pagesSeen[0] != 0 || pagesSeen[1] != 1 {
		t.Errorf("straddler token pages = %v, want [0 1]", pagesSeen)
	}
	// Canonical spans are relative to the whole line on both sides of
	// the break, not to the part each page was offered.
	for _, tok := range straddler.Tokens {
		text := result.Document.Text[tok.Span.Start:tok.Span.End]
		if tok.Canonical == nil || canonical[2][tok.Canonical.Start:tok.Canonical.End] != text {
			t.Errorf("token %q (page %d): canonical span %v does not select it", text, tok.Page, tok.Canonical)
		}
	}
	first, last := straddler.Tokens[0], straddler.Tokens[len(straddler.Tokens)-1]
	if result.Document.Text[first.Span.Start:first.Span.End] != "Nora" || result.Document.Text[last.Span.Start:last.Span.End] != "it." {
		t.Errorf("token spans do not index the merged text")
	}

	// Furniture is reported per page and kept out of the text.
	for p, info := range result.Document.Pages {
		if len(info.Furniture) != 2 {
			t.Errorf("page %d furniture = %+v, want running head and page number", p, info.Furniture)
		}
	}
	if strings.Contains(result.Document.Text, "Carrow Light") {
		t.Errorf("running head leaked into the text")
	}
	if len(result.Unhandled) != 0 {
		t.Errorf("unhandled = %q", result.Unhandled)
	}
	if a := result.Assignments[2]; !a.Straddle || a.Page != 0 {
		t.Errorf("assignment of the straddling line = %+v", a)
	}
}

func TestSortJoinsWordHyphenatedAcrossPages(t *testing.T) {
	canonical := []string{
		"She stood two hundred yards from the tower behind a mortar wall.",
	}
	pages := []Page{
		page(layout(0.10, "She stood two hundred yards from the tow‐")),
		page(layout(0.10, "er behind a mortar wall.")),
	}
	result, err := Sort(pages, canonical, sorters.ConfigForLanguage("english"), logger.Noop{})
	if err != nil {
		t.Fatal(err)
	}
	if got := paragraphTexts(result.Document); len(got) != 1 || got[0] != canonical[0] {
		t.Fatalf("got %q, want %q", got, canonical)
	}

	// The re-joined word sits on page 0 and points at its second half on
	// page 1; every token maps to its own word of the canonical line.
	doc := result.Document
	for _, tok := range doc.Paragraphs[0].Tokens {
		text := doc.Text[tok.Span.Start:tok.Span.End]
		if tok.Canonical == nil || canonical[0][tok.Canonical.Start:tok.Canonical.End] != text {
			t.Errorf("token %q: canonical span %v does not select it", text, tok.Canonical)
		}
		if text == "tower" {
			if tok.Page != 0 || tok.Continuation == nil || tok.Continuation.Page != 1 {
				t.Errorf("tower: page %d, continuation %+v; want page 0 continuing on page 1", tok.Page, tok.Continuation)
			}
		} else if tok.Continuation != nil {
			t.Errorf("token %q should have no continuation", text)
		}
	}
}

func TestStripFurnitureLeavesBodyAlone(t *testing.T) {
	// Body text starts at the same height on every page; differing text
	// there is not furniture. A lone page with a number is not either.
	pages := []Page{
		page(layout(0.10, "alpha bravo charlie", "delta echo foxtrot")),
		page(layout(0.10, "golf hotel india", "juliet kilo lima")),
	}
	for p, f := range stripFurniture(pages) {
		if len(f) != 0 {
			t.Errorf("page %d: body text taken as furniture: %+v", p, f)
		}
	}
	if len(pages[0].Blocks) != 6 || len(pages[1].Blocks) != 6 {
		t.Errorf("blocks removed from body")
	}

	single := []Page{page(layout(0.10, "alpha bravo"), at("7", 0.5, 0.95))}
	if f := stripFurniture(single); len(f[0]) != 0 {
		t.Errorf("single page: nothing to compare against, got %+v", f[0])
	}

	// Roman and arabic numerals in the same place are both page labels.
	numbered := []Page{
		page(layout(0.10, "alpha bravo charlie"), at("xii", 0.5, 0.95)),
		page(layout(0.10, "delta echo foxtrot"), at("1", 0.5, 0.95)),
	}
	for p, f := range stripFurniture(numbered) {
		if len(f) != 1 {
			t.Errorf("page %d: page label not recognized: %+v", p, f)
		}
	}
}

func TestCanonicalFragment(t *testing.T) {
	cases := []struct {
		text, lang string
		start, end int
		want       string
		offset     int
	}{
		{"a four-block stretch of road", "english", 0, 2, "a four-", 0},
		{"a four-block stretch of road", "english", 2, 6, "block stretch of road", 7},
		{"朝は五時に起こされた。 Tokyo", "japanese", 0, 3, "朝は五", 0},
		{"  alpha\u00a0beta gamma", "english", 1, 3, "beta gamma", 9},
	}
	for _, c := range cases {
		got, offset, ok := canonicalFragment(c.text, Fragment{Start: c.start, End: c.end}, language.ForLanguage(c.lang, c.text))
		if !ok || got != c.want || offset != c.offset {
			t.Errorf("%q: got %q at %d (%v), want %q at %d", c.text, got, offset, ok, c.want, c.offset)
		}
	}
}
