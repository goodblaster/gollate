//go:build darwin

package apple

import (
	"bytes"
	"image/png"
	"math"
	"testing"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
)

// Exercise Vision itself on authored proportional text: allocating line width
// by character count clips the sentence beginning after WWW and narrow iii.
func TestVisionWordBounds(t *testing.T) {
	if testing.Short() {
		t.Skip("runs Apple Vision")
	}
	dc := gg.NewContext(1400, 240)
	dc.SetRGB(1, 1, 1)
	dc.Clear()
	ttf, err := truetype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	dc.SetFontFace(truetype.NewFace(ttf, &truetype.Options{Size: 48, DPI: 72}))
	dc.SetRGB(0, 0, 0)
	words := []string{"Café", "WWW", "iii", "WWW.", "But", "I", "see", "another."}
	var lefts, widths []float64
	x := 40.0
	space, _ := dc.MeasureString(" ")
	for _, word := range words {
		width, _ := dc.MeasureString(word)
		dc.DrawString(word, x, 130)
		lefts, widths = append(lefts, x), append(widths, width)
		x += width + space
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		t.Fatal(err)
	}
	lines, err := (&Engine{}).ParseBytes(buf.Bytes(), []string{"en-US"})
	if err != nil {
		t.Fatal(err)
	}
	var got []Word
	for _, line := range lines {
		got = append(got, line.Words...)
	}
	if len(got) != len(words) {
		t.Fatalf("recognized %d words, want %d: %+v", len(got), len(words), got)
	}
	for i, word := range got {
		if word.Text != words[i] {
			t.Errorf("word %d: got %q, want %q", i, word.Text, words[i])
		}
		// Vision adds a small margin around glyphs; tolerate ten pixels at
		// 48-point type, but not the much larger proportional-width drift.
		if word.BoundsSource != "vision" || math.Abs(word.Left*1400-lefts[i]) > 10 || math.Abs((word.Left+word.Width)*1400-(lefts[i]+widths[i])) > 10 {
			t.Errorf("%q bounds %.1f..%.1f (%s), want near %.1f..%.1f", word.Text, word.Left*1400, (word.Left+word.Width)*1400, word.BoundsSource, lefts[i], lefts[i]+widths[i])
		}
	}
}
