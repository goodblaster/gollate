package multipage

import (
	"strings"

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

	// Unhandled lists canonical lines matched on no page.
	Unhandled []string
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
	furniture := stripFurniture(pages)

	// Canonical vocabulary, for re-joining a word hyphenated across a
	// page break.
	vocabulary := make(map[string]bool)
	for _, line := range canonical {
		for _, token := range tokenize(line) {
			vocabulary[token] = true
		}
	}
	joinSeamHyphens(pages, vocabulary)

	// Segment: which canonical lines are on which page.
	lineTokens := make([][]string, len(canonical))
	for i, line := range canonical {
		lineTokens[i] = tokenize(line)
	}
	pageTokens := make([][]string, len(pages))
	for p, page := range pages {
		for _, b := range page.Blocks {
			pageTokens[p] = append(pageTokens[p], tokenize(b.Text)...)
		}
	}
	assignments := Segment(lineTokens, pageTokens)

	result := &Result{Document: &Document{}, Assignments: assignments}
	merged := &merger{handler: handler, byLine: make(map[int]*mergedParagraph)}
	unhandledSeen := make(map[string]bool)

	for p, page := range pages {
		// This page's lines: those placed here, plus the tail of a line
		// that straddles in from the previous page. A straddling line is
		// offered in two parts - its head here, its tail on the next
		// page - so neither page can claim words that belong to the
		// other. Blank lines between come along to keep paragraph
		// structure.
		var selected []int
		part := make(map[int]string) // canonical index -> text offered to this page
		for i, a := range assignments {
			if len(lineTokens[i]) == 0 {
				continue
			}
			switch {
			case a.Page == p && a.Straddle:
				part[i], _ = splitAtToken(canonical[i], a.Cut, tokenize)
			case a.Page == p:
				part[i] = canonical[i]
			case a.Straddle && a.Page == p-1:
				_, part[i] = splitAtToken(canonical[i], a.Cut, tokenize)
			default:
				continue
			}
			selected = append(selected, i)
		}
		var subset []string
		var global []int // subset index -> canonical index
		if len(selected) > 0 {
			chosen := make(map[int]bool, len(selected))
			for _, i := range selected {
				chosen[i] = true
			}
			for i := selected[0]; i <= selected[len(selected)-1]; i++ {
				if chosen[i] {
					subset = append(subset, part[i])
					global = append(global, i)
				} else if len(lineTokens[i]) == 0 {
					subset = append(subset, "")
					global = append(global, i)
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
		for _, text := range sorter.UnhandledText() {
			if !unhandledSeen[text] {
				unhandledSeen[text] = true
				result.Unhandled = append(result.Unhandled, text)
			}
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
		merged.addPage(p, sorter.Document(), global)
		result.Document.Pages = append(result.Document.Pages, info)
	}
	if result.Unhandled == nil {
		result.Unhandled = []string{}
	}

	merged.write(result.Document)
	return result, nil
}

// mergedToken is a token awaiting its final position in the text blob.
type mergedToken struct {
	text       string
	separator  string // what follows it within its paragraph ("", " ", "-")
	page       int
	bounds     ocr.BoundingBox
	confidence float64
}

type mergedParagraph struct {
	canonicalLine int // -1 if unmatched
	tokens        []mergedToken
}

// merger accumulates per-page documents into one, in page order.
type merger struct {
	handler    language.Handler
	paragraphs []*mergedParagraph
	byLine     map[int]*mergedParagraph // canonical line -> its paragraph so far
}

// addPage appends one page's sorted document. global maps the page
// sorter's canonical line indices back to the full document's. A paragraph
// whose canonical line already has a paragraph - the head of a line that
// straddled in from the previous page - is joined onto it rather than
// started anew, so the page break leaves no paragraph break.
func (m *merger) addPage(page int, doc *sorters.Document, global []int) {
	for _, para := range doc.Paragraphs {
		if len(para.Tokens) == 0 {
			continue
		}

		tokens := make([]mergedToken, len(para.Tokens))
		for i, tok := range para.Tokens {
			tokens[i] = mergedToken{
				text:       doc.Text[tok.Span.Start:tok.Span.End],
				page:       page,
				bounds:     tok.Bounds,
				confidence: tok.Confidence,
			}
			if i+1 < len(para.Tokens) {
				tokens[i].separator = doc.Text[tok.Span.End:para.Tokens[i+1].Span.Start]
			}
		}

		line := -1
		if para.CanonicalLine != nil && *para.CanonicalLine < len(global) {
			line = global[*para.CanonicalLine]
		}
		if earlier, ok := m.byLine[line]; ok && line >= 0 {
			last := &earlier.tokens[len(earlier.tokens)-1]
			if m.handler.NeedsSpaceBetween(last.text, tokens[0].text) {
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

// splitAtToken divides text so that the head holds its first cut tokens
// and the tail the rest. The split falls on a word boundary where the text
// has them, otherwise between characters.
func splitAtToken(text string, cut int, tokenize func(string) []string) (head, tail string) {
	if cut <= 0 {
		return "", text
	}
	bySpace := strings.ContainsAny(strings.TrimSpace(text), " \t")
	for i, r := range text {
		if i == 0 {
			continue
		}
		if bySpace && !(r == ' ' || r == '\t') {
			continue
		}
		if len(tokenize(text[:i])) >= cut {
			return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i:])
		}
	}
	return text, ""
}

// lineBreakHyphens mirrors the sorter's set: the marks left at the end of
// a line when a word is broken across it.
const lineBreakHyphens = "-\u2010\u00AD\u00AC"

// joinSeamHyphens re-joins a word hyphenated across a page break: the last
// block of one page ends in a hyphen, and with the first block of the next
// it spells a canonical word that neither half is. The joined word stays
// on the first page; the second half is removed from the next. Run after
// furniture is stripped, so the page number is not in the way.
func joinSeamHyphens(pages []Page, vocabulary map[string]bool) {
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
		bothWords := vocabulary[sorters.NormalizeText(stem)] && vocabulary[sorters.NormalizeText(first.Text)]
		if !vocabulary[joined] || bothWords {
			continue
		}
		last.OriginalOcrText = last.Text + " " + first.Text
		last.CorrectionType = "dehyphenated"
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
				Span:       sorters.Span{Start: start, End: text.Len()},
				Page:       tok.page,
				Bounds:     tok.bounds,
				Confidence: tok.confidence,
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
