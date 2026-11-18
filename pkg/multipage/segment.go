// Package multipage sorts documents that span several pages.
//
// The single-page sorter (pkg/sorters) assumes one page: its geometry is
// page fractions and its pathfinding budget assumes one page's worth of
// duplicate words. This package sits above it. It decides which canonical
// lines live on which page (Segment), runs the single-page sorter on each
// page with just those lines, and merges the results into one document in
// canonical order, joining paragraphs that continue across a page break.
//
// Pages are never pooled: block Index and LineId are page-local, and each
// page keeps its own 0-1 coordinate space.
package multipage

import "math"

// Fragment is a half-open token range [Start, End) of a canonical line
// assigned to one page. Token indices use Segment's input tokenization.
type Fragment struct {
	Page  int
	Start int
	End   int
}

// Assignment places one canonical line on a page.
type Assignment struct {
	// Fragments covers the line in canonical order, on any number of pages.
	// Empty for unassigned lines and paragraph separators. The legacy fields
	// below summarize the first fragment and first boundary.
	Fragments []Fragment

	// Page is the 0-based page the line is on, or where it starts if it
	// straddles. -1 when unassigned (including when there are no pages).
	Page int

	// Straddle is true when the line starts on Page and finishes on
	// another page (possibly across several page boundaries).
	Straddle bool

	// Cut is, for a straddling line, the index of its first token that
	// falls on the second fragment's page.
	Cut int

	// Coverage is the rarity-weighted share of the line's tokens found on
	// its assigned pages. Near zero means there is little placement
	// evidence; the line may be unassigned or placed by its neighbors.
	Coverage float64
}

const (
	// positionWeight is the cost, in coverage units, of placing a line one
	// page away from where the anchors (or proportions) predict. It is a
	// tiebreaker: real coverage differences between the right page and a
	// wrong one are an order of magnitude larger.
	positionWeight = 0.02

	// straddleGainTokens is how many more of a line's tokens a two-page
	// split must account for than its best single page before the line is
	// called a straddle. A count, not a share: the part of a long
	// paragraph that spills onto the next page is often one short line.
	straddleGainTokens = 3

	// straddleMinTokens is the fewest tokens either side of a straddle
	// must hold; shorter than this and the "other half" is noise.
	straddleMinTokens = 2

	// seamGram is the length of the token run used to pin a straddle's cut:
	// the tokens just before the cut should be consecutive on the head
	// page and the tokens just after it consecutive on the tail page.
	seamGram = 3
)

// bandThreshold and bandWidth bound the work on long documents: past this
// many line-page pairs, each line is only scored against pages within
// bandWidth of its expected position, widening if unsupported. (Variables
// so tests can force the
// banded path on a small document.)
var (
	bandThreshold = 4_000_000
	bandWidth     = 8
)

