package sorters

import (
	"strings"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// Document is the primary output format, modeled on Google Document AI
// rather than AWS Textract: Text holds all sorted text as one readable
// blob, and the layout below it references byte ranges of that blob
// instead of carrying its own copies of the text.
//
//   - Document.Text - the full page in reading order, paragraphs separated
//     by newlines. Most consumers want this first.
//   - Paragraph - one sorted output line/paragraph: a span into Text, the
//     bounding box of the whole paragraph, and its tokens.
//   - Token - one word (one character for CJK): a span into Text plus that
//     word's page coordinates and OCR confidence.
//
// Spans are byte offsets into Text (safe for direct Go slicing; the same
// convention Document AI uses), not rune counts - this matters for CJK and
// accented text.
//
// Document, Paragraph, and Token implement String() for debugging: each
// returns its slice of the text. The backing references are unexported and
// never serialized; after JSON round-trips, use the spans against
// Document.Text directly.
type Document struct {
	Text       string      `json:"text"`
	Paragraphs []Paragraph `json:"paragraphs"`
}

// Span is a byte range [Start, End) into Document.Text.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Paragraph is one sorted line/paragraph of output.
type Paragraph struct {
	Span   Span            `json:"span"`
	Bounds ocr.BoundingBox `json:"bounds"`
	Tokens []Token         `json:"tokens"`

	text *string // backref for String(); debugging only, never serialized
}

// Token is one word's position: where its text sits in Document.Text and
// where the word sits on the page.
type Token struct {
	Span       Span            `json:"span"`
	Bounds     ocr.BoundingBox `json:"bounds"`
	Confidence float64         `json:"confidence,omitempty"`

	text *string // backref for String(); debugging only, never serialized
}

func (d *Document) String() string { return d.Text }

func (p Paragraph) String() string {
	if p.text == nil {
		return ""
	}
	return (*p.text)[p.Span.Start:p.Span.End]
}

func (t Token) String() string {
	if t.text == nil {
		return ""
	}
	return (*t.text)[t.Span.Start:t.Span.End]
}

// Document assembles the sorted output into the document format. Call after
// Sort(); before that it returns an empty document.
func (s *Sorter) Document() *Document {
	doc := &Document{}
	var text strings.Builder
	var run []Block

	flush := func() {
		if len(run) == 0 {
			return
		}
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		para := Paragraph{Span: Span{Start: text.Len()}}
		for i, blk := range run {
			start := text.Len()
			text.WriteString(blk.Text)
			para.Tokens = append(para.Tokens, Token{
				Span:       Span{Start: start, End: text.Len()},
				Bounds:     blk.BoundingBox,
				Confidence: blk.Confidence,
			})
			if i < len(run)-1 && s.handler.NeedsSpaceBetween(blk.Text, run[i+1].Text) {
				text.WriteByte(' ')
			}
		}
		para.Span.End = text.Len()
		para.Bounds = unionBounds(para.Tokens)
		doc.Paragraphs = append(doc.Paragraphs, para)
		run = nil
	}

	// The sorter's native output separates lines with empty blocks; the
	// document format replaces that in-band convention with structure.
	for _, blk := range s.output {
		if blk.Engine() == "" {
			flush()
			continue
		}
		run = append(run, blk)
	}
	flush()

	doc.Text = text.String()
	for i := range doc.Paragraphs {
		doc.Paragraphs[i].text = &doc.Text
		for j := range doc.Paragraphs[i].Tokens {
			doc.Paragraphs[i].Tokens[j].text = &doc.Text
		}
	}
	return doc
}

// unionBounds is the smallest box containing every token.
func unionBounds(tokens []Token) ocr.BoundingBox {
	if len(tokens) == 0 {
		return ocr.BoundingBox{}
	}
	first := tokens[0].Bounds
	top, left := first.Top, first.Left
	bottom, right := first.Top+first.Height, first.Left+first.Width
	for _, t := range tokens[1:] {
		b := t.Bounds
		top = min(top, b.Top)
		left = min(left, b.Left)
		bottom = max(bottom, b.Top+b.Height)
		right = max(right, b.Left+b.Width)
	}
	return ocr.BoundingBox{Top: top, Left: left, Width: right - left, Height: bottom - top}
}
