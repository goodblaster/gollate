package sorters

import (
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// repairRow lays out words as one engine line: shared LineId, emit order.
func repairRow(words []string, lineId string, top float64, startIndex int) []ocr.Block {
	var blocks []ocr.Block
	for i, w := range words {
		blocks = append(blocks, ocr.Block{
			Text: w, NormedText: NormalizeText(w), Extractor: "test", LineId: lineId,
			Index: startIndex + i, PageWidth: 1000, PageHeight: 1000,
			BoundingBox: ocr.BoundingBox{Top: top, Left: 0.05 + float64(i)*0.08, Width: 0.06, Height: 0.02},
		})
	}
	return blocks
}

func repairConfig() SorterConfig {
	config := DefaultConfig()
	config.MinWordsForEarlyPasses = 3
	config.DisableLineRepair = false // opt in (held back from default; see config.go)
	return config
}

func TestLineRepairRekeysMisreadWord(t *testing.T) {
	// "monuments" misread as "rnonuments" - edit distance 2, beyond
	// gap-fill's ceiling, but flanked by exact matches on its engine line.
	canonical := []string{"ancient civilizations built monuments that still stand"}
	blocks := repairRow([]string{"ancient", "civilizations", "built", "rnonuments", "that", "still", "stand"}, "17", 0.1, 0)

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, repairConfig())
	sorted, err := sorter.Sort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	m := sorter.Metrics()
	if m.LineRepairs != 1 {
		t.Errorf("LineRepairs = %d, want 1", m.LineRepairs)
	}
	if m.LinesFound != 1 {
		t.Errorf("LinesFound = %d, want 1 (repaired line must match whole)", m.LinesFound)
	}

	// Match-only: output carries what OCR read, with repair metadata.
	var texts []string
	repairedSeen := false
	for _, b := range sorted {
		if b.Text != "" {
			texts = append(texts, b.Text)
		}
		if b.Text == "rnonuments" {
			repairedSeen = true
			if b.SuggestedText != "monuments" || b.CorrectionType != "line-anchored" || b.EditDistance != 2 {
				t.Errorf("repair metadata wrong: %+v", b)
			}
		}
	}
	if !repairedSeen {
		t.Error("misread block missing from output")
	}
	want := "ancient civilizations built rnonuments that still stand"
	if got := strings.Join(texts, " "); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestLineRepairEdgeToken(t *testing.T) {
	// Line-initial misread: only one flank, so the one-sided trigram rule
	// must fire.
	canonical := []string{"ancient civilizations built monuments"}
	blocks := repairRow([]string{"amcient", "civilizations", "built", "monuments"}, "3", 0.1, 0)

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, repairConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	if got := sorter.Metrics().LineRepairs; got != 1 {
		t.Errorf("LineRepairs = %d, want 1 (edge trigram)", got)
	}
}

func TestLineRepairSkipsAmbiguousContext(t *testing.T) {
	// "the ? fox" occurs with two different middles - no repair.
	canonical := []string{"the quick fox and the lazy fox"}
	blocks := repairRow([]string{"the", "qvick", "fox", "and", "the", "lazy", "fox"}, "1", 0.1, 0)

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, repairConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	if got := sorter.Metrics().LineRepairs; got != 0 {
		t.Errorf("LineRepairs = %d, want 0 (ambiguous middle must be skipped)", got)
	}
}

func TestLineRepairNeverTouchesKnownWords(t *testing.T) {
	// "home" misread as "some" - but "some" exists in canonical, so it is
	// not an alien and must never be rewritten (the failure mode that got
	// fuzzy matching deleted).
	canonical := []string{"go home now", "buy some milk"}
	blocks := append(
		repairRow([]string{"go", "some", "now"}, "1", 0.1, 0),
		repairRow([]string{"buy", "some", "milk"}, "2", 0.15, 3)...,
	)

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, repairConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	if got := sorter.Metrics().LineRepairs; got != 0 {
		t.Errorf("LineRepairs = %d, want 0 (vocabulary words are untouchable)", got)
	}
}

func TestLineRepairColumnSpanGuard(t *testing.T) {
	// An engine line wrongly spanning two columns: the alien's neighbors
	// come from different canonical lines and are never adjacent in
	// canonical, so the flanking pattern does not exist.
	canonical := []string{"alpha bravo charlie", "delta echo foxtrot"}
	blocks := repairRow([]string{"bravo", "zzzzz", "echo"}, "9", 0.1, 0)

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, repairConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	if got := sorter.Metrics().LineRepairs; got != 0 {
		t.Errorf("LineRepairs = %d, want 0 (column-spanning line must not repair)", got)
	}
}

func TestLineRepairDisableFlag(t *testing.T) {
	canonical := []string{"ancient civilizations built monuments that still stand"}
	blocks := repairRow([]string{"ancient", "civilizations", "built", "rnonuments", "that", "still", "stand"}, "17", 0.1, 0)

	config := repairConfig()
	config.DisableLineRepair = true
	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	if got := sorter.Metrics().LineRepairs; got != 0 {
		t.Errorf("LineRepairs = %d, want 0 with DisableLineRepair", got)
	}
}

func TestLineRepairInertWithoutLineData(t *testing.T) {
	canonical := []string{"ancient civilizations built monuments that still stand"}
	blocks := repairRow([]string{"ancient", "civilizations", "built", "rnonuments", "that", "still", "stand"}, "", 0.1, 0)

	sorter := NewOcrSorterWithConfig(blocks, canonical, nil, repairConfig())
	if _, err := sorter.Sort(); err != nil {
		t.Fatalf("Sort failed: %v", err)
	}
	if got := sorter.Metrics().LineRepairs; got != 0 {
		t.Errorf("LineRepairs = %d, want 0 without LineId (degrades to current behavior)", got)
	}
}
