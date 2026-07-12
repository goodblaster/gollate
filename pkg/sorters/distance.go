package sorters

import (
	"fmt"
	"math"
	"strings"
)

const (
	MaxWordDistance     = 10.0
	MaxSentenceDistance = 30.0
	BaseSequential      = 0.1
	BaseLineWrap        = 1.0
	BaseNextColumn      = 25.0 // Strong penalty for column/line jumps to prefer spatial proximity
	BaseDefault         = 100.0
)

func MaxNextLineHeight(w1, w2 Block) float64 {
	//return math.Max(w1.Height(), w2.Height()) * 1.25
	return (w1.Height() + w2.Height()) * 1.1
}

func MaxLineGap(w1, w2 Block) float64 {
	return math.Max(w1.Height(), w2.Height()) * 2.0
}

func (s *Sorter) PrintDistances(nodes []int) {
	var ds []float64
	var sum float64
	for i := 0; i < len(nodes)-1; i++ {
		dist := Distance(s.input[nodes[i]], s.input[nodes[i+1]])
		ds = append(ds, dist)
		sum += dist
	}
	fmt.Println(sum, ds)
}

// Distance calculates the spatial distance between two blocks.
// Lower values indicate blocks are more likely to be sequential in reading order.
// Uses HorizontalLTR_TTB (left-to-right, top-to-bottom) reading order.
// For other reading orders, use DistanceWithOrder.
func Distance(w1 Block, w2 Block) float64 {
	return DistanceWithOrder(w1, w2, HorizontalLTR_TTB)
}

// DistanceWithOrder calculates the spatial distance between two blocks
// respecting the specified reading order.
// Uses the default BaseNextColumn constant for column jump penalty.
func DistanceWithOrder(w1 Block, w2 Block, order ReadingOrder) float64 {
	return distanceWithPenalty(w1, w2, order, BaseNextColumn)
}

// distanceWithPenalty calculates the spatial distance between two blocks
// with a configurable column jump penalty.
func distanceWithPenalty(w1 Block, w2 Block, order ReadingOrder, columnPenalty float64) float64 {
	b1 := w1
	b2 := w2
	if w1.Engine() == "" {
		return 0
	}

	// On same line and maybe sequential.
	if onSameLineWithOrder(w1, w2, order) && maybeSequentialWithOrder(w1, w2, order) {
		primaryDist := primaryAxisDistance(w1, w2, order)
		maxDist := primaryAxisSize(w1, order) + primaryAxisSize(w2, order)
		if !withinDistance(w1, w2, maxDist) {
			return columnPenalty + primaryDist
		}
		return BaseSequential + primaryDist
	}

	debug := false
	if debug {
		fmt.Println("w1.Bottom()", w1.PixelBottom())
		fmt.Println("w2.Top()", w2.PixelTop())
		fmt.Println("w1.Right()", w1.PixelRight())
		fmt.Println("w2.Left()", w2.PixelLeft())
		fmt.Println("w2.Top()-w1.Bottom()", w2.PixelTop()-w1.PixelBottom())
		fmt.Println("MaxLineGap(w1, w2)", MaxLineGap(w1, w2))
	}

	// Wrapped to next line/column based on reading order
	if isWrappedToNextLine(w1, w2, order) {
		return BaseLineWrap + secondaryAxisDistance(w1, w2, order)
	}

	// Wrapped to next column/section based on reading order
	if isWrappedToNextColumn(w1, w2, order) {
		return columnPenalty + primaryAxisDistance(w1, w2, order)
	}

	// Default to physical distance on center with a large multiplier.
	// These will probably be thrown out every time.
	x1, y1 := b1.Center()
	x2, y2 := b2.Center()
	physical := math.Sqrt(math.Pow(x2-x1, 2) + math.Pow(y2-y1, 2))
	return BaseDefault + physical
}

// distance calculates the spatial distance between two blocks using the sorter's
// configured reading order and column jump penalty.
func (s *Sorter) distance(w1, w2 Block) float64 {
	return distanceWithPenalty(w1, w2, s.config.ReadingOrder, s.config.ColumnJumpPenalty)
}

