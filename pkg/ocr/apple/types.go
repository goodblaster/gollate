//go:build darwin

package apple

// Word represents a single word with its bounding box coordinates.
// Coordinates are normalized to 0-1 range.
type Word struct {
	Text   string  `json:"text"`
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Rect represents a bounding box.
type Rect struct {
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Line represents a line of text with its bounding box and words.
type Line struct {
	LineText   string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Rect       Rect    `json:"rect"`
	Words      []Word  `json:"words"`
}
