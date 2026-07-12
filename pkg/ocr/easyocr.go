package ocr

import "github.com/goodblaster/gollate/pkg/engines/easyocr"

// FromEasyOCR converts an EasyOCR block to the common Block format.
func FromEasyOCR(easy easyocr.Block, pageWidth, pageHeight int) Block {
	b := Block{
		Text: easy.Text,
		BoundingBox: BoundingBox{
			Left:   float64(easy.Boxes[0][0]+2) / float64(pageWidth),
			Top:    float64(easy.Boxes[0][1]+2) / float64(pageHeight),
			Width:  float64(easy.Boxes[2][0]-easy.Boxes[0][0]-5) / float64(pageWidth),
			Height: float64(easy.Boxes[2][1]-easy.Boxes[0][1]-5) / float64(pageHeight),
		},
		Confidence: easy.Confidence,
		Extractor:  easy.Engine(),
		Original:   easy,
		PageWidth:  pageWidth,
		PageHeight: pageHeight,
	}
	return b
}
