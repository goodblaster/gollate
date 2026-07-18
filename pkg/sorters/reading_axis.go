package sorters

import "math"

// Reading-axis helpers for post-processing. The leftover assembly pipeline
// historically assumed horizontal reading: paragraph gaps were measured
// vertically and sentence fragments ordered by top position. Under a
// vertical reading order both assumptions scramble output that arrives in
// perfect emit order (vertical fixtures ride emit order through leftover
// assembly — see TESTING.md issue #4). These helpers express the same
// logic along the configured reading order's advance axis.

// blankSentencePos sorts blank-line sentinels ahead of any real position
// on every axis (positions are page fractions or their negations, so
// they always exceed -inf).
var blankSentencePos = math.Inf(-1)

// sentencePos returns the ordering key for a sentence starting at block b:
// reading earlier means a smaller key. Horizontal: higher on the page.
// Vertical RTL: further right (negated left edge). Vertical LTR: further
// left.
func sentencePos(b Block, order ReadingOrder) float64 {
	switch order {
	case VerticalTTB_RTL:
		return -(b.BoundingBox.Left + b.BoundingBox.Width)
	case VerticalTTB_LTR:
		return b.BoundingBox.Left
	default:
		return b.BoundingBox.Top
	}
}

// sentencePosTiebreak orders sentences that share a primary position:
// for vertical text, several sentences can start in the same column, and
// within a column reading proceeds top-down. Horizontal keeps no
// tiebreak (0), preserving the pipeline's historical comparator exactly.
func sentencePosTiebreak(b Block, order ReadingOrder) float64 {
	if order == VerticalTTB_RTL || order == VerticalTTB_LTR {
		return b.BoundingBox.Top
	}
	return 0
}

// advanceGap returns the whitespace between consecutive blocks along the
// axis that reading advances across lines: vertical distance for
// horizontal text (next line is below), horizontal distance for vertical
// text (next column is beside). Large values mean a paragraph break.
func advanceGap(last, next Block, order ReadingOrder) float64 {
	switch order {
	case VerticalTTB_RTL:
		return last.BoundingBox.Left - (next.BoundingBox.Left + next.BoundingBox.Width)
	case VerticalTTB_LTR:
		return next.BoundingBox.Left - (last.BoundingBox.Left + last.BoundingBox.Width)
	default:
		return next.BoundingBox.Top - (last.BoundingBox.Top + last.BoundingBox.Height)
	}
}

// advanceRef returns the block size used to scale advanceGap thresholds:
// heights for horizontal text, widths (column pitch) for vertical.
func advanceRef(a, b Block, order ReadingOrder) float64 {
	if order == VerticalTTB_RTL || order == VerticalTTB_LTR {
		return math.Max(a.BoundingBox.Width, b.BoundingBox.Width)
	}
	return math.Max(a.BoundingBox.Height, b.BoundingBox.Height)
}
