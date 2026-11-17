package bookoverlay

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/multipage"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

func spanOf(text, word string) *sorters.Span {
	start := strings.Index(text, word)
	return &sorters.Span{Start: start, End: start + len(word)}
}

func TestCanonicalNumbersPreserveBytesAndContinuations(t *testing.T) {
	canonical := Lines("Heading\n\nCafé co-operate now. Missing sentence here.\n")
	line := 2
	doc := &multipage.Document{Text: "now. Café co operate ???", Pages: make([]multipage.PageInfo, 2)}
	b := ocr.BoundingBox{Left: .2, Top: .2, Width: .1, Height: .1}
	para := multipage.Paragraph{CanonicalLine: &line}
	for i, pair := range [][2]string{{"now.", "now."}, {"Café", "Café"}, {"co", "co"}, {"operate", "operate"}, {"???", ""}} {
		tok := multipage.Token{Span: *spanOf(doc.Text, pair[0]), Page: i % 2, Bounds: b}
		if pair[1] != "" {
			tok.Canonical = spanOf(canonical[line], pair[1])
		}
		if i == 3 {
			tok.Continuation = &ocr.BlockPart{Page: 0, Bounds: b}
		}
		para.Tokens = append(para.Tokens, tok)
	}
	doc.Paragraphs = []multipage.Paragraph{para}
	m, err := Build(doc, canonical, 117, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Words) != 7 || m.Words[1].Text != "Café" || m.Words[2].Text != "co-operate" {
		t.Fatalf("wrong canonical words: %+v", m.Words)
	}
	if m.Words[2].Span.Start != len("Café ") || m.Words[2].Line != 2 {
		t.Fatal("lost UTF-8 byte offsets or blank source line")
	}
	if got := m.Pages[0].Marks[0]; got.OutputOrder != 1 || !reflect.DeepEqual(got.Words, []int{4}) {
		t.Fatalf("used output order instead of canonical number: %+v", got)
	}
	if got := m.Pages[0].Marks[2]; !got.Continuation || got.OutputOrder != 4 || label(got) != "3+" {
		t.Fatalf("bad cross-page continuation: %+v", got)
	}
	if got := m.Pages[0].Marks[3]; label(got) != "U5" {
		t.Fatalf("invented a canonical link: %+v", got)
	}
	if !reflect.DeepEqual(m.Missing, []int{1, 5, 6, 7}) {
		t.Fatalf("missing words: %v", m.Missing)
	}
	if m.Sentences[1].Text != "Café co-operate now." {
		t.Fatalf("sentence boundary: %+v", m.Sentences)
	}
}

func TestWord117AndDeterministicSampling(t *testing.T) {
	var words []string
	for n := 1; n <= 120; n++ {
		words = append(words, fmt.Sprintf("w%d.", n))
	}
	canonical := strings.Join(words, " ")
	line := 0
	doc := &multipage.Document{Text: canonical, Pages: make([]multipage.PageInfo, 1)}
	para := multipage.Paragraph{CanonicalLine: &line}
	for _, word := range words {
		span := spanOf(canonical, word)
		para.Tokens = append(para.Tokens, multipage.Token{Span: *span, Canonical: span, Bounds: ocr.BoundingBox{Left: .1, Top: .2, Width: .1, Height: .1}})
	}
	doc.Paragraphs = []multipage.Paragraph{para}
	m, err := Build(doc, []string{canonical}, 117, 2)
	if err != nil {
		t.Fatal(err)
	}
	if label(m.Pages[0].Marks[116]) != "117" {
		t.Fatal("canonical word 117 not labeled 117")
	}
	for i := 0; i < 5; i++ {
		other, err := Build(doc, []string{canonical}, 117, 2)
		if err != nil || !reflect.DeepEqual(m, other) {
			t.Fatal("sampling not reproducible")
		}
	}
	other, _ := Build(doc, []string{canonical}, 118, 2)
	if reflect.DeepEqual(m.Pages[0].Sentences, other.Pages[0].Sentences) {
		t.Fatal("seed did not change sample")
	}
	if len(m.Pages[0].Sentences) != 2 {
		t.Fatal("expected two samples")
	}
}

func TestRenderWritesPageViewsAtOriginalPositions(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{230, 230, 230, 255})
		}
	}
	f, err := os.Create(filepath.Join(dir, "page-1.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	line := 0
	doc := &multipage.Document{Text: "word", Pages: make([]multipage.PageInfo, 1), Paragraphs: []multipage.Paragraph{{CanonicalLine: &line, Tokens: []multipage.Token{{Span: sorters.Span{Start: 0, End: 4}, Canonical: &sorters.Span{Start: 0, End: 4}, Bounds: ocr.BoundingBox{Left: .2, Top: .2, Width: .2, Height: .1}}}}}}
	before, _ := json.Marshal(doc)
	out := filepath.Join(dir, "overlays")
	if err := Write(doc, []string{"word"}, Options{ImageDir: dir, OutputDir: out, Title: "test", Seed: 117, Samples: 2}); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(doc)
	if string(before) != string(after) {
		t.Fatal("renderer modified result")
	}
	for _, mode := range []string{"order", "spotcheck"} {
		f, err := os.Open(filepath.Join(out, mode, "page-001.png"))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Bounds().Dx() != 820 || decoded.Bounds().Dy() != 496 {
			t.Fatalf("unexpected PNG bounds: %v", decoded.Bounds())
		}
		// The interior of the .2,.2 box maps to x=50,y=190 after
		// 2x scaling and the 96-pixel header. A distant source pixel stays gray.
		r, g, b, _ := decoded.At(50, 190).RGBA()
		if r == g && g == b {
			t.Fatal("box missing at original word position")
		}
		r, g, b, _ = decoded.At(10, 190).RGBA()
		if r != g || g != b {
			t.Fatal("source outside box unexpectedly colored")
		}
	}
	data, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Pages) != 1 || label(m.Pages[0].Marks[0]) != "1" {
		t.Fatal("bad manifest")
	}
	doc.Paragraphs[0].Tokens[0].Page = 1
	if _, err := Build(doc, []string{"word"}, 117, 2); err == nil {
		t.Fatal("invalid page accepted")
	}
}
