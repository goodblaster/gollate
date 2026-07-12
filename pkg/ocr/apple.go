package ocr

import (
	"strconv"

	"github.com/goodblaster/gollate/pkg/engines/apple"
)

// FromApple converts an Apple Vision OCR block to the common Block format.
func FromApple(block apple.Block, pageWidth, pageHeight int) Block {
	return Block{
		Text:       block.Text, // Original text with punctuation
		NormedText: "",         // Will be set by sorter using proper NormalizeText function
		BoundingBox: BoundingBox{
			Top:    block.Top,
			Left:   block.Left,
			Width:  block.Width,
			Height: block.Height,
		},
		Confidence: block.Confidence,
		Extractor:  "apple",
		PageWidth:  pageWidth,
		PageHeight: pageHeight,
		LineId:     strconv.Itoa(block.LineNum),
	}
}