// primaryAxisDistance returns the distance along the primary reading axis.
func primaryAxisDistance(w1, w2 Block, order ReadingOrder) float64 {
	switch order {
	case HorizontalLTR_TTB:
		return w2.Left() - w1.Right() // Left to right
	case HorizontalRTL_TTB:
		return w1.Left() - w2.Right() // Right to left
	case VerticalTTB_RTL:
		return w2.Top() - w1.Bottom() // Top to bottom
	case VerticalTTB_LTR:
		return w2.Top() - w1.Bottom() // Top to bottom
	default:
		return w2.Left() - w1.Right()
	}
}

// secondaryAxisDistance returns the distance along the secondary reading axis.
func secondaryAxisDistance(w1, w2 Block, order ReadingOrder) float64 {
	switch order {
	case HorizontalLTR_TTB, HorizontalRTL_TTB:
		return w2.Top() - w1.Bottom() // Top to bottom
	case VerticalTTB_RTL:
		return w1.Left() - w2.Right() // Right to left
	case VerticalTTB_LTR:
		return w2.Left() - w1.Right() // Left to right
	default:
		return w2.Top() - w1.Bottom()
	}
}

// primaryAxisSize returns the block's size along the primary reading axis.
func primaryAxisSize(w Block, order ReadingOrder) float64 {
	if order.IsHorizontal() {
		return w.Width()
	}
	return w.Height()
}

// onSameLineWithOrder checks if two blocks are on the same line given reading order.
func onSameLineWithOrder(w1, w2 Block, order ReadingOrder) bool {
	if order.IsHorizontal() {
		return onSameLine(w1, w2) // Uses vertical alignment
	}
	// For vertical text, check horizontal alignment
	return math.Abs(w1.Left()-w2.Left()) < MaxNextLineHeight(w1, w2)
}

// maybeSequentialWithOrder checks if blocks might be sequential given reading order.
func maybeSequentialWithOrder(w1, w2 Block, order ReadingOrder) bool {
	switch order {
	case HorizontalLTR_TTB:
		return w2.Left() >= w1.Left() // w2 is to the right
	case HorizontalRTL_TTB:
		return w2.Right() <= w1.Right() // w2 is to the left
	case VerticalTTB_RTL, VerticalTTB_LTR:
		return w2.Top() >= w1.Top() // w2 is below
	default:
		return w2.Left() >= w1.Left()
	}
}

// isWrappedToNextLine checks if w2 is on the next line after w1.
func isWrappedToNextLine(w1, w2 Block, order ReadingOrder) bool {
	if order.IsHorizontal() {
		// Horizontal text: next line is below, and starts before w1 ends
		// Allow 3 pixel overlaps.
		return w1.PixelBottom() < w2.PixelTop()+3 &&
			w1.Right() > w2.Left() &&
			(w2.Top()-w1.Bottom()) <= MaxLineGap(w1, w2)
	}
	// Vertical text: next line is to the left (RTL) or right (LTR)
	if order == VerticalTTB_RTL {
		// Next column is to the left
		return w1.PixelRight() < w2.PixelLeft()+3 &&
			w1.Bottom() > w2.Top() &&
			(w1.Left()-w2.Right()) <= MaxLineGap(w1, w2)
	}
	// VerticalTTB_LTR: next column is to the right
	return w1.PixelLeft() > w2.PixelRight()-3 &&
		w1.Bottom() > w2.Top() &&
		(w2.Left()-w1.Right()) <= MaxLineGap(w1, w2)
}

