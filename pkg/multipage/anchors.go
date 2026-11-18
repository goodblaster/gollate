package multipage

import (
	"sort"
)

type gram [3]string
type occurrence struct{ page, offset int }

// Only unique sequences can supply an anchor. Repeated dialogue, running
// heads and common phrases must be resolved by their neighbors instead.
func indexGrams(pages [][]string) map[gram]occurrence {
	index := make(map[gram]occurrence)
	for p, words := range pages {
		for j := 0; j+3 <= len(words); j++ {
			key := gram{words[j], words[j+1], words[j+2]}
			if _, ok := index[key]; ok {
				index[key] = occurrence{page: -1}
			} else {
				index[key] = occurrence{p, j}
			}
		}
	}
	return index
}

type anchor struct {
	line, page                  int
	canonicalOffset, pageOffset float64
}

// Keep a longest nondecreasing sequence, not a greedy first match: one
// displaced quotation must not prevent all following anchors from landing.
func monotoneAnchors(in []anchor) []anchor {
	var tails []int
	prev := make([]int, len(in))
	for i, a := range in {
		at := sort.Search(len(tails), func(j int) bool { return in[tails[j]].page > a.page })
		prev[i] = -1
		if at > 0 {
			prev[i] = tails[at-1]
		}
		if at == len(tails) {
			tails = append(tails, i)
		} else {
			tails[at] = i
		}
	}
	out := make([]anchor, len(tails))
	if len(out) == 0 {
		return out
	}
	for i, j := len(out)-1, tails[len(tails)-1]; i >= 0; i-- {
		out[i] = in[j]
		j = prev[j]
	}
	return out
}

// anchoredPositions corrects the proportion-only prior using distinctive
// sequences. Entirely unsupported runs are left unassigned and contribute
// no positional cost. Short isolated failures still interpolate normally:
// they may simply be a badly recognized visual line.
func anchoredPositions(lines, pages [][]string, expected []int, docFreq map[string]int, index map[gram]occurrence, coverage func(int, int) float64) []bool {
	skip := make([]bool, len(lines))
	weak := make([]bool, len(lines))
	var candidates []anchor
	for i, line := range lines {
		absent := 0
		for _, w := range line {
			if docFreq[w] == 0 {
				absent++
			}
		}
		weak[i] = len(line) > 0 && absent*5 >= len(line)*3
		votes := make(map[int]int)
		for j := 0; j+3 <= len(line); j++ {
			if a, ok := index[gram{line[j], line[j+1], line[j+2]}]; ok && a.page >= 0 {
				votes[a.page]++
			}
		}
		best, n, runner := -1, 0, 0
		// Iterate pages deterministically; tied candidates never anchor.
		for p := range pages {
			v := votes[p]
			if v > n {
				runner = n
				best, n = p, v
			} else if v > runner {
				runner = v
			}
		}
		if n >= 2 && n >= 2*runner+1 && coverage(i, best) >= .8 {
			a := anchor{line: i, page: best}
			for j := 0; j+3 <= len(line); j++ {
				if at, ok := index[gram{line[j], line[j+1], line[j+2]}]; ok && at.page == best {
					a.canonicalOffset += float64(j) + 1.5
					a.pageOffset += float64(at.offset) + 1.5
				}
			}
			a.canonicalOffset /= float64(n)
			a.pageOffset /= float64(n)
			candidates = append(candidates, a)
			weak[i] = false
		}
	}
	for i := 0; i < len(lines); {
		if !weak[i] {
			i++
			continue
		}
		j := i + 1
		for j < len(lines) && (weak[j] || len(lines[j]) == 0) {
			j++
		}
		count := 0
		for k := i; k < j; k++ {
			if weak[k] {
				count++
			}
		}
		if count >= 3 {
			for k := i; k < j; k++ {
				skip[k] = weak[k]
			}
		}
		i = j
	}
	anchors := monotoneAnchors(candidates)
	if len(anchors) == 0 {
		return skip
	}
	// Token midpoints preserve proportional interpolation between anchors,
	// but canonical-only runs consume no position in the physical document.
	pos := make([]float64, len(lines))
	sum := 0.0
	for i, line := range lines {
		n := float64(len(line))
		if skip[i] {
			n = 0
		}
		pos[i] = sum + n/2
		sum += n
	}
	pageEnd := make([]float64, len(pages))
	sum = 0
	for p, words := range pages {
		sum += float64(len(words))
		pageEnd[p] = sum
	}
	source := make([]float64, len(anchors))
	target := make([]float64, len(anchors))
	for j, a := range anchors {
		source[j] = pos[a.line] - float64(len(lines[a.line]))/2 + a.canonicalOffset
		target[j] = a.pageOffset
		if a.page > 0 {
			target[j] += pageEnd[a.page-1]
		}
	}
	right := 0
	for i := range lines {
		for right < len(anchors) && source[right] < pos[i] {
			right++
		}
		projected := 0.0
		switch {
		case right == 0:
			projected = target[0] + pos[i] - source[0]
		case right == len(anchors):
			projected = target[right-1] + pos[i] - source[right-1]
		default:
			fraction := 0.0
			if source[right] > source[right-1] {
				fraction = (pos[i] - source[right-1]) / (source[right] - source[right-1])
			}
			projected = target[right-1] + fraction*(target[right]-target[right-1])
		}
		expected[i] = min(len(pages)-1, sort.Search(len(pages), func(p int) bool { return pageEnd[p] >= projected }))
	}
	for _, a := range anchors {
		expected[a.line] = a.page
	}
	return skip
}