// Segment assigns each canonical line to a page.
//
// lines[i] and pages[p] are token lists (matching keys, as the sorter
// would tokenize them). Assigned lines are monotone - a later line is never
// on an earlier page - because canonical order is reading order; that is
// the one structural guarantee available, and it is what places repeated
// lines and lines found nowhere.
//
// Unique token sequences anchor the positional estimate; unsupported runs
// can remain unassigned (Page -1). Each line is scored by how many tokens a page
// holds, weighted by rarity so that "the" says little and a proper noun
// says a lot, with a small penalty for straying from the anchored position. A dynamic program picks the monotone
// assignment with the best total. Lines at page boundaries are then tested
// for straddling. Unique sequences also locate paragraphs spanning three or
// more pages. Fragments, rather than the legacy first-boundary summary,
// describe every page of those paragraphs.
func Segment(lines, pages [][]string) []Assignment {
	out := make([]Assignment, len(lines))
	if len(pages) == 0 {
		for i := range out {
			out[i].Page = -1
		}
		return out
	}

	// Per-page token counts and rarity weights.
	counts := make([]map[string]int, len(pages))
	docFreq := make(map[string]int)
	pageTokens := 0
	for p, tokens := range pages {
		counts[p] = make(map[string]int, len(tokens))
		for _, t := range tokens {
			if counts[p][t] == 0 {
				docFreq[t]++
			}
			counts[p][t]++
		}
		pageTokens += len(tokens)
	}
	weights := make(map[string]float64, len(docFreq))
	for token, df := range docFreq {
		weights[token] = math.Log(1 + float64(len(pages))/float64(df))
	}
	weight := func(token string) float64 {
		if w, ok := weights[token]; ok {
			return w
		}
		return math.Ln2 // absent everywhere: no placement signal
	}
	// Build each line's multiset once, in token order. Previously every
	// line/page score rebuilt it and recomputed every logarithm.
	type needToken struct {
		text   string
		count  int
		weight float64
	}
	needs := make([][]needToken, len(lines))
	totals := make([]float64, len(lines))
	for i, line := range lines {
		positions := make(map[string]int, len(line))
		for _, t := range line {
			if j, ok := positions[t]; ok {
				needs[i][j].count++
			} else {
				positions[t] = len(needs[i])
				needs[i] = append(needs[i], needToken{t, 1, weight(t)})
			}
			totals[i] += weight(t)
		}
	}
	coverage := func(line, page int) float64 {
		if totals[line] == 0 {
			return 0
		}
		hit := 0.0
		for _, t := range needs[line] {
			hit += t.weight * float64(min(t.count, counts[page][t.text]))
		}
		return hit / totals[line]
	}

	// Expected page of each line, from cumulative proportions: the line's
	// midpoint through the canonical text, mapped onto the pages' share of
	// all page tokens.
	lineTokens := 0
	for _, line := range lines {
		lineTokens += len(line)
	}
	pageEnd := make([]float64, len(pages)) // cumulative fraction at each page's end
	running := 0
	for p, tokens := range pages {
		running += len(tokens)
		if pageTokens > 0 {
			pageEnd[p] = float64(running) / float64(pageTokens)
		} else {
			pageEnd[p] = float64(p+1) / float64(len(pages))
		}
	}
	expected := make([]int, len(lines))
	running, page := 0, 0
	for i, line := range lines {
		mid := 0.5
		if lineTokens > 0 {
			mid = (float64(running) + float64(len(line))/2) / float64(lineTokens)
		}
		for page < len(pages)-1 && mid > pageEnd[page] {
			page++
		}
		expected[i] = page
		running += len(line)
	}

	gramIndex := indexGrams(pages)
	skip := anchoredPositions(lines, pages, expected, docFreq, gramIndex, coverage)
	banded := len(lines)*len(pages) > bandThreshold

	// Monotone alignment. best[p] is the best total for the lines so far
	// with the current line on page p; from stores the previous page only
	// inside each row's searched band.
	neg := math.Inf(-1)
	best := make([]float64, len(pages))
	from := make([][]int32, len(lines))
	rowStart := make([]int, len(lines))
	next := make([]float64, len(pages))
	for i, line := range lines {
		if skip[i] {
			continue
		}
		// Short lines are weak evidence; do not let "Buy" outvote a paragraph.
		confidence := math.Min(1, float64(len(line))/5)
		lo, hi := 0, len(pages)
		if banded {
			lo = max(0, expected[i]-bandWidth)
			hi = min(len(pages), expected[i]+bandWidth+1)
		}
		// Widen an unsupported band instead of making an exclusion final.
		// Strong sequences normally keep the narrow path sufficient.
		if banded {
			evidence := 0.0
			for p := lo; p < hi; p++ {
				evidence = max(evidence, coverage(i, p))
			}
			if evidence < .35 {
				lo, hi = 0, len(pages)
			}
		}
		rowStart[i] = lo
		from[i] = make([]int32, hi-lo)
		runBest, runArg := neg, 0
		for p := range pages {
			prev := 0.0
			if i > 0 {
				prev = best[p]
			}
			if prev > runBest {
				runBest, runArg = prev, p
			}
			if p < lo || p >= hi {
				next[p] = neg
				continue
			}
			score := confidence*coverage(i, p) - positionWeight*math.Abs(float64(p-expected[i]))
			next[p] = runBest + score
			from[i][p-lo] = int32(runArg)
		}
		best, next = next, best
	}
	end := 0
	for p := range pages {
		if best[p] > best[end] {
			end = p
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if skip[i] {
			out[i].Page = -1
			continue
		}
		out[i].Page = end
		out[i].Coverage = coverage(i, end)
		end = int(from[i][end-rowStart[i]])
	}

	// Pin long paragraphs before the one-seam search, which would otherwise
	// claim only two pages and can spend cubic time trying all tail cuts.
	for i, line := range lines {
		if skip[i] {
			continue
		}
		if fragments := longFragments(line, pages, gramIndex); fragments != nil {
			out[i].Fragments = fragments
			out[i].Page, out[i].Straddle, out[i].Cut = fragments[0].Page, true, fragments[0].End
			hit := 0.0
			for _, f := range fragments {
				used := make(map[string]int)
				for _, token := range line[f.Start:f.End] {
					if used[token] < counts[f.Page][token] {
						hit += weight(token)
						used[token]++
					}
				}
			}
			out[i].Coverage = hit / totals[i]
		}
	}

	// Straddles. Only a line at a page boundary can be one: the last
	// non-empty line placed on a page, or the first placed on the next.
	firstOn := make(map[int]int)
	lastOn := make(map[int]int)
	for i, line := range lines {
		if len(line) == 0 || out[i].Page < 0 {
			continue
		}
		p := out[i].Page
		if _, ok := firstOn[p]; !ok {
			firstOn[p] = i
		}
		lastOn[p] = i
	}
	// hasRun reports whether run occurs as consecutive tokens on page.
	hasRun := func(page int, run []string) bool {
		tokens := pages[page]
	scan:
		for i := 0; i+len(run) <= len(tokens); i++ {
			for j, t := range run {
				if tokens[i+j] != t {
					continue scan
				}
			}
			return true
		}
		return false
	}
	trySplit := func(i, head int) {
		line := lines[i]
		if head < 0 || head+1 >= len(pages) || len(line) < 2*straddleMinTokens || out[i].Straddle {
			return
		}
		// prefix[k]: tokens [0,k) found on the head page; suffix[k]: tokens
		// [k,n) found on the tail page - as rarity weight (W) and as a
		// plain count (N). Each page's supply of a repeated token is
		// consumed in order.
		n := len(line)
		prefixW, suffixW := make([]float64, n+1), make([]float64, n+1)
		prefixN, suffixN := make([]int, n+1), make([]int, n+1)
		used := make(map[string]int)
		total := 0.0
		for k, t := range line {
			w := weight(t)
			total += w
			prefixW[k+1], prefixN[k+1] = prefixW[k], prefixN[k]
			if used[t] < counts[head][t] {
				used[t]++
				prefixW[k+1] += w
				prefixN[k+1]++
			}
		}
		clear(used)
		for k := n - 1; k >= 0; k-- {
			t := line[k]
			suffixW[k], suffixN[k] = suffixW[k+1], suffixN[k+1]
			if used[t] < counts[head+1][t] {
				used[t]++
				suffixW[k] += weight(t)
				suffixN[k]++
			}
		}
		if total == 0 {
			return
		}
		bestScore := 0.0
		for k := straddleMinTokens; k <= n-straddleMinTokens; k++ {
			bestScore = math.Max(bestScore, (prefixW[k]+suffixW[k])/total)
		}

		// Words near the break that both pages happen to contain leave a
		// range of equally good cuts. The true one is where the text
		// actually runs off the head page and resumes on the tail page:
		// the tokens before it are consecutive on one, those after it
		// consecutive on the other.
		bestCut, bestSeam := -1, -1
		for k := straddleMinTokens; k <= n-straddleMinTokens; k++ {
			if (prefixW[k]+suffixW[k])/total < bestScore-1e-9 {
				continue
			}
			seam := 0
			if hasRun(head, line[max(0, k-seamGram):k]) {
				seam++
			}
			if hasRun(head+1, line[k:min(n, k+seamGram)]) {
				seam++
			}
			if seam > bestSeam {
				bestCut, bestSeam = k, seam
			}
		}
		if bestCut < 0 {
			return
		}
		// With the break pinned on both sides, less extra coverage is
		// needed: a short tail of mostly common words ("the window, and
		// waited for the first ship") adds few tokens its page alone has.
		need := straddleGainTokens
		if bestSeam == 2 {
			need = straddleGainTokens - 1
		}
		single := max(prefixN[n], suffixN[0])
		if prefixN[bestCut]+suffixN[bestCut]-single >= need {
			out[i] = Assignment{Page: head, Straddle: true, Cut: bestCut, Coverage: bestScore}
		}
	}
	// Sequence evidence, for when the bags are not enough (a short tail of
	// common words, or misreads on either side of the break): the head is
	// the text the head page ENDS with and the tail is the text the tail
	// page BEGINS with, in emit order. Each side must mostly line up -
	// exactly, when it is only a few tokens.
	trySequence := func(i, head int) {
		line := lines[i]
		n := len(line)
		if head < 0 || head+1 >= len(pages) || n < 3 || out[i].Straddle {
			return
		}
		prevPage, nextPage := pages[head], pages[head+1]
		if len(prevPage) == 0 || len(nextPage) == 0 {
			return
		}
		enough := func(matched, length int) bool {
			if length < 4 {
				return matched == length
			}
			return matched*4 >= length*3
		}
		bestCut, bestMatched := -1, 0
		for k := 1; k <= n-1; k++ {
			// A single word carried over the break (the last word of a
			// paragraph alone at the top of the next page, or the first
			// alone at the foot of this one) is too little to match by
			// itself. Accept it only when the pages break exactly there.
			if k < straddleMinTokens || n-k < straddleMinTokens {
				if prevPage[len(prevPage)-1] != line[k-1] || nextPage[0] != line[k] {
					continue
				}
			}
			headWindow := prevPage[max(0, len(prevPage)-k-2):]
			tailWindow := nextPage[:min(len(nextPage), n-k+2)]
			headMatched := lcsCount(line[:k], headWindow)
			tailMatched := lcsCount(line[k:], tailWindow)
			if !enough(headMatched, k) || !enough(tailMatched, n-k) {
				continue
			}
			if headMatched+tailMatched > bestMatched {
				bestCut, bestMatched = k, headMatched+tailMatched
			}
		}
		if bestCut >= 0 {
			out[i] = Assignment{Page: head, Straddle: true, Cut: bestCut, Coverage: float64(bestMatched) / float64(n)}
		}
	}

	for p := 0; p+1 < len(pages); p++ {
		if i, ok := lastOn[p]; ok {
			trySplit(i, p)
			trySequence(i, p)
		}
		if i, ok := firstOn[p+1]; ok {
			trySequence(i, p)
		}
		if i, ok := firstOn[p+1]; ok {
			trySplit(i, p)
		}
	}

	for i, line := range lines {
		a := &out[i]
		if a.Page < 0 || len(line) == 0 || len(a.Fragments) > 0 {
			continue
		}
		if a.Straddle {
			a.Fragments = []Fragment{{a.Page, 0, a.Cut}, {a.Page + 1, a.Cut, len(line)}}
		} else {
			a.Fragments = []Fragment{{a.Page, 0, len(line)}}
		}
	}

	// Empty lines (paragraph separators) follow the line before them.
	for i, line := range lines {
		if len(line) == 0 && i > 0 {
			out[i] = Assignment{Page: out[i-1].Page}
			if out[i-1].Straddle {
				out[i].Page = out[i-1].Fragments[len(out[i-1].Fragments)-1].Page
			}
		}
	}
	return out
}

// lcsCount is the length of the longest common subsequence of a and b.
func lcsCount(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else {
				cur[j] = max(prev[j], cur[j-1])
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
