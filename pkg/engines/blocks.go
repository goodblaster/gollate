package engines

import (
	"encoding/json"
	"io"

	"github.com/goodblaster/errors"
	"github.com/goodblaster/gollate/pkg/ocr"
)

// blocksEngine is the engine-neutral escape hatch: input is a JSON array of
// blocks already in the normalized format (see ocr.Block), in OCR emit
// order. Any engine we don't ship an adapter for - Textract, Google
// Document AI, a local model - can be used by converting its response to
// this schema in any language:
//
//	[
//	  {
//	    "text": "Hello",
//	    "bounds": {"top": 0.1, "left": 0.2, "width": 0.05, "height": 0.02},
//	    "normalized_conf": 0.98
//	  },
//	  ...
//	]
//
// Coordinates are fractions of the page (0-1). Blocks must be word/token
// granularity, not lines or paragraphs. Index, OriginalIndex, page
// dimensions, and a default Extractor are filled in here, so only text and
// bounds are required.
type blocksEngine struct{}

func (blocksEngine) Name() string { return "blocks" }

func (blocksEngine) Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
	var raw []ocr.Block
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, errors.Wrap(err, "failed to parse blocks JSON - expected an array of normalized blocks (see ocr.Block)")
	}

	var blocks []ocr.Block
	for i, block := range raw {
		if block.Text == "" {
			continue
		}
		if block.BoundingBox.Left < 0 || block.BoundingBox.Left > 1 ||
			block.BoundingBox.Top < 0 || block.BoundingBox.Top > 1 {
			return nil, errors.Newf("block %d (%q): coordinates must be 0-1 page fractions, got top=%v left=%v",
				i, block.Text, block.BoundingBox.Top, block.BoundingBox.Left)
		}
		if block.Extractor == "" {
			// A non-empty engine name is load-bearing: distance calculation
			// short-circuits for blocks without one.
			block.Extractor = "blocks"
		}
		block.Index = i
		block.OriginalIndex = i
		block.PageWidth = pageWidth
		block.PageHeight = pageHeight
		blocks = append(blocks, block)
	}
	return blocks, nil
}
