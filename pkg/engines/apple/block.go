package apple

// Block represents a word from Apple Vision OCR with its position and confidence.
type Block struct {
	Text       string  `json:"text"`
	Top        float64 `json:"top"`
	Left       float64 `json:"left"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	Confidence float64 `json:"confidence"`
	LineNum    int     `json:"line_num"` // For line tracking
}

// Rect represents a bounding box for a line.
type Rect struct {
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Line represents a line of text from Apple Vision OCR.
// This matches the JSON structure returned by the Vision framework.
type Line struct {
	LineText   string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Rect       Rect    `json:"rect"`
	Words      []Block `json:"words"`
}
