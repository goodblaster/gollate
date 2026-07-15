package pdftext

import (
	"strings"
	"testing"
)

func TestSelect(t *testing.T) {
	if _, err := Select("no-such-backend", ""); err == nil {
		t.Fatal("expected error for unknown backend")
	}
	b, err := Select("auto", "")
	if err != nil {
		t.Skipf("no backend available on this machine: %v", err)
	}
	if b.Name() != Backends()[0].Name() {
		t.Fatalf("auto picked %q, want first available %q", b.Name(), Backends()[0].Name())
	}
}

func TestSelect_IndicPrefersPoppler(t *testing.T) {
	if !(popplerBackend{}).Available() {
		t.Skip("pdftotext not installed")
	}
	b, err := Select("auto", "hindi")
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if b.Name() != "poppler" {
		t.Fatalf("hindi hint picked %q, want poppler", b.Name())
	}
}

// TestPoppler_HindiFixture is the reason the poppler backend exists:
// PDFKit drops Devanagari matras; poppler must round-trip them.
func TestPoppler_HindiFixture(t *testing.T) {
	b := popplerBackend{}
	if !b.Available() {
		t.Skip("pdftotext not installed")
	}
	pages, err := b.Extract("../../testdata/ocr-tests/hindi-single/document.pdf")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(pages) != 1 || len(pages[0].Words) < 100 {
		t.Fatalf("expected 1 full page, got %d page(s)", len(pages))
	}
	var joined strings.Builder
	for _, w := range pages[0].Words {
		if w.LineId == "" {
			t.Fatalf("word %q missing line id", w.Text)
		}
		joined.WriteString(w.Text)
		joined.WriteByte(' ')
	}
	// First canonical words, matras intact.
	for _, want := range []string{"दैनिक", "जीवन", "ताना-बाना"} {
		if !strings.Contains(joined.String(), want) {
			t.Fatalf("extracted text missing %q (matras dropped?)", want)
		}
	}
}

func TestRepairRTL(t *testing.T) {
	// "نسيج الحياة" as poppler emits it: visual order — words
	// leftmost-first on the line, characters reversed within words.
	rev := func(s string) string { return reverseRunes(s) }
	words := []Word{
		{Text: rev("الحياة"), Left: 0.2, LineId: "1-0-0"},
		{Text: rev("نسيج"), Left: 0.5, LineId: "1-0-0"},
		{Text: "plain", Left: 0.1, LineId: "1-0-1"},
	}
	got := repairRTL(words)
	if got[0].Text != "نسيج" || got[1].Text != "الحياة" {
		t.Fatalf("RTL line not repaired: %q %q", got[0].Text, got[1].Text)
	}
	if got[2].Text != "plain" {
		t.Fatalf("LTR line disturbed: %q", got[2].Text)
	}
}

func TestParseTSV(t *testing.T) {
	tsv := "level\tpage_num\tpar_num\tblock_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"1\t1\t0\t0\t0\t0\t0.0\t0.0\t100.0\t200.0\t-1\t###PAGE###\n" +
		"3\t1\t0\t0\t0\t0\t10.0\t20.0\t30.0\t10.0\t-1\t###FLOW###\n" +
		"5\t1\t0\t0\t0\t0\t10.0\t20.0\t30.0\t10.0\t95\tHello\n"
	pages, err := parseTSV([]byte(tsv))
	if err != nil {
		t.Fatalf("parseTSV: %v", err)
	}
	if len(pages) != 1 || pages[0].Width != 100 || pages[0].Height != 200 {
		t.Fatalf("bad page: %+v", pages)
	}
	w := pages[0].Words[0]
	if w.Text != "Hello" || w.Left != 0.1 || w.Top != 0.1 || w.Width != 0.3 || w.Height != 0.05 {
		t.Fatalf("bad word: %+v", w)
	}
	if w.LineId != "1-0-0-0" {
		t.Fatalf("bad line id: %q", w.LineId)
	}
}
