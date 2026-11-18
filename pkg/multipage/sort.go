package multipage

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/goodblaster/gollate/pkg/language"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// Page is one page of input: its blocks, in the page's own 0-1 coordinate
// space and emit order, and its size (pixels or points - only the aspect
// ratio matters to sorting; the values are passed through to the output).
type Page struct {
	Blocks []ocr.Block
	Width  float64
	Height float64
}

// Document is a multi-page document in canonical reading order. Like
// sorters.Document, Text is the whole text as one blob and paragraphs and
// tokens reference byte spans of it; unlike it, a paragraph may run across
// a page break, so position lives on tokens, each with its page.
type Document struct {
	Text       string      `json:"text"`
	Paragraphs []Paragraph `json:"paragraphs"`
	Pages      []PageInfo  `json:"pages"`
}

// Paragraph is one paragraph of output: usually one canonical line, joined
// across the page break if it straddles one.
type Paragraph struct {
	// CanonicalLine indexes the canonical text given to Sort; nil for text
	// that matched no canonical line.
	CanonicalLine *int         `json:"canonical_line,omitempty"`
	Span          sorters.Span `json:"span"`
	Tokens        []Token      `json:"tokens"`
}

// Token is one word (one character for CJK) and where it sits.
type Token struct {
	Span       sorters.Span    `json:"span"`
	Page       int             `json:"page"`
	Bounds     ocr.BoundingBox `json:"bounds"`
	Confidence float64         `json:"confidence,omitempty"`

	// Canonical is the byte range of the paragraph's canonical line that
	// this token corresponds to; nil when it could not be established.
	Canonical *sorters.Span `json:"canonical,omitempty"`

	// Continuation is where the rest of the word is when it was broken
	// across a line or a page: Bounds covers its first half, Continuation
	// the second - possibly on the next page.
	Continuation *ocr.BlockPart `json:"continuation,omitempty"`
}

