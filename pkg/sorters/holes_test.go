package sorters

import (
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// holeTestRow lays out words as one visual row of blocks with real engine
// metadata (distance() short-circuits to 0 for blocks with no engine).
func holeTestRow(words []string, top float64, startIndex int) []ocr.Block {
	var blocks []ocr.Block
	for i, w := range words {
		blocks = append(blocks, ocr.Block{
			Text:       w,
			NormedText: w,
			Extractor:  "test",
			Index:      startIndex + i,
			PageWidth:  1000,
			PageHeight: 1000,
			BoundingBox: ocr.BoundingBox{
				Top: top, Left: 0.05 + float64(i)*0.08, Width: 0.06, Height: 0.02,
			},
		})
	}
	return blocks
}

func holeTestConfig() SorterConfig {
	config := DefaultConfig()
	config.MinWordsForEarlyPasses = 5
	return config
}

func joinTexts(blocks []Block) string {
	var texts []string
	for _, b := range blocks {
		if strings.TrimSpace(b.Text) != "" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, " ")
}

func TestChainHoleBridgesMissingWord(t *testing.T) {
	canonical := []string{"the quick brown fox jumps over seventeen lazy dogs"}
	// "brown" was never emitted by OCR: remove it but keep the layout gap.
	words := []string{"the", "quick", "brown", "fox", "jumps", "over", "seventeen", "lazy", "dogs"}
	var blocks []ocr.Block
	for _, b := range holeTestRow(words, 0.1, 0) {
		if b.Text != "brown" {
			blocks = append(blocks, b)
		}
	}

	config := holeTestConfig()
	config.EnableChainHoles = true

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
	sorted, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	m := sorter.Metrics()
	if m.LinesFound != 1 {
		t.Errorf("LinesFound = %d, want 1 (hole should keep the line whole)", m.LinesFound)
	}
	if m.HolesBridged != 1 || m.HolesLeftEmpty != 1 || m.HolesFilled != 0 {
		t.Errorf("holes bridged/filled/empty = %d/%d/%d, want 1/0/1", m.HolesBridged, m.HolesFilled, m.HolesLeftEmpty)
	}
	want := "the quick fox jumps over seventeen lazy dogs"
	if got := joinTexts(sorted); got != want {
		t.Errorf("sorted output = %q, want %q", got, want)
	}
}

func TestChainHoleGapFillRequiresExactText(t *testing.T) {
	// "brown" misread as "hrown", and no line data for repair: the hole is
	// bridged but gap-fill must NOT claim a near-miss - the edit-distance
	// relaxation that once lived here was measured worthless and removed.
	canonical := []string{"the quick brown fox jumps over seventeen lazy dogs"}
	words := []string{"the", "quick", "hrown", "fox", "jumps", "over", "seventeen", "lazy", "dogs"}
	blocks := holeTestRow(words, 0.1, 0)

	config := holeTestConfig()
	config.EnableChainHoles = true

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
	sorted, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	m := sorter.Metrics()
	if m.HolesBridged != 1 || m.HolesFilled != 0 || m.HolesLeftEmpty != 1 {
		t.Errorf("holes bridged/filled/empty = %d/%d/%d, want 1/0/1", m.HolesBridged, m.HolesFilled, m.HolesLeftEmpty)
	}
	// The unclaimed near-miss surfaces as a leftover, not silently dropped.
	want := "the quick fox jumps over seventeen lazy dogs hrown"
	if got := joinTexts(sorted); got != want {
		t.Errorf("sorted output = %q, want %q", got, want)
	}
}

func TestLineRepairCoversWhatGapFillNoLongerDoes(t *testing.T) {
	// Same misread, but WITH line data: repair rekeys it upfront, the line
	// matches whole (no hole at all), and output keeps the OCR text.
	canonical := []string{"the quick brown fox jumps over seventeen lazy dogs"}
	words := []string{"the", "quick", "hrown", "fox", "jumps", "over", "seventeen", "lazy", "dogs"}
	blocks := holeTestRow(words, 0.1, 0)
	for i := range blocks {
		blocks[i].LineId = "1"
	}

	config := holeTestConfig()
	config.EnableChainHoles = true

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
	sorted, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	m := sorter.Metrics()
	if m.LineRepairs != 1 || m.HolesBridged != 0 {
		t.Errorf("LineRepairs/HolesBridged = %d/%d, want 1/0", m.LineRepairs, m.HolesBridged)
	}
	want := "the quick hrown fox jumps over seventeen lazy dogs"
	if got := joinTexts(sorted); got != want {
		t.Errorf("sorted output = %q, want %q", got, want)
	}
}

func TestChainHoleGapFillRejectsUnrelatedBlock(t *testing.T) {
	canonical := []string{"the quick brown fox jumps over seventeen lazy dogs"}
	// The block in the gap reads "xyzzy" - spatially right, textually wrong.
	words := []string{"the", "quick", "xyzzy", "fox", "jumps", "over", "seventeen", "lazy", "dogs"}
	blocks := holeTestRow(words, 0.1, 0)

	config := holeTestConfig()
	config.EnableChainHoles = true

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	m := sorter.Metrics()
	if m.HolesFilled != 0 || m.HolesLeftEmpty != 1 {
		t.Errorf("holes filled/empty = %d/%d, want 0/1 (text confirmation must reject)", m.HolesFilled, m.HolesLeftEmpty)
	}
}

func TestChainHolesOffByDefault(t *testing.T) {
	canonical := []string{"the quick brown fox jumps over seventeen lazy dogs"}
	words := []string{"the", "quick", "hrown", "fox", "jumps", "over", "seventeen", "lazy", "dogs"}
	blocks := holeTestRow(words, 0.1, 0)

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, holeTestConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	m := sorter.Metrics()
	if m.HolesBridged != 0 || m.HolesFilled != 0 || m.HolesLeftEmpty != 0 {
		t.Errorf("hole metrics = %d/%d/%d, want all 0 with flag off", m.HolesBridged, m.HolesFilled, m.HolesLeftEmpty)
	}
}

func TestLevenshteinDistanceRunes(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"brown", "brown", 0},
		{"brown", "hrown", 1},
		{"brown", "browm", 1},
		{"brown", "brwn", 1},
		{"brown", "xyzzy", 5},
		{"", "abc", 3},
		// Rune-based: multi-byte substitution counts as one edit.
		{"día", "dia", 1},
		{"año", "ano", 1},
	}
	for _, c := range cases {
		if got := levenshteinDistanceRunes([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("levenshteinDistanceRunes(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestWrapBridgingFindsMultiVisualLinePath(t *testing.T) {
	canonical := []string{"alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima"}
	row1 := holeTestRow([]string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}, 0.10, 0)
	row2 := holeTestRow([]string{"golf", "hotel", "india", "juliet", "kilo", "lima"}, 0.13, 6)
	blocks := append(row1, row2...)

	for _, bridging := range []bool{false, true} {
		config := holeTestConfig()
		config.EnableWrapBridging = bridging

		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		sorted, err := sorter.Sort()
		if err != nil {
			t.Fatalf("Sort failed (bridging=%v): %v", bridging, err)
		}

		found := sorter.Metrics().LinesFound
		if bridging && found != 1 {
			t.Errorf("bridging on: LinesFound = %d, want 1 (path must cross the wrap)", found)
		}
		if !bridging && found != 0 {
			t.Errorf("bridging off: LinesFound = %d, want 0 (issue #3 wall)", found)
		}
		if bridging {
			want := "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima"
			if got := joinTexts(sorted); got != want {
				t.Errorf("bridging on: output = %q, want %q", got, want)
			}
		}
	}
}

func TestHoleAcrossWrapRequiresWrapBridging(t *testing.T) {
	// "foxtrot" (last word of row 1) is missing: the hole's bridge step goes
	// from "echo" across the wrap to "golf", which only works with both
	// flags on.
	canonical := []string{"alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima"}
	row1 := holeTestRow([]string{"alpha", "bravo", "charlie", "delta", "echo"}, 0.10, 0)
	row2 := holeTestRow([]string{"golf", "hotel", "india", "juliet", "kilo", "lima"}, 0.13, 5)
	blocks := append(row1, row2...)

	config := holeTestConfig()
	config.EnableChainHoles = true
	config.EnableWrapBridging = true

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
	sorted, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	m := sorter.Metrics()
	if m.LinesFound != 1 || m.HolesBridged != 1 {
		t.Errorf("LinesFound/HolesBridged = %d/%d, want 1/1", m.LinesFound, m.HolesBridged)
	}
	want := "alpha bravo charlie delta echo golf hotel india juliet kilo lima"
	if got := joinTexts(sorted); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestHoleWrapDeterministic(t *testing.T) {
	canonical := []string{"alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima"}

	var first string
	for run := range 5 {
		row1 := holeTestRow([]string{"alpha", "bravo", "chxrlie", "delta", "echo", "foxtrot"}, 0.10, 0)
		row2 := holeTestRow([]string{"golf", "hotel", "india", "juliet", "kilo", "lima"}, 0.13, 6)
		blocks := append(row1, row2...)

		config := holeTestConfig()
		config.EnableChainHoles = true
		config.EnableWrapBridging = true

		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		sorted, err := sorter.Sort()
		if err != nil {
			t.Fatalf("Sort failed on run %d: %v", run+1, err)
		}
		out := joinTexts(sorted)
		if run == 0 {
			first = out
		} else if out != first {
			t.Fatalf("run %d differs from run 1 (nondeterministic holes/wrap)", run+1)
		}
	}
}
