package sorters

import "math"

// Vertical text detection (on by default, DisableVerticalDetection to turn
// off). Orientation must be inferred from block geometry, never supplied by
// the caller (the language-only-hint rule): within the OCR engine's own
// line grouping, horizontal text advances along x while vertical text
// (Japanese tategaki) stacks along y. When a clear majority of multi-block
// engine lines flow vertically, the reading order switches to
// VerticalTTB_RTL (columns right to left, the tategaki convention).
//
// Requires line data (LineId); without it there is no flow signal and the
// configured reading order stands. Note the OCR side matters just as much:
// Tesseract needs jpn_vert in its language list to read tategaki at all,
// and Apple Vision cannot read it (an engine limitation no sorter change
// can compensate for).

// verticalMinLines is the minimum number of vertically-flowing engine lines
// required before switching; verticalMajority is how dominant they must be.
const (
	verticalMinLines = 3
	verticalMajority = 2.0 // vertical lines must outnumber horizontal 2:1
)

func detectVerticalText(blocks []Block) bool {
	groups := make(map[string][]Block)
	for _, b := range blocks {
		if b.LineId != "" {
			groups[b.LineId] = append(groups[b.LineId], b)
		}
	}
	if len(groups) == 0 {
		// No engine line grouping at all (e.g. PDF text layers): fall
		// back to emit-order flow.
		return detectVerticalFlow(blocks)
	}

	vertical, horizontal := 0, 0
	for _, group := range groups {
		if len(group) < 3 {
			continue
		}
		var dx, dy float64
		for i := 1; i < len(group); i++ {
			x1, y1 := group[i-1].Center()
			x2, y2 := group[i].Center()
			dx += math.Abs(x2 - x1)
			dy += math.Abs(y2 - y1)
		}
		if dy > dx {
			vertical++
		} else {
			horizontal++
		}
	}

	return vertical >= verticalMinLines && float64(vertical) >= verticalMajority*float64(horizontal)
}

// verticalMinFlowPairs is the minimum number of vertically-flowing
// consecutive block pairs the fallback needs before switching; higher
// than verticalMinLines because single pairs are weaker evidence than
// whole engine lines.
const verticalMinFlowPairs = 30

// detectVerticalFlow infers orientation from consecutive emit-order pairs
// when the engine provides no line grouping. Emit order is reading order
// by the adapter contract, so in vertical text consecutive tokens are
// squarish glyphs stacked downward with horizontal overlap, while in
// horizontal text they step rightward with vertical overlap. The aspect
// guard keeps wide tokens (whole horizontal phrases, as PDF text layers
// emit for CJK) from counting as vertical flow: their downward steps are
// line breaks, not column flow.
func detectVerticalFlow(blocks []Block) bool {
	const maxVerticalAspect = 1.6

	vertical, horizontal := 0, 0
	for i := 1; i < len(blocks); i++ {
		a, b := blocks[i-1], blocks[i]

		yOverlap := a.Top() < b.Bottom() && b.Top() < a.Bottom()
		xOverlap := a.Left() < b.Right() && b.Left() < a.Right()

		if yOverlap && !xOverlap && b.Left() >= a.Left() {
			horizontal++
			continue
		}
		squarish := a.Height() > 0 && b.Height() > 0 &&
			a.Width()/a.Height() < maxVerticalAspect &&
			b.Width()/b.Height() < maxVerticalAspect
		if xOverlap && !yOverlap && b.Top() > a.Top() && squarish {
			vertical++
		}
	}

	return vertical >= verticalMinFlowPairs && float64(vertical) >= verticalMajority*float64(horizontal)
}