// longFragments recognizes paragraphs traversing several seams or a blank
// page using a monotone consensus of unique sequence anchors. Ordinary one-seam
// cases retain
// the established bag/sequence arbitration in Segment. Page edges pin
// the cuts; unsupported or crossing anchors do not manufacture fragments.
func longFragments(line []string, pages [][]string, index map[gram]occurrence) []Fragment {
	type run struct{ page, first, last, firstOffset, lastOffset int }
	var runs []run
	// A phrase unique in OCR is not necessarily unique in the reference:
	// another occurrence may have been damaged or broken across lines.
	counts := make(map[gram]int)
	for i := 0; i+3 <= len(line); i++ {
		counts[gram{line[i], line[i+1], line[i+2]}]++
	}
	var candidates []anchor
	for i := 0; i+3 <= len(line); i++ {
		key := gram{line[i], line[i+1], line[i+2]}
		a, ok := index[key]
		if !ok || a.page < 0 || counts[key] != 1 {
			continue
		}
		candidates = append(candidates, anchor{page: a.page, canonicalOffset: float64(i), pageOffset: float64(a.offset)})
	}
	// A displaced quotation or one accidental match must not veto hundreds
	// of consistent anchors on the actual pages.
	for _, a := range monotoneAnchors(candidates) {
		i, offset := int(a.canonicalOffset), int(a.pageOffset)
		if len(runs) > 0 {
			last := &runs[len(runs)-1]
			if a.page == last.page {
				last.last = i
				last.lastOffset = offset
				continue
			}
		}
		runs = append(runs, run{a.page, i, i, offset, offset})
	}
	// Require two distinct anchors per page; isolated matches at the
	// ends of the monotone chain are not evidence for an extra fragment.
	strong := runs[:0]
	for _, r := range runs {
		if r.last > r.first {
			strong = append(strong, r)
		}
	}
	runs = strong
	if len(runs) < 2 || (len(runs) == 2 && runs[1].page == runs[0].page+1) {
		return nil
	}
	var fragments []Fragment
	start := 0
	for j := 0; j+1 < len(runs); j++ {
		a, b := runs[j], runs[j+1]
		lo, hi := a.last+3, b.first
		if lo > hi {
			return nil
		}
		// Extrapolate from the closest exact sequences to the actual page
		// edges. With insertions/deletions, choose the midpoint of the two
		// estimates, clamped between the supporting anchors.
		fromHead := a.last + len(pages[a.page]) - a.lastOffset
		fromTail := b.first - b.firstOffset
		cut := max(lo, min(hi, (fromHead+fromTail)/2))
		if cut <= start {
			return nil
		}
		fragments = append(fragments, Fragment{Page: a.page, Start: start, End: cut})
		start = cut
	}
	fragments = append(fragments, Fragment{Page: runs[len(runs)-1].page, Start: start, End: len(line)})
	return fragments
}
