package tesseract

import (
	"encoding/json"
	"io"

	"github.com/goodblaster/errors"
)

// TesseractWord represents a single word from Tesseract JSON output.
// Tesseract can output various formats; this supports the common bbox + text format.
type TesseractWord struct {
	Text       string  `json:"text"`
	LineNum    int     `json:"line_num"`
	Left       int     `json:"left"`
	Top        int     `json:"top"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Confidence float64 `json:"conf"` // 0-100 scale
}

// TesseractDocument represents the JSON structure from Tesseract.
type TesseractDocument struct {
	Words []TesseractWord `json:"words"`
}

// isLatinScript returns true if the text contains primarily Latin/ASCII characters
func isLatinScript(text string) bool {
	if len(text) == 0 {
		return false
	}
	latinCount := 0
	for _, r := range text {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			latinCount++
		}
	}
	// Consider it Latin if more than 50% of characters are ASCII letters
	return float64(latinCount)/float64(len([]rune(text))) > 0.5
}

// hasComplexScript returns true if the text contains CJK, Devanagari, Arabic, or other complex scripts
func hasComplexScript(text string) bool {
	for _, r := range text {
		// CJK
		if (r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
			(r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
			(r >= 0x3040 && r <= 0x309F) || // Hiragana
			(r >= 0x30A0 && r <= 0x30FF) { // Katakana
			return true
		}
		// Devanagari (Hindi, Sanskrit, etc.)
		if r >= 0x0900 && r <= 0x097F {
			return true
		}
		// Arabic
		if r >= 0x0600 && r <= 0x06FF {
			return true
		}
		// Bengali
		if r >= 0x0980 && r <= 0x09FF {
			return true
		}
		// Thai
		if r >= 0x0E00 && r <= 0x0E7F {
			return true
		}
	}
	return false
}

// Read parses Tesseract OCR JSON from a reader and returns blocks.
//
// Expected JSON format:
//
//	{
//	  "words": [
//	    {
//	      "text": "Hello",
//	      "line_num": 0,
//	      "left": 100,
//	      "top": 200,
//	      "width": 50,
//	      "height": 20,
//	      "conf": 95.5
//	    },
//	    ...
//	  ]
//	}
//
// Coordinates are in pixels. Confidence is 0-100.
func Read(r io.Reader) ([]Block, error) {
	var doc TesseractDocument
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, errors.Wrap(err, "failed to decode Tesseract JSON")
	}

	if len(doc.Words) == 0 {
		return nil, errors.New("no words found in Tesseract JSON")
	}

	// Detect if document is primarily complex script (CJK, Devanagari, Arabic, etc.)
	complexScriptCount := 0
	for _, word := range doc.Words {
		if hasComplexScript(word.Text) {
			complexScriptCount++
		}
	}
	isComplexScriptDocument := float64(complexScriptCount)/float64(len(doc.Words)) > 0.3

	blocks := make([]Block, 0, len(doc.Words))
	for _, word := range doc.Words {
		// Skip empty text
		if word.Text == "" {
			continue
		}

		// Skip very low confidence words (likely OCR garbage)
		if word.Confidence < 20.0 {
			continue
		}

		// For complex script documents (Hindi, Japanese, Arabic, etc.):
		// Filter out low-confidence Latin words that are likely OCR errors
		if isComplexScriptDocument && isLatinScript(word.Text) && word.Confidence < 50.0 {
			continue
		}

		blocks = append(blocks, Block{
			Text:       word.Text,
			LineNum:    word.LineNum,
			Left:       word.Left,
			Top:        word.Top,
			Width:      word.Width,
			Height:     word.Height,
			Confidence: int(word.Confidence), // Convert to int (0-100)
		})
	}

	return blocks, nil
}
