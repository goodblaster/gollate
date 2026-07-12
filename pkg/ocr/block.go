package ocr

// Block represents an OCR text block in a common format across all OCR engines.
// Engine-specific blocks (Apple, Tesseract, EasyOCR) are converted to this format.
//
// # Contract for engine adapters
//
// Anything producing Blocks for the sorter - a custom engines.Engine, the
// "blocks" input format, or direct library use - must satisfy:
//
//   - Word/token granularity: one Block per word (per character for CJK is
//     acceptable), never whole lines or paragraphs. Matching happens at the
//     word level.
//   - BoundingBox in page fractions (0-1), not pixels. PageWidth/PageHeight
//     carry the pixel dimensions; parts of wrap detection use both.
//   - Extractor must be non-empty. It is load-bearing: distance calculation
//     short-circuits for blocks with no engine name, silently disabling
//     pathfinding.
//   - Index must reflect OCR emit order (0..n-1). Emit order is an
//     algorithmic signal - candidate rotation and leftover assembly rely on
//     it - not an arbitrary ID.
//   - One page per sort. Multi-page documents are sorted page by page;
//     tall single pages should be sliced for OCR (see pkg/slicing) and
//     merged back into one coordinate space beforehand.
//   - Confidence normalized to 0-1 when available; LineId optionally
//     preserves the engine's own line grouping for downstream consumers.
type Block struct {
	Text          string      `json:"text"`
	NormedText    string      `json:"normed_text"`
	BoundingBox   BoundingBox `json:"bounds"`
	Confidence    float64     `json:"normalized_conf"`
	Extractor     string      `json:"engine"`
	Index         int         `json:"index"`
	OriginalIndex int         `json:"original_index"` // Index before normalization/splitting
	Original      any         `json:"original"`
	PageWidth     int         `json:"page_width"`
	PageHeight    int         `json:"page_height"`
	LineId        string      `json:"line_id,omitempty"`
	HyphenAfter   string      `json:"hyphen_after,omitempty"` // Hyphen that came after this word (for reconstruction)

	// Spelling correction fields (populated when corrections are detected)
	OriginalOcrText string  `json:"original_ocr_text,omitempty"` // Original OCR text before correction (if corrected)
	SuggestedText   string  `json:"suggested_text,omitempty"`    // Suggested corrected text
	CorrectionType  string  `json:"correction_type,omitempty"`   // How it was corrected: "suggested", etc.
	EditDistance    int     `json:"edit_distance,omitempty"`     // Edit distance from original to suggested
	Similarity      float64 `json:"similarity,omitempty"`        // Text similarity ratio (0-1)
}

func (b Block) String() string {
	return b.Text
}

func (b Block) Engine() string {
	return b.Extractor
}

type BoundingBox struct {
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Block  *Block  `json:"-"` // optional pointer for reference back to parent block
}

func (b Block) Top() float64 {
	return b.BoundingBox.Top
}

func (b Block) Left() float64 {
	return b.BoundingBox.Left
}

func (b Block) Width() float64 {
	return b.BoundingBox.Width
}

func (b Block) Height() float64 {
	return b.BoundingBox.Height
}

func (b Block) Right() float64 {
	return b.Left() + b.Width()
}

func (b Block) Bottom() float64 {
	return b.Top() + b.Height()
}

func (b Block) Center() (x, y float64) {
	return b.Left() + b.Width()/2, b.Top() + b.Height()/2
}

func (b Block) PixelWidth() int {
	return int(b.Width() * float64(b.PageWidth))
}

func (b Block) PixelHeight() int {
	return int(b.Height() * float64(b.PageHeight))
}

func (b Block) PixelTop() int {
	return int(b.Top() * float64(b.PageHeight))
}

func (b Block) PixelLeft() int {
	return int(b.Left() * float64(b.PageWidth))
}

func (b Block) PixelRight() int {
	return b.PixelLeft() + b.PixelWidth()
}

func (b Block) PixelBottom() int {
	return b.PixelTop() + b.PixelHeight()
}

func (b Block) PixelCenter() (x, y int) {
	return b.PixelLeft() + b.PixelWidth()/2, b.PixelTop() + b.PixelHeight()/2
}
