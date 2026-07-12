package sorters

import (
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// tileBlocks lays out two product tiles side by side, each with a distinct
// two-word headline over an identical "learn more" action line. The correct
// reading order is tile-major: headline A, learn more A, headline B, learn
// more B - only the headline anchors can tell the duplicate instances apart.
func tileBlocks() []ocr.Block {
	mk := func(text string, index int, top, left float64) ocr.Block {
		return ocr.Block{
			Text: text, NormedText: text, Extractor: "test", Index: index,
			PageWidth: 1000, PageHeight: 1000,
			BoundingBox: ocr.BoundingBox{Top: top, Left: left, Width: 0.06, Height: 0.02},
		}
	}
	return []ocr.Block{
		// Tile A (left)
		mk("aurora", 0, 0.10, 0.05),
		mk("lamp", 1, 0.10, 0.12),
		mk("learn", 2, 0.15, 0.05),
		mk("more", 3, 0.15, 0.12),
		// Tile B (right)
		mk("breeze", 4, 0.10, 0.55),
		mk("fan", 5, 0.10, 0.62),
		mk("learn", 6, 0.15, 0.55),
		mk("more", 7, 0.15, 0.62),
	}
}

var tileCanonical = []string{
	"aurora lamp",
	"learn more",
	"",
	"breeze fan",
	"learn more",
}

func sortedIndexes(blocks []Block) []int {
	var out []int
	for _, b := range blocks {
		if b.Text != "" {
			out = append(out, b.Index)
		}
	}
	return out
}

func indexesEqual(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestShortLineAnchoringPicksCorrectDuplicateInstance(t *testing.T) {
	config := DefaultConfig()
	config.EnableShortLineAnchoring = true

	sorter := NewOcrSorterWithConfig(tileBlocks(), tileCanonical, nil, config)
	sorted, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	m := sorter.Metrics()
	if m.LinesFound != 4 {
		t.Errorf("LinesFound = %d, want 4 (pass loop must survive to attempt short lines)", m.LinesFound)
	}
	want := []int{0, 1, 2, 3, 4, 5, 6, 7}
	if got := sortedIndexes(sorted); !indexesEqual(got, want) {
		t.Errorf("block order = %v, want %v (duplicate 'learn more' must anchor to its own tile)", got, want)
	}
}

func TestShortLineAnchoringOffByDefault(t *testing.T) {
	sorter := NewOcrSorterWithConfig(tileBlocks(), tileCanonical, nil, DefaultConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	m := sorter.Metrics()
	// Issue #1 unfixed: with all lines short, pass 0 finds nothing and the
	// loop exits before short lines are ever attempted.
	if m.LinesFound != 0 {
		t.Errorf("LinesFound = %d, want 0 with flag off (early-exit starvation)", m.LinesFound)
	}
	if m.ShortLinesAnchored != 0 {
		t.Errorf("ShortLinesAnchored = %d, want 0 with flag off", m.ShortLinesAnchored)
	}
}

// reconTileBlocks uses three-word headlines so a MinWordsForEarlyPasses of 3
// admits headlines to the main loop while the two-word "learn more" lines
// are starved by the pass-loop early exit (issue #1) - the state the
// reconciliation pass exists to rescue. Optional "buy" lines exercise the
// single-word case.
func reconTileBlocks(withBuy bool) []ocr.Block {
	mk := func(text string, index int, top, left float64) ocr.Block {
		return ocr.Block{
			Text: text, NormedText: text, Extractor: "test", Index: index,
			PageWidth: 1000, PageHeight: 1000,
			BoundingBox: ocr.BoundingBox{Top: top, Left: left, Width: 0.06, Height: 0.02},
		}
	}
	blocks := []ocr.Block{
		// Tile A (left)
		mk("aurora", 0, 0.10, 0.05),
		mk("desk", 1, 0.10, 0.12),
		mk("lamp", 2, 0.10, 0.19),
		mk("learn", 3, 0.15, 0.05),
		mk("more", 4, 0.15, 0.12),
		// Tile B (right)
		mk("breeze", 5, 0.10, 0.55),
		mk("floor", 6, 0.10, 0.62),
		mk("fan", 7, 0.10, 0.69),
		mk("learn", 8, 0.15, 0.55),
		mk("more", 9, 0.15, 0.62),
	}
	if withBuy {
		blocks = append(blocks, mk("buy", 10, 0.19, 0.05), mk("buy", 11, 0.19, 0.55))
	}
	return blocks
}

func TestReconciliationRescuesShortLines(t *testing.T) {
	// Anchoring stays OFF: headlines are found in the main loop, the
	// duplicated "learn more" lines are starved by the pass-loop early
	// exit. The reconciliation pass must rescue both, each anchored to its
	// own tile.
	canonical := []string{"aurora desk lamp", "learn more", "", "breeze floor fan", "learn more"}
	config := DefaultConfig()
	config.MinWordsForEarlyPasses = 3
	config.EnableReconciliationPass = true

	sorter := NewOcrSorterWithConfig(reconTileBlocks(false), canonical, nil, config)
	sorted, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	m := sorter.Metrics()
	if m.LinesReconciled != 2 {
		t.Errorf("LinesReconciled = %d, want 2 (LinesFound=%d)", m.LinesReconciled, m.LinesFound)
	}
	want := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	if got := sortedIndexes(sorted); !indexesEqual(got, want) {
		t.Errorf("block order = %v, want %v (reconciled lines must anchor to their tiles)", got, want)
	}
}

func TestReconciliationSingleWordLineRequiresLowAnchorThreshold(t *testing.T) {
	// A single-word "buy" line under each tile. With the default
	// ReconMinExactAnchors (2) they stay unreconciled; with 1 they are
	// spatially pinned by their neighbors.
	canonical := []string{
		"aurora desk lamp", "learn more", "buy", "",
		"breeze floor fan", "learn more", "buy",
	}

	for _, minAnchors := range []int{2, 1} {
		config := DefaultConfig()
		config.MinWordsForEarlyPasses = 3
		config.EnableReconciliationPass = true
		config.ReconMinExactAnchors = minAnchors

		sorter := NewOcrSorterWithConfig(reconTileBlocks(true), canonical, nil, config)
		sorted, err := sorter.Sort()
		if err != nil {
			t.Fatalf("Sort failed (minAnchors=%d): %v", minAnchors, err)
		}
		m := sorter.Metrics()
		switch minAnchors {
		case 2:
			if m.LinesReconciled != 2 {
				t.Errorf("minAnchors=2: LinesReconciled = %d, want 2 (single-word lines excluded)", m.LinesReconciled)
			}
		case 1:
			if m.LinesReconciled != 4 {
				t.Errorf("minAnchors=1: LinesReconciled = %d, want 4 (buy lines pinned by anchors)", m.LinesReconciled)
			}
			want := []int{0, 1, 2, 3, 4, 10, 5, 6, 7, 8, 9, 11}
			if got := sortedIndexes(sorted); !indexesEqual(got, want) {
				t.Errorf("minAnchors=1: block order = %v, want %v", got, want)
			}
		}
	}
}

func TestAnchoringDeterministic(t *testing.T) {
	var first []int
	for run := range 5 {
		config := DefaultConfig()
		config.EnableShortLineAnchoring = true
		config.EnableReconciliationPass = true
		config.ReconMinExactAnchors = 1

		sorter := NewOcrSorterWithConfig(tileBlocks(), tileCanonical, nil, config)
		sorted, err := sorter.Sort()
		if err != nil {
			t.Fatalf("Sort failed on run %d: %v", run+1, err)
		}
		got := sortedIndexes(sorted)
		if run == 0 {
			first = got
		} else if !indexesEqual(got, first) {
			t.Fatalf("run %d order %v differs from run 1 %v", run+1, got, first)
		}
	}
}
