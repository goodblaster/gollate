package sorters

import (
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// column lays out blocks stacked vertically (tategaki) or in a row.
func flowBlocks(vertical bool) []ocr.Block {
	var blocks []ocr.Block
	for line := 0; line < 4; line++ {
		for i := 0; i < 5; i++ {
			b := ocr.Block{Text: "字", NormedText: "字", Extractor: "t", LineId: string(rune('a' + line)), Index: line*5 + i}
			if vertical {
				b.BoundingBox = ocr.BoundingBox{Left: 0.8 - float64(line)*0.1, Top: 0.1 + float64(i)*0.05, Width: 0.03, Height: 0.03}
			} else {
				b.BoundingBox = ocr.BoundingBox{Left: 0.1 + float64(i)*0.05, Top: 0.1 + float64(line)*0.1, Width: 0.03, Height: 0.03}
			}
			blocks = append(blocks, b)
		}
	}
	return blocks
}

func TestDetectVerticalText(t *testing.T) {
	if !detectVerticalText(flowBlocks(true)) {
		t.Error("stacked columns must detect as vertical")
	}
	if detectVerticalText(flowBlocks(false)) {
		t.Error("horizontal rows must not detect as vertical")
	}
	blocks := flowBlocks(true)
	for i := range blocks {
		blocks[i].LineId = "" // no line data -> no flow signal
	}
	if detectVerticalText(blocks) {
		t.Error("must be inert without LineId")
	}
}
