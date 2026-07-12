package easyocr

import (
	"encoding/json"
	"io"

	"github.com/goodblaster/errors"
)

// EasyOCRResult represents a single detection from EasyOCR.
// EasyOCR returns results as: [bbox, text, confidence]
// where bbox is [[x1,y1], [x2,y2], [x3,y3], [x4,y4]] (four corners)
type EasyOCRResult struct {
	BBox       [][2]float64 `json:"bbox"`       // Four corner points
	Text       string       `json:"text"`       // Detected text
	Confidence float64      `json:"confidence"` // 0-1 scale
}

// EasyOCRDocument represents the JSON structure from EasyOCR.
type EasyOCRDocument struct {
	Results []EasyOCRResult `json:"results"`
}

// Read parses EasyOCR JSON from a reader and returns blocks.
//
// Expected JSON format:
//
//	{
//	  "results": [
//	    {
//	      "bbox": [[x1,y1], [x2,y2], [x3,y3], [x4,y4]],
//	      "text": "Hello",
//	      "confidence": 0.95
//	    },
//	    ...
//	  ]
//	}
//
// The bbox contains four corner points of the detected text region.
// Coordinates are in pixels. Confidence is 0-1 scale.
func Read(r io.Reader) ([]Block, error) {
	var doc EasyOCRDocument
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, errors.Wrap(err, "failed to decode EasyOCR JSON")
	}

	if len(doc.Results) == 0 {
		return nil, errors.New("no results found in EasyOCR JSON")
	}

	blocks := make([]Block, 0, len(doc.Results))
	for _, result := range doc.Results {
		// Skip empty text
		if result.Text == "" {
			continue
		}

		// Validate bbox has 4 points
		if len(result.BBox) != 4 {
			continue
		}

		// Convert float64 bbox to int [][2]int
		boxes := make([][2]int, 4)
		for i := 0; i < 4; i++ {
			boxes[i][0] = int(result.BBox[i][0])
			boxes[i][1] = int(result.BBox[i][1])
		}

		blocks = append(blocks, Block{
			Text:       result.Text,
			Boxes:      boxes,
			Confidence: result.Confidence,
		})
	}

	return blocks, nil
}
