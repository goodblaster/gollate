package ocr

import "github.com/goodblaster/gollate/pkg/engines/tesseract"

// FromTesseract converts a Tesseract OCR block to the common Block format.
func FromTesseract(tess tesseract.Block, pageWidth, pageHeight int) Block {
	return Block{
		Text: tess.Text,
		BoundingBox: BoundingBox{
			Left:   float64(tess.Left) / float64(pageWidth),
			Top:    float64(tess.Top) / float64(pageHeight),
			Width:  float64(tess.Width) / float64(pageWidth),
			Height: float64(tess.Height) / float64(pageHeight),
		},
		Confidence: float64(tess.Confidence) / 100,
		Extractor:  tess.Engine(),
		Original:   tess,
		PageWidth:  pageWidth,
		PageHeight: pageHeight,
	}
}
