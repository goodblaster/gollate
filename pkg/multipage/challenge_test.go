package multipage

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/language"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// Each part is independently positioned on its expected page. The truth
// records canonical byte offsets AND physical page/box, so identical words
// on different pages cannot earn credit by matching the wrong occurrence.
type challengePart struct {
	Line, Page int
	Text       string
	Top        float64
}
type challenge struct {
	Name, Language string
	Canonical      []string
	Parts          []challengePart
}
type truthToken struct {
	line, start, end, page int
	bounds                 ocr.BoundingBox
}

func challenges() []challenge {
	data, err := os.ReadFile("testdata/challenges.json")
	if err != nil {
		panic(err)
	}
	var cases []challenge
	if err := json.Unmarshal(data, &cases); err != nil {
		panic(err)
	}
	return cases
}

func challengeInput(c challenge) ([]Page, []truthToken) {
	h := language.ForLanguage(c.Language, c.Canonical...)
	var pages []Page
	var truth []truthToken
	cursor := make([]int, len(c.Canonical))
	for _, part := range c.Parts {
		for len(pages) <= part.Page {
			pages = append(pages, Page{Width: 800, Height: 1200})
		}
		p := &pages[part.Page]
		words := strings.Fields(part.Text)
		if c.Language == "japanese" {
			words = nil
			for _, r := range part.Text {
				if r != ' ' {
					words = append(words, string(r))
				}
			}
		}
		for col, w := range words {
			box := ocr.BoundingBox{Left: .05 + float64(col%12)*.07, Top: part.Top + float64(col/12)*.04, Width: .06, Height: .02}
			p.Blocks = append(p.Blocks, ocr.Block{Text: w, Extractor: "test", Index: len(p.Blocks), BoundingBox: box})
			key := sorters.NormalizeText(w)
			if len(h.Tokenize(key)) == 0 {
				continue
			}
			// Locate the original word (without punctuation) in the canonical
			// line, advancing from the previous part, not from the line's start.
			raw := strings.Trim(w, ".-‐。")
			relative := strings.Index(c.Canonical[part.Line][cursor[part.Line]:], raw)
			if relative < 0 {
				panic(fmt.Sprintf("invalid fixture %s: %q absent from canonical line %d", c.Name, raw, part.Line))
			}
			at := relative + cursor[part.Line]
			end := at + len(raw)
			truth = append(truth, truthToken{part.Line, at, end, part.Page, box})
			cursor[part.Line] = end
		}
	}
	return pages, truth
}

func challengeAccuracy(c challenge, r *Result, truth []truthToken) (float64, float64) {
	h := language.ForLanguage(c.Language, c.Canonical...)
	found := make(map[int]bool)
	for _, para := range r.Document.Paragraphs {
		if para.CanonicalLine == nil {
			continue
		}
		for _, tok := range para.Tokens {
			if tok.Canonical == nil {
				continue
			}
			for i, want := range truth {
				if *para.CanonicalLine == want.line && tok.Page == want.page && tok.Bounds.Left == want.bounds.Left && tok.Bounds.Top == want.bounds.Top && tok.Canonical.Start <= want.start && tok.Canonical.End >= want.end {
					found[i] = true
					break
				}
			}
		}
	}
	canonical := h.Tokenize(sorters.NormalizeText(strings.Join(c.Canonical, " ")))
	actual := h.Tokenize(sorters.NormalizeText(r.Document.Text))
	return 100 * float64(lcsCount(canonical, actual)) / float64(len(canonical)), 100 * float64(len(found)) / float64(len(truth))
}

func TestChallengeDocuments(t *testing.T) {
	for _, c := range challenges() {
		t.Run(c.Name, func(t *testing.T) {
			pages, truth := challengeInput(c)
			r, err := Sort(pages, c.Canonical, sorters.ConfigForLanguage(c.Language), logger.Noop{})
			if err != nil {
				t.Fatal(err)
			}
			recall, localized := challengeAccuracy(c, r, truth)
			t.Logf("text-order=%.2f%% exact-location=%.2f%% paragraphs=%d", recall, localized, len(r.Document.Paragraphs))
			if strings.Contains(c.Name, "compound-seam") {
				want := c.Canonical[0]
				if strings.HasPrefix(c.Name, "unicode-") {
					want = strings.ReplaceAll(want, "-", "‐")
				}
				if r.Document.Text != want {
					t.Errorf("compound formatting: got %q want %q", r.Document.Text, want)
				}
			}
			if recall != 100 || localized != 100 || len(r.Document.Paragraphs) != len(c.Canonical) {
				t.Errorf("incomplete recovery: text=%q assignments=%+v", r.Document.Text, r.Assignments)
			}
		})
	}
}

