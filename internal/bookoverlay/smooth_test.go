package bookoverlay

import (
	"math"
	"reflect"
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

func TestSmoothingPreservesSentenceEdgesAndColumns(t *testing.T) {
	m := &Manifest{Words: []Word{{Sentence: 1}, {Sentence: 2}, {Sentence: 2}, {Sentence: 2}, {Sentence: 2}}}
	mark := func(n int, x, y, w, h float64) Mark {
		return Mark{OutputOrder: n, Words: []int{n}, Bounds: ocr.BoundingBox{Left: x, Top: y, Width: w, Height: h}}
	}
	marks := []Mark{
		mark(1, .1, .1, .1, .03),     // previous sentence
		mark(2, .21, .1, .08, .03),   // But: leave its left edge unchanged
		mark(3, .30, .105, .02, .02), // I: smooth the short glyph and gap
		mark(4, .6, .1, .1, .03),     // other column: leave gutter open
		mark(5, .1, .15, .1, .03),    // next line: no bridge
	}
	before := append([]Mark(nil), marks...)
	got := SmoothMarks(marks, m, 1000, 1000)
	if !reflect.DeepEqual(marks, before) {
		t.Fatal("changed source boxes")
	}
	if got[1].Bounds.Left != marks[1].Bounds.Left || math.Abs(got[2].Bounds.Left+got[2].Bounds.Width-.32) > 1e-9 {
		t.Fatal("moved outer word boundaries")
	}
	if math.Abs(got[1].Bounds.Left+got[1].Bounds.Width-got[2].Bounds.Left) > 1e-9 || got[1].Bounds.Top != got[2].Bounds.Top || got[1].Bounds.Height != got[2].Bounds.Height {
		t.Fatal("neighboring words are not a connected, level band")
	}
	for _, i := range []int{0, 3, 4} {
		if !reflect.DeepEqual(got[i], marks[i]) {
			t.Fatalf("changed separate sentence, column, or line: %d", i)
		}
	}
}