// PageInfo describes one input page in the result.
type PageInfo struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`

	// Furniture is text on the page that is no part of the canonical text
	// and sits outside the body: page numbers, running heads. It is kept
	// here rather than in the document text, where a stray "247" would
	// land in the middle of a paragraph at every page break.
	Furniture []Furniture `json:"furniture,omitempty"`

	// CanonicalLines is how many canonical lines were offered to this
	// page; LinesFound and LeftoverBlocks are the page sorter's metrics.
	CanonicalLines int `json:"canonical_lines"`
	LinesFound     int `json:"lines_found"`
	LeftoverBlocks int `json:"leftover_blocks"`
}

// Furniture is a run of non-canonical text in a page's margin.
type Furniture struct {
	Text   string          `json:"text"`
	Bounds ocr.BoundingBox `json:"bounds"`
}

// Result is the outcome of a multi-page sort.
type Result struct {
	Document *Document

	// Assignments is the page each canonical line was placed on, parallel
	// to the canonical lines given to Sort.
	Assignments []Assignment

	// Unhandled lists uncovered canonical token runs (including partially
	// matched lines), in input order. Repeated occurrences are retained.
	Unhandled []string

	// Unmatched identifies uncovered canonical token runs by line and byte
	// range. Repeated text retains separate occurrences. Unhandled is the
	// readable text of these ranges, in canonical order.
	Unmatched []UnmatchedSpan
}

// UnmatchedSpan is a half-open byte range within a canonical input line.
type UnmatchedSpan struct {
	CanonicalLine int          `json:"canonical_line"`
	Span          sorters.Span `json:"span"`
}

// Sort sorts a multi-page document against its canonical text.
//
// config is the single-page sorter configuration, normally
// sorters.ConfigForLanguage(lang); language remains the only hint. log may
// be nil.
func Sort(pages []Page, canonical []string, config sorters.SorterConfig, log logger.Logger) (*Result, error) {
	if log == nil {
		log = logger.Default()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	handler := language.ForLanguage(config.Language, canonical...)
	tokenize := func(text string) []string {
		return handler.Tokenize(sorters.NormalizeText(text))
	}

	// Work on a copy: furniture removal and seam joining edit the pages.
	pages = append([]Page(nil), pages...)
	for p := range pages {
		pages[p].Blocks = append([]ocr.Block(nil), pages[p].Blocks...)
	}
	furniture := stripFurniture(pages, canonical...)

	// Canonical vocabulary, for re-joining a word hyphenated across a
	// page break.
	vocabulary := make(map[string]bool)
	pairs := make(map[[2]string]bool)
	previous := ""
	for _, line := range canonical {
		tokens := tokenize(line)
		for _, token := range tokens {
			vocabulary[token] = true
			if previous != "" {
				pairs[[2]string{previous, token}] = true
			}
			previous = token
		}
	}
	joinSeamHyphens(pages, vocabulary, pairs)

	// Segment: which canonical lines are on which page.
	lineTokens := make([][]string, len(canonical))
	lineSpans := make([][]sorters.Span, len(canonical))
	for i, line := range canonical {
		lineTokens[i] = tokenize(line)
		lineSpans[i] = sorters.CanonicalTokenSpans(line, handler)
	}
	pageTokens := make([][]string, len(pages))
	for p, page := range pages {
		for _, b := range page.Blocks {
			pageTokens[p] = append(pageTokens[p], tokenize(b.Text)...)
		}
	}
	assignments := Segment(lineTokens, pageTokens)

	result := &Result{Document: &Document{}, Assignments: assignments}
	merged := &merger{handler: handler, canonical: canonical, byLine: make(map[int]*mergedParagraph)}

	for p, page := range pages {
		// Offer only this page's token ranges, including fragments of a
		// paragraph spanning several pages. No page can claim tokens
		// assigned elsewhere. Blank separators between selected lines
		// come along to retain paragraph structure.
		var selected []int
		part := make(map[int]string)    // canonical index -> text offered to this page
		partOffset := make(map[int]int) // where that text starts in the canonical line
		for i, a := range assignments {
			if len(lineTokens[i]) == 0 {
				continue
			}
			for _, fragment := range a.Fragments {
				if fragment.Page != p {
					continue
				}
				piece, offset, ok := canonicalFragmentFromSpans(canonical[i], fragment, lineSpans[i], len(lineTokens[i]))
				if !ok {
					return nil, fmt.Errorf("canonical line %d: cannot map token range [%d,%d) to bytes", i, fragment.Start, fragment.End)
				}
				part[i], partOffset[i] = piece, offset
			}
			if _, ok := part[i]; !ok {
				continue
			}
			selected = append(selected, i)
		}
		var subset []string
		var global []int  // subset index -> canonical index
		var offsets []int // subset index -> offset of the offered text in that line
		if len(selected) > 0 {
			chosen := make(map[int]bool, len(selected))
			for _, i := range selected {
				chosen[i] = true
			}
			for i := selected[0]; i <= selected[len(selected)-1]; i++ {
				if chosen[i] {
					subset = append(subset, part[i])
					global = append(global, i)
					offsets = append(offsets, partOffset[i])
				} else if len(lineTokens[i]) == 0 {
					subset = append(subset, "")
					global = append(global, i)
					offsets = append(offsets, 0)
				}
			}
		}

		blocks := make([]ocr.Block, len(page.Blocks))
		copy(blocks, page.Blocks)
		for i := range blocks {
			blocks[i].Page = p
			if blocks[i].PageWidth == 0 || blocks[i].PageHeight == 0 {
				blocks[i].PageWidth, blocks[i].PageHeight = int(page.Width+0.5), int(page.Height+0.5)
			}
		}

		sorter := sorters.NewOcrSorterWithConfig(blocks, subset, log, config)
		if _, err := sorter.Sort(); err != nil {
			return nil, err
		}

		metrics := sorter.Metrics()
		info := PageInfo{
			Width:          page.Width,
			Height:         page.Height,
			CanonicalLines: len(selected),
			LinesFound:     metrics.LinesFound,
			LeftoverBlocks: metrics.LeftoverBlocks,
		}
		info.Furniture = furniture[p]
		merged.addPage(p, sorter.Document(), global, offsets)
		result.Document.Pages = append(result.Document.Pages, info)
	}
	merged.write(result.Document)
	result.findUnmatched(canonical, lineSpans)
	return result, nil
}

// mergedToken is a token awaiting its final position in the text blob.
type mergedToken struct {
	text         string
	separator    string // what follows it within its paragraph ("", " ", "-")
	page         int
	bounds       ocr.BoundingBox
	confidence   float64
	canonical    *sorters.Span
	continuation *ocr.BlockPart
}

type mergedParagraph struct {
	canonicalLine int // -1 if unmatched
	tokens        []mergedToken
}

// merger accumulates per-page documents into one, in page order.
type merger struct {
	handler    language.Handler
	canonical  []string
	paragraphs []*mergedParagraph
	byLine     map[int]*mergedParagraph // canonical line -> its paragraph so far
}

// addPage appends one page's sorted document. global maps the page
// sorter's canonical line indices back to the full document's. A paragraph
// whose canonical line already has a paragraph - the head of a line that
// straddled in from the previous page - is joined onto it rather than
// started anew, so the page break leaves no paragraph break.
func (m *merger) addPage(page int, doc *sorters.Document, global, offsets []int) {
	for _, para := range doc.Paragraphs {
		if len(para.Tokens) == 0 {
			continue
		}

		// The page sorter saw only its part of a straddling line, so its
		// canonical spans are relative to that part.
		line, offset := -1, 0
		if para.CanonicalLine != nil && *para.CanonicalLine < len(global) {
			line, offset = global[*para.CanonicalLine], offsets[*para.CanonicalLine]
		}

		tokens := make([]mergedToken, len(para.Tokens))
		for i, tok := range para.Tokens {
			tokens[i] = mergedToken{
				text:         doc.Text[tok.Span.Start:tok.Span.End],
				page:         page,
				bounds:       tok.Bounds,
				confidence:   tok.Confidence,
				continuation: tok.Continuation,
			}
			if tok.Canonical != nil {
				tokens[i].canonical = &sorters.Span{Start: tok.Canonical.Start + offset, End: tok.Canonical.End + offset}
			}
			if i+1 < len(para.Tokens) {
				tokens[i].separator = doc.Text[tok.Span.End:para.Tokens[i+1].Span.Start]
			}
		}

		if earlier, ok := m.byLine[line]; ok && line >= 0 {
			last := &earlier.tokens[len(earlier.tokens)-1]
			if m.handler.NeedsSpaceBetween(last.text, tokens[0].text) && !m.compoundSeam(line, *last, tokens[0]) {
				last.separator = " "
			}
			earlier.tokens = append(earlier.tokens, tokens...)
			continue
		}
		mp := &mergedParagraph{canonicalLine: line, tokens: tokens}
		m.paragraphs = append(m.paragraphs, mp)
		if line >= 0 {
			m.byLine[line] = mp
		}
	}
}

// canonicalFragment maps matching-token boundaries back to the original
// bytes once, including punctuation between adjacent token spans. Unlike
// whitespace splitting, this handles compounds and mixed CJK/Latin text.
func canonicalFragment(text string, fragment Fragment, handler language.Handler) (string, int, bool) {
	return canonicalFragmentFromSpans(text, fragment, sorters.CanonicalTokenSpans(text, handler), len(handler.Tokenize(sorters.NormalizeText(text))))
}

func canonicalFragmentFromSpans(text string, fragment Fragment, spans []sorters.Span, tokenCount int) (string, int, bool) {
	// A whole line needs no internal boundary, even if normalization could
	// not supply exact token spans (it can still be sorted normally).
	if fragment.Start == 0 && fragment.End == tokenCount {
		left := strings.TrimLeftFunc(text, unicode.IsSpace)
		return strings.TrimSpace(left), len(text) - len(left), true
	}
	start, end := 0, len(text)
	if fragment.Start < 0 || fragment.End > len(spans) || fragment.Start >= fragment.End {
		return "", 0, false
	}
	if fragment.Start > 0 {
		start = spans[fragment.Start].Start
		if start < spans[fragment.Start-1].End {
			return "", 0, false
		}
	}
	if fragment.End < len(spans) {
		end = spans[fragment.End].Start
		if end < spans[fragment.End-1].End {
			return "", 0, false
		}
	}
	piece := text[start:end]
	trimmed := strings.TrimLeftFunc(piece, unicode.IsSpace)
	start += len(piece) - len(trimmed)
	return strings.TrimSpace(trimmed), start, true
}

// Keep the punctuation joining a compound without adding a space at a
// page seam. Ordinary word boundaries retain their normal spacing.
func (m *merger) compoundSeam(line int, a, b mergedToken) bool {
	if a.canonical == nil || b.canonical == nil || line < 0 || line >= len(m.canonical) {
		return false
	}
	text := m.canonical[line]
	end, start := a.canonical.End, b.canonical.Start
	if end > start || end < 0 || start > len(text) {
		return false
	}
	lastRune, _ := utf8.DecodeLastRuneInString(a.text)
	return !strings.ContainsFunc(text[end:start], unicode.IsSpace) && strings.ContainsRune(lineBreakHyphens, lastRune)
}

// findUnmatched derives diagnostics from final canonical coverage, not
// temporary per-page failures. Missing occurrences never collapse by text.
func (r *Result) findUnmatched(canonical []string, lineSpans [][]sorters.Span) {
	covered := make([][]sorters.Span, len(canonical))
	for _, para := range r.Document.Paragraphs {
		if para.CanonicalLine == nil {
			continue
		}
		i := *para.CanonicalLine
		for _, tok := range para.Tokens {
			if tok.Canonical != nil {
				covered[i] = append(covered[i], *tok.Canonical)
			}
		}
	}
	r.Unhandled = []string{}
	r.Unmatched = []UnmatchedSpan{}
	for i, line := range canonical {
		spans := lineSpans[i]
		if spans == nil && strings.TrimSpace(sorters.NormalizeText(line)) != "" {
			r.Unmatched = append(r.Unmatched, UnmatchedSpan{CanonicalLine: i, Span: sorters.Span{Start: 0, End: len(line)}})
			r.Unhandled = append(r.Unhandled, strings.TrimSpace(line))
			continue
		}
		ranges := covered[i]
		sort.Slice(ranges, func(a, b int) bool { return ranges[a].Start < ranges[b].Start })
		pos, end, missing := 0, -1, -1
		flush := func(until int) {
			if missing < 0 {
				return
			}
			span := sorters.Span{Start: spans[missing].Start, End: spans[until-1].End}
			r.Unmatched = append(r.Unmatched, UnmatchedSpan{CanonicalLine: i, Span: span})
			r.Unhandled = append(r.Unhandled, strings.TrimSpace(line[span.Start:span.End]))
			missing = -1
		}
		for j, span := range spans {
			for pos < len(ranges) && ranges[pos].Start <= span.Start {
				end = max(end, ranges[pos].End)
				pos++
			}
			if end >= span.End {
				flush(j)
			} else if missing < 0 {
				missing = j
			}
		}
		flush(len(spans))
	}
}

// lineBreakHyphens mirrors the sorter's set: the marks left at the end of
// a line when a word is broken across it.
const lineBreakHyphens = "-\u2010\u00AD\u00AC"

// joinSeamHyphens re-joins a word hyphenated across a page break: the last
// block of one page ends in a hyphen, and with the first block of the next
// it spells a canonical word, without evidence for the two parts as an
// adjacent canonical pair. The joined word stays
// on the first page; the second half is removed from the next. Run after
// furniture is stripped, so the page number is not in the way.
func joinSeamHyphens(pages []Page, vocabulary map[string]bool, pairs map[[2]string]bool) {
	for p := 0; p+1 < len(pages); p++ {
		if len(pages[p].Blocks) == 0 || len(pages[p+1].Blocks) == 0 {
			continue
		}
		last := &pages[p].Blocks[len(pages[p].Blocks)-1]
		first := pages[p+1].Blocks[0]
		stem := strings.TrimRight(strings.TrimSpace(last.Text), lineBreakHyphens)
		if stem == "" || stem == strings.TrimSpace(last.Text) {
			continue
		}
		joined := sorters.NormalizeText(stem + first.Text)
		separatePair := pairs[[2]string{sorters.NormalizeText(stem), sorters.NormalizeText(first.Text)}]
		if !vocabulary[joined] || separatePair {
			continue
		}
		last.OriginalOcrText = last.Text + " " + first.Text
		last.CorrectionType = "dehyphenated"
		last.Continuation = &ocr.BlockPart{Page: p + 1, Bounds: first.BoundingBox}
		last.Text = stem + first.Text
		pages[p+1].Blocks = pages[p+1].Blocks[1:]
	}
}

// write lays the accumulated paragraphs out as text and spans.
func (m *merger) write(doc *Document) {
	var text strings.Builder
	for _, mp := range m.paragraphs {
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		para := Paragraph{Span: sorters.Span{Start: text.Len()}}
		if mp.canonicalLine >= 0 {
			line := mp.canonicalLine
			para.CanonicalLine = &line
		}
		for i, tok := range mp.tokens {
			start := text.Len()
			text.WriteString(tok.text)
			para.Tokens = append(para.Tokens, Token{
				Span:         sorters.Span{Start: start, End: text.Len()},
				Page:         tok.page,
				Bounds:       tok.bounds,
				Confidence:   tok.confidence,
				Canonical:    tok.canonical,
				Continuation: tok.continuation,
			})
			if i+1 < len(mp.tokens) {
				text.WriteString(tok.separator)
			}
		}
		para.Span.End = text.Len()
		doc.Paragraphs = append(doc.Paragraphs, para)
	}
	doc.Text = text.String()
	if doc.Paragraphs == nil {
		doc.Paragraphs = []Paragraph{}
	}
}
