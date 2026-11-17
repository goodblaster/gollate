// Package bookoverlay renders diagnostic views of saved multi-page results.
// It displays the sorter's existing links; it never realigns OCR to the source.
package bookoverlay

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"unicode"

	"github.com/goodblaster/gollate/pkg/multipage"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// All public word, sentence, page, and output-order numbers are one-based.
// Source lines and byte spans retain the sorter's zero-based convention.
type Word struct {
	Number   int          `json:"number"`
	Line     int          `json:"line"`
	Span     sorters.Span `json:"span"`
	Text     string       `json:"text"`
	Sentence int          `json:"sentence"`
}

type Sentence struct {
	Number int    `json:"number"`
	First  int    `json:"first_word"`
	Last   int    `json:"last_word"`
	Text   string `json:"text"`
}

type Mark struct {
	OutputOrder  int             `json:"output_order"`
	Words        []int           `json:"canonical_words,omitempty"`
	Text         string          `json:"output_text"`
	Bounds       ocr.BoundingBox `json:"bounds"`
	Continuation bool            `json:"continuation,omitempty"`
}

type Page struct {
	Number    int    `json:"page"`
	Marks     []Mark `json:"marks"`
	Sentences []int  `json:"sampled_sentences"`
}

type Manifest struct {
	Numbering string     `json:"numbering"`
	Seed      int64      `json:"seed"`
	Words     []Word     `json:"canonical_words"`
	Sentences []Sentence `json:"sentences"`
	Missing   []int      `json:"unlinked_canonical_words"`
	Pages     []Page     `json:"pages"`
}

// whitespaceSpans indexes original UTF-8 bytes, not runes or normalized words.
// A printed compound is one word even when the matcher splits it into tokens.
func whitespaceSpans(s string) []sorters.Span {
	var spans []sorters.Span
	start := -1
	for i, r := range s {
		if unicode.IsSpace(r) {
			if start >= 0 {
				spans = append(spans, sorters.Span{Start: start, End: i})
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		spans = append(spans, sorters.Span{Start: start, End: len(s)})
	}
	return spans
}

func sentenceEnd(word string) bool {
	s := strings.TrimRight(word, "\"'”’)]}")
	if strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?") {
		return true
	}
	if !strings.HasSuffix(s, ".") {
		return false
	}
	// An endnote's leading "116." belongs to the following sentence.
	if strings.TrimFunc(strings.TrimSuffix(s, "."), unicode.IsDigit) == "" {
		return false
	}
	switch strings.ToLower(s) {
	case "mr.", "mrs.", "ms.", "dr.", "st.", "prof.", "e.g.", "i.e.", "etc.":
		return false
	}
	return true
}

func validSpan(s sorters.Span, text string) bool {
	return s.Start >= 0 && s.End > s.Start && s.End <= len(text)
}

func validBounds(b ocr.BoundingBox) bool {
	for _, x := range []float64{b.Left, b.Top, b.Width, b.Height} {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return b.Width > 0 && b.Height > 0 && b.Left < 1 && b.Top < 1 && b.Left+b.Width > 0 && b.Top+b.Height > 0
}

func Build(doc *multipage.Document, canonical []string, seed int64, samples int) (*Manifest, error) {
	if doc == nil || len(doc.Pages) == 0 {
		return nil, fmt.Errorf("document has no pages")
	}
	if samples < 1 || samples > 2 {
		return nil, fmt.Errorf("sentences per page must be 1 or 2")
	}
	m := &Manifest{Numbering: "1-based whitespace words across the complete canonical text, including headings and note numbers; output_order is the 1-based final OCR token order; source lines and byte spans are 0-based", Seed: seed}
	byLine := make([][]int, len(canonical))
	for line, text := range canonical {
		spans := whitespaceSpans(text)
		first, begin := len(m.Words)+1, 0
		for i, span := range spans {
			n := len(m.Words) + 1
			m.Words = append(m.Words, Word{n, line, span, text[span.Start:span.End], len(m.Sentences) + 1})
			byLine[line] = append(byLine[line], n)
			if sentenceEnd(text[span.Start:span.End]) || i == len(spans)-1 {
				m.Sentences = append(m.Sentences, Sentence{len(m.Sentences) + 1, first, n, text[spans[begin].Start:span.End]})
				first, begin = n+1, i+1
			}
		}
	}
	m.Pages = make([]Page, len(doc.Pages))
	for i := range m.Pages {
		m.Pages[i].Number = i + 1
	}
	linked := make([]bool, len(m.Words))
	order := 0
	for _, para := range doc.Paragraphs {
		if para.CanonicalLine != nil && (*para.CanonicalLine < 0 || *para.CanonicalLine >= len(canonical)) {
			return nil, fmt.Errorf("invalid canonical line %d", *para.CanonicalLine)
		}
		for _, token := range para.Tokens {
			order++
			if !validSpan(token.Span, doc.Text) {
				return nil, fmt.Errorf("invalid output span at token %d", order)
			}
			mark := Mark{OutputOrder: order, Text: doc.Text[token.Span.Start:token.Span.End], Bounds: token.Bounds}
			if token.Canonical != nil {
				if para.CanonicalLine == nil {
					return nil, fmt.Errorf("canonical span without line at token %d", order)
				}
				line := *para.CanonicalLine
				if !validSpan(*token.Canonical, canonical[line]) {
					return nil, fmt.Errorf("invalid canonical span at token %d", order)
				}
				for _, n := range byLine[line] {
					w := m.Words[n-1]
					if w.Span.Start < token.Canonical.End && w.Span.End > token.Canonical.Start {
						mark.Words = append(mark.Words, n)
						linked[n-1] = true
					}
				}
			}
			add := func(page int, mark Mark) error {
				if page < 0 || page >= len(m.Pages) || !validBounds(mark.Bounds) {
					return fmt.Errorf("invalid page/bounds at token %d", order)
				}
				m.Pages[page].Marks = append(m.Pages[page].Marks, mark)
				return nil
			}
			if err := add(token.Page, mark); err != nil {
				return nil, err
			}
			if part := token.Continuation; part != nil {
				mark.Bounds, mark.Continuation = part.Bounds, true
				if err := add(part.Page, mark); err != nil {
					return nil, err
				}
			}
		}
	}
	for i, yes := range linked {
		if !yes {
			m.Missing = append(m.Missing, i+1)
		}
	}
	for i := range m.Pages {
		seen := make(map[int]bool)
		for _, mark := range m.Pages[i].Marks {
			for _, n := range mark.Words {
				seen[m.Words[n-1].Sentence] = true
			}
		}
		var candidates []int
		for n := range seen {
			candidates = append(candidates, n)
		}
		sort.Ints(candidates) // Never let map iteration affect a seeded sample.
		rng := rand.New(rand.NewSource(seed + int64(i)*1000003))
		rng.Shuffle(len(candidates), func(a, b int) { candidates[a], candidates[b] = candidates[b], candidates[a] })
		m.Pages[i].Sentences = candidates[:min(samples, len(candidates))]
		sort.Ints(m.Pages[i].Sentences)
	}
	return m, nil
}
