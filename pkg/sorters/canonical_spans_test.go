package sorters

import (
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/language"
	"github.com/goodblaster/gollate/pkg/logger"
)

func TestCanonicalTokenSpans(t *testing.T) {
	text := `About $4.2 million by 6:30 a.m., said Dr. Okafor (the four-block plan).`
	tokens := canonicalTokenSpans(text, &language.English{})
	if tokens == nil {
		t.Fatal("no spans")
	}
	var got []string
	for _, tok := range tokens {
		got = append(got, tok.token+"="+text[tok.span.Start:tok.span.End])
	}
	want := []string{
		"about=About", "$4=$4", "2=2", "million=million", "by=by", "6=6", "30=30", "a_m=a.m.,",
		"said=said", "dr=Dr.", "okafor=Okafor", "the=(the", "four=four", "block=block", "plan=plan).",
	}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestCanonicalTokenSpansCJK(t *testing.T) {
	text := "朝は5時に起こされた。祖母は「早く」と言った。"
	tokens := canonicalTokenSpans(text, &language.CJK{})
	if tokens == nil {
		t.Fatal("no spans")
	}
	for _, tok := range tokens {
		if text[tok.span.Start:tok.span.End] != tok.token {
			t.Errorf("token %q spans %q", tok.token, text[tok.span.Start:tok.span.End])
		}
	}
	if len(tokens) != 19 {
		t.Errorf("got %d tokens, want 19 (punctuation carries none)", len(tokens))
	}
}

// Tokens point back at their own words of the canonical line, including
// repeated words (each instance to its own occurrence) and through a
// misread that left a hole.
func TestDocumentTokensCarryCanonicalSpans(t *testing.T) {
	canonical := []string{"the cat saw the dog and the dog saw the cat"}
	ocrWords := strings.Fields("the cat saw the dog and the d0g saw the cat") // one misread
	var blocks []Block
	left := 0.05
	for i, text := range ocrWords {
		b := word(text, left, 0.10, 0.05)
		b.Index = i
		blocks = append(blocks, b)
		left += 0.06
	}
	sorter := NewOcrSorterWithConfig(blocks, canonical, logger.Noop{}, ConfigForLanguage("english"))
	if _, err := sorter.Sort(); err != nil {
		t.Fatal(err)
	}
	doc := sorter.Document()
	if len(doc.Paragraphs) != 1 || doc.Paragraphs[0].CanonicalLine == nil {
		t.Fatalf("expected one matched paragraph, got %+v", doc.Paragraphs)
	}
	lastStart := -1
	for _, tok := range doc.Paragraphs[0].Tokens {
		text := tok.String()
		if text == "d0g" {
			continue // unreadable: may or may not be tied to "dog"
		}
		if tok.Canonical == nil {
			t.Errorf("token %q has no canonical span", text)
			continue
		}
		if got := canonical[0][tok.Canonical.Start:tok.Canonical.End]; got != text {
			t.Errorf("token %q -> canonical %q", text, got)
		}
		if tok.Canonical.Start <= lastStart {
			t.Errorf("token %q: span start %d not after previous %d (repeated word tied to the wrong occurrence)", text, tok.Canonical.Start, lastStart)
		}
		lastStart = tok.Canonical.Start
	}
}