// 400 pages cross the production band threshold. Unique identifiers are
// interspersed with common words; canonical-only runs precede, interrupt,
// and follow the body. Three scan-only pages exercise drift in both directions.
func driftBook() (lines, pages [][]string, want []int) {
	absent := func(n int) {
		for i := 0; i < n; i++ {
			lines = append(lines, strings.Fields("unprinted license and editorial disclaimer"))
			want = append(want, -1)
		}
	}
	absent(1000)
	for p := 0; p < 400; p++ {
		pages = append(pages, nil)
		if p >= 200 && p < 203 {
			pages[p] = strings.Fields("scan only appendix illustration credits")
			continue
		}
		if p == 120 {
			absent(500)
		}
		for l := 0; l < 25; l++ {
			tokens := []string{"the", fmt.Sprintf("marker%d_%d", p, l), "and", fmt.Sprintf("reference%d_%d", p, l), "end"}
			lines = append(lines, tokens)
			pages[p] = append(pages[p], tokens...)
			want = append(want, p)
		}
	}
	absent(1000)
	return
}
func TestChallengeEditionDrift(t *testing.T) {
	lines, pages, want := driftBook()
	got := Segment(lines, pages)
	correct, total, absent := 0, 0, 0
	for i, p := range want {
		if p < 0 {
			if got[i].Page < 0 {
				absent++
			}
			continue
		}
		total++
		if got[i].Page == p {
			correct++
		}
	}
	t.Logf("correct-page=%.2f%% (%d/%d); unassigned-absent=%d/2500", 100*float64(correct)/float64(total), correct, total, absent)
	if correct != total || absent != 2500 {
		t.Errorf("edition drift displaced body or assigned absent lines")
	}
}

func TestChallengeMissingOccurrences(t *testing.T) {
	r, err := Sort(nil, []string{"missing twice", "missing twice"}, sorters.ConfigForLanguage("english"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Unhandled) != 2 {
		t.Fatalf("unhandled=%q, want both missing occurrences", r.Unhandled)
	}
}

func BenchmarkChallengeDrift(b *testing.B) {
	lines, pages, _ := driftBook()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Segment(lines, pages)
	}
}
func BenchmarkBook300(b *testing.B) {
	pages, lines := syntheticBook(20261113, 300)
	cfg := sorters.ConfigForLanguage("english")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Sort(pages, lines, cfg, logger.Noop{}); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkChallengeDocuments(b *testing.B) {
	for _, c := range challenges() {
		b.Run(c.Name, func(b *testing.B) {
			pages, _ := challengeInput(c)
			cfg := sorters.ConfigForLanguage(c.Language)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Sort(pages, c.Canonical, cfg, logger.Noop{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A long paragraph with repeated common words crosses five pages. Real
// misreads and dropped words are introduced AFTER ground truth is built.
// Surviving words must point at their original occurrence, including on
// pages after a completely blank sheet.
func TestChallengeNoisyLongParagraph(t *testing.T) {
	c := challenge{Name: "noisy-long-paragraph", Language: "english"}
	var all []string
	for p := 0; p < 5; p++ {
		var words []string
		for w := 0; w < 60; w++ {
			word := fmt.Sprintf("word%d", p*60+w)
			if w%4 == 0 {
				word = "the"
			}
			words = append(words, word)
		}
		physicalPage := p
		if p >= 2 {
			physicalPage++
		}
		c.Parts = append(c.Parts, challengePart{Line: 0, Page: physicalPage, Text: strings.Join(words, " "), Top: .1})
		all = append(all, words...)
	}
	c.Canonical = []string{strings.Join(all, " ")}
	pages, truth := challengeInput(c)
	for p := range pages {
		if len(pages[p].Blocks) == 0 {
			continue
		}
		pages[p].Blocks[15].Text = "misread"
		pages[p].Blocks = append(pages[p].Blocks[:30], pages[p].Blocks[31:]...)
	}
	r, err := Sort(pages, c.Canonical, sorters.ConfigForLanguage(c.Language), nil)
	if err != nil {
		t.Fatal(err)
	}
	recall, localized := challengeAccuracy(c, r, truth)
	t.Logf("300 canonical words, 5 misreads, 5 drops: text-order=%.2f%% exact-location=%.2f%%", recall, localized)
	ceiling := 100 * 290.0 / 300
	if recall < ceiling-1e-6 || localized < ceiling-1e-6 || len(r.Document.Paragraphs) != 1 {
		t.Errorf("recovery below surviving-input ceiling %.2f%%: paragraphs=%d", ceiling, len(r.Document.Paragraphs))
	}
	if len(r.Unmatched) != 10 {
		t.Errorf("unmatched=%+v, want the 10 missing/misread positions", r.Unmatched)
	}
}

func TestChallengeFurnitureTitleAndBody(t *testing.T) {
	canonical := []string{"The Keeper's Ledgers", "Nora found the old tower.", "The ferry left before dawn."}
	pages := []Page{
		page(at("The Keeper's Ledgers", .3, .03), layout(.15, canonical[0], canonical[1]), at("1", .5, .95)),
		page(at("The Keeper's Ledgers", .3, .03), layout(.15, canonical[2]), at("2", .5, .95)),
	}
	r, err := Sort(pages, canonical, sorters.ConfigForLanguage("english"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Document.Text != strings.Join(canonical, "\n") {
		t.Fatalf("text=%q", r.Document.Text)
	}
	for p, info := range r.Document.Pages {
		if len(info.Furniture) != 2 {
			t.Errorf("page %d: furniture=%+v", p, info.Furniture)
		}
	}
}

func TestChallengeUnmatchedPositions(t *testing.T) {
	canonical := []string{"same missing sentence", "alpha beta gamma", "same missing sentence"}
	r, err := Sort([]Page{page(layout(.2, "alpha beta gamma"))}, canonical, sorters.ConfigForLanguage("english"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Unhandled) != 2 || len(r.Unmatched) != 2 {
		t.Fatalf("unhandled=%q unmatched=%+v", r.Unhandled, r.Unmatched)
	}
	for j, line := range []int{0, 2} {
		if r.Unmatched[j].CanonicalLine != line || r.Unmatched[j].Span.Start != 0 || r.Unmatched[j].Span.End != len(canonical[line]) {
			t.Errorf("missing occurrence %d: %+v", j, r.Unmatched[j])
		}
	}
}
