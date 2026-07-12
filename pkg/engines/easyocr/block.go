package easyocr

// Block represents an EasyOCR block.
type Block struct {
	Text       string
	Boxes      [][2]int // Bounding box corners
	Confidence float64
}

// Engine returns the engine name.
func (b Block) Engine() string {
	return "easyocr"
}