// isWrappedToNextColumn checks if w2 is in the next column/section after w1.
func isWrappedToNextColumn(w1, w2 Block, order ReadingOrder) bool {
	switch order {
	case HorizontalLTR_TTB:
		// Traditional vertical wrap: up and to right (reading down left column, then up to right column top)
		wrapsUp := w2.Left() > w1.Right() && w2.Bottom() < w1.Top()

		// Horizontal column jump: significantly to the right at same vertical level
		// This handles side-by-side columns (newspaper layout)
		isRightward := w2.Left() > w1.Right()
		hasLargeGap := (w2.Left() - w1.Right()) > 0.15 // 15% page width gap indicates column boundary
		sameVerticalRegion := math.Abs(w2.Top()-w1.Top()) < MaxLineGap(w1, w2)
		horizontalJump := isRightward && hasLargeGap && sameVerticalRegion

		return wrapsUp || horizontalJump

	case HorizontalRTL_TTB:
		// Traditional vertical wrap: up and to left (reading down right column, then up to left column top)
		wrapsUp := w2.Right() < w1.Left() && w2.Bottom() < w1.Top()

		// Horizontal column jump: significantly to the left at same vertical level
		// This handles side-by-side columns (newspaper layout in RTL)
		isLeftward := w2.Right() < w1.Left()
		hasLargeGap := (w1.Left() - w2.Right()) > 0.15 // 15% page width gap indicates column boundary
		sameVerticalRegion := math.Abs(w2.Top()-w1.Top()) < MaxLineGap(w1, w2)
		horizontalJump := isLeftward && hasLargeGap && sameVerticalRegion

		return wrapsUp || horizontalJump

	case VerticalTTB_RTL:
		// Traditional wrap: to the left and up (reading down top-right column, then to top of next column left)
		wrapsUp := w2.Right() < w1.Left() && w2.Top() < w1.Top()

		// Vertical column jump: significantly to the left at same horizontal level
		// This handles side-by-side vertical columns (newspaper/magazine layout)
		// "Same horizontal level" for vertical text means similar TOP positions
		isLeftward := w2.Right() < w1.Left()
		hasLargeGap := (w1.Left() - w2.Right()) > 0.15 // 15% page width gap indicates column boundary
		sameHorizontalRegion := math.Abs(w2.Top()-w1.Top()) < MaxLineGap(w1, w2)
		verticalColumnJump := isLeftward && hasLargeGap && sameHorizontalRegion

		return wrapsUp || verticalColumnJump

	case VerticalTTB_LTR:
		// Traditional wrap: to the right and up (reading down top-left column, then to top of next column right)
		wrapsUp := w2.Left() > w1.Right() && w2.Top() < w1.Top()

		// Vertical column jump: significantly to the right at same horizontal level
		// This handles side-by-side vertical columns (newspaper/magazine layout)
		// "Same horizontal level" for vertical text means similar TOP positions
		isRightward := w2.Left() > w1.Right()
		hasLargeGap := (w2.Left() - w1.Right()) > 0.15 // 15% page width gap indicates column boundary
		sameHorizontalRegion := math.Abs(w2.Top()-w1.Top()) < MaxLineGap(w1, w2)
		verticalColumnJump := isRightward && hasLargeGap && sameHorizontalRegion

		return wrapsUp || verticalColumnJump

	default:
		return w2.Left() > w1.Right() && w2.Bottom() < w1.Top()
	}
}

func SentenceDistance(line1 []Block, line2 []Block) float64 {
	blockI := line1[len(line1)-1]
	blockJ := line2[0]
	return Distance(blockI, blockJ)
}

func withinDistance(w1, w2 Block, maxDistance float64) bool {
	x1, y1 := w1.Center()
	x2, y2 := w2.Center()
	physical := math.Sqrt(math.Pow(x2-x1, 2) + math.Pow(y2-y1, 2))
	return physical <= maxDistance
}

func mapBlocks(ocrWords []Block) map[string][]Block {
	mapped := make(map[string][]Block)
	for _, w := range ocrWords {
		if strings.TrimSpace(w.Text) == "" {
			continue
		}
		mapped[w.NormedText] = append(mapped[w.NormedText], w)
	}
	return mapped
}

func onSameLine(w1, w2 Block) bool {
	// Check if the two boxes overlap on the y-axis.
	return w1.Top() <= w2.Bottom() && w1.Bottom() >= w2.Top()
}
