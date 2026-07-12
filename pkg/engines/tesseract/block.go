package tesseract

// Block represents a Tesseract OCR block.
type Block struct {
	Text       string
	LineNum    int
	Left       int
	Top        int
	Width      int
	Height     int
	Confidence int
}

// Engine returns the engine name.
func (b Block) Engine() string {
	return "tesseract"
}
