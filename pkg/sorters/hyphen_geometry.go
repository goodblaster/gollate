package sorters

import (
	"math"
	"strings"
)

// Exact neighbors can distinguish be-/cause from a genuine "be cause"
// phrase elsewhere in the reference. Both flanks must stay on their OCR
// lines, and the split alternative must not occur in the same context.
func hyphenContextAgrees(blocks []Block, head, tail int, sequences [][]string, joined string) bool {
	if head == 0 || tail+1 >= len(blocks) {
		return false
	}
	sameLine := func(a, b Block) bool {
		return a.LineId != "" && a.LineId == b.LineId && a.Left() < b.Left()
	}
	if !sameLine(blocks[head-1], blocks[head]) || !sameLine(blocks[tail], blocks[tail+1]) {
		return false
	}
	prev, next := blocks[head-1].NormedText, blocks[tail+1].NormedText
	word, ok := uniqueMiddle(sequences, prev, next)
	if !ok || word != joined {
		return false
	}
	stem := NormalizeText(strings.TrimRight(strings.TrimSpace(blocks[head].Text), lineBreakHyphens))
	for _, seq := range sequences {
		for i := 0; i+3 < len(seq); i++ {
			if seq[i] == prev && seq[i+1] == stem && seq[i+2] == blocks[tail].NormedText && seq[i+3] == next {
				return false
			}
		}
	}
	return true
}

// wrappedSuccessors uses row membership and geometry to find the next visual
// line in the same column. Adjacent engine lines need not be consecutive in
// reading order, and a line can combine columns. This lookup does not reorder
// blocks or change their coordinates.
// Only hyphenated line ends need this lookup.
func wrappedSuccessors(blocks []Block, heads []int) map[int]int {
	left := make(map[int]int)
	lineHeights := make(map[string]float64)
	for _, b := range blocks {
		if b.LineId != "" {
			lineHeights[b.LineId] = max(lineHeights[b.LineId], b.Height())
		}
	}
	charHeight := func(b Block) float64 {
		h := max(b.Height(), lineHeights[b.LineId])
		if b.PageWidth > 0 && b.PageHeight > 0 {
			h *= float64(b.PageHeight) / float64(b.PageWidth)
		}
		return h
	}
	neighbor := func(i int, before bool) int {
		if j, ok := left[i]; before && ok {
			return j
		}
		a := blocks[i]
		best, gap := -1, math.Inf(1)
		for j, b := range blocks {
			if i == j || a.Page != b.Page || b.Height() <= 0 {
				continue
			}
			if a.LineId != "" && b.LineId != "" && a.LineId != b.LineId {
				continue // neighboring columns may share a baseline, not a line
			}
			overlap := min(a.Bottom(), b.Bottom()) - max(a.Top(), b.Top())
			// Lowercase glyphs, capitals and descenders have different
			// tight boxes. Requiring half of the taller box can split one
			// real line into several false starts.
			if overlap < .5*min(a.Height(), b.Height()) {
				continue
			}
			d := b.Left() - a.Right()
			if before {
				d = a.Left() - b.Right()
			}
			if d >= -.002 && d <= 1.2*max(charHeight(a), charHeight(b)) && d < gap {
				best, gap = j, d
			}
		}
		if before {
			left[i] = best
		}
		return best
	}
	out := make(map[int]int)
	for _, i := range heads {
		b := blocks[i]
		if b.Height() <= 0 || neighbor(i, false) >= 0 {
			continue // not a visual line end
		}
		start := i
		for steps := 0; steps < len(blocks); steps++ {
			j := neighbor(start, true)
			if j < 0 || blocks[j].Left() >= blocks[start].Left() {
				break
			}
			start = j
		}
		best, dy := -1, math.Inf(1)
		for j, next := range blocks {
			if next.Page != b.Page || next.Left() >= b.Left() || next.Height() <= 0 {
				continue
			}
			d := next.Top() - b.Top()
			if d < .5*b.Height() || d > 2.5*max(b.Height(), next.Height())+1e-9 || math.Abs(next.Left()-blocks[start].Left()) > 1.5*charHeight(b) {
				continue
			}
			if d < dy && neighbor(j, true) < 0 {
				best, dy = j, d
			}
		}
		if best >= 0 {
			out[i] = best
		}
	}
	return out
}
