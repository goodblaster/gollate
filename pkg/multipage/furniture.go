package multipage

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// Page furniture is text printed on the page that belongs to the edition
// rather than the work: page numbers and running heads. A transcription
// (the canonical text) never contains it. Left in, it lands in the body at
// every page break - "…automatic since 1994. 1" - and a running head made
// of the book's title is claimed by the real title line.
//
// It is recognized here, before segmentation and sorting, from what makes
// it furniture: it is the top or bottom line of a page, and another page
// has a line at the same height that either reads the same (a running
// head) or is also a page label (a page number). Position and repetition
// are inferred from the blocks themselves, so this stays inside the rule
// that the caller supplies no layout hints.

// furnitureBand is how closely two pages' edge lines must agree in height
// (as a fraction of page height) to be the same piece of furniture.
const furnitureBand = 0.012

// pageLabelRe matches a page number as it prints: arabic or roman
// numerals, optionally after "page"/"p".
var pageLabelRe = regexp.MustCompile(`^(?:page|p)?\s*(?:[0-9]{1,4}|[ivxlcdm]{1,7})$`)

// similarText reports whether two edge lines read as the same running
// head, allowing for OCR misreads ("Tbe Keeper's Ledgers"): identical, or
// within a quarter of their length in edits.
func similarText(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	longest := max(len(ra), len(rb))
	if longest < 4 || min(len(ra), len(rb))*4 < longest*3 {
		return false
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]*4 <= longest
}

// edgeLine is the top or bottom visual line of a page.
type edgeLine struct {
	blocks []int   // indices into the page's blocks
	text   string  // normalized, in reading order
	y      float64 // vertical center
	bounds ocr.BoundingBox
}

// findEdgeLine returns the page's topmost (top=true) or bottommost line:
// the extreme block and every block that vertically overlaps it.
func findEdgeLine(blocks []ocr.Block, top bool) (edgeLine, bool) {
	extreme := -1
	for i, b := range blocks {
		if strings.TrimSpace(b.Text) == "" {
			continue
		}
		if extreme == -1 ||
			(top && b.Top() < blocks[extreme].Top()) ||
			(!top && b.Bottom() > blocks[extreme].Bottom()) {
			extreme = i
		}
	}
	if extreme == -1 {
		return edgeLine{}, false
	}
	ref := blocks[extreme]
	mid := ref.Top() + ref.Height()/2
	var line edgeLine
	left, right, lo, hi := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for i, b := range blocks {
		if strings.TrimSpace(b.Text) == "" || b.Top() > mid || b.Bottom() < mid {
			continue
		}
		line.blocks = append(line.blocks, i)
		left, right = math.Min(left, b.Left()), math.Max(right, b.Right())
		lo, hi = math.Min(lo, b.Top()), math.Max(hi, b.Bottom())
	}
	sort.Slice(line.blocks, func(a, b int) bool {
		return blocks[line.blocks[a]].Left() < blocks[line.blocks[b]].Left()
	})
	var words []string
	for _, i := range line.blocks {
		words = append(words, blocks[i].Text)
	}
	line.text = sorters.NormalizeText(strings.Join(words, " "))
	line.y = (lo + hi) / 2
	line.bounds = ocr.BoundingBox{Left: left, Top: lo, Width: right - left, Height: hi - lo}
	return line, line.text != ""
}

// stripFurniture removes page furniture from every page and returns it,
// per page. Pages are modified in place (their Blocks slices are replaced).
func stripFurniture(pages []Page, canonical ...string) [][]Furniture {
	// Canonical text wins over a layout heuristic. Repetition alone is
	// not enough to distinguish running heads from dialogue or headings.
	canonicalCounts := make(map[string]int)
	for _, line := range canonical {
		canonicalCounts[sorters.NormalizeText(line)]++
	}
	// Count body occurrences independently of the candidate margin lines.
	// A real title inside the body permits its repeated running heads to
	// be removed; a title printed only in a margin must be preserved.
	var body strings.Builder
	for _, page := range pages {
		var words []string
		for _, b := range page.Blocks {
			if b.Top() >= .08 && b.Bottom() <= .86 {
				words = append(words, b.Text)
			}
		}
		body.WriteString(" " + sorters.NormalizeText(strings.Join(words, " ")) + " ")
	}
	bodyText := body.String()
	protected := make(map[string]int)
	found := make([][]Furniture, len(pages))
	if len(pages) < 2 {
		return found
	}
	for _, top := range []bool{true, false} {
		lines := make([]edgeLine, len(pages))
		ok := make([]bool, len(pages))
		for p := range pages {
			lines[p], ok[p] = findEdgeLine(pages[p].Blocks, top)
			if !ok[p] {
				continue
			}
			line := lines[p]
			// Require an actual margin and a gap to the body. In particular,
			// an ordinary topmost body line is not automatically a header.
			if (top && line.bounds.Top+line.bounds.Height > 0.08) ||
				(!top && line.bounds.Top < 0.86) {
				ok[p] = false
				continue
			}
			edge := make(map[int]bool, len(line.blocks))
			for _, i := range line.blocks {
				edge[i] = true
			}
			gap := math.Inf(1)
			for i, b := range pages[p].Blocks {
				if edge[i] || strings.TrimSpace(b.Text) == "" {
					continue
				}
				if top {
					gap = min(gap, b.Top()-line.bounds.Top-line.bounds.Height)
				} else {
					gap = min(gap, line.bounds.Top-b.Bottom())
				}
			}
			ok[p] = gap > 0
		}
		isFurniture := make([]bool, len(pages))
		for p := range pages {
			if !ok[p] {
				continue
			}
			label := pageLabelRe.MatchString(lines[p].text)
			for q := range pages {
				if q == p || !ok[q] || math.Abs(lines[p].y-lines[q].y) > furnitureBand {
					continue
				}
				if similarText(lines[p].text, lines[q].text) || (label && pageLabelRe.MatchString(lines[q].text)) {
					isFurniture[p] = true
					break
				}
			}
		}
		for p := range pages {
			if !isFurniture[p] {
				continue
			}
			text := lines[p].text
			if _, known := protected[text]; !known {
				count := canonicalCounts[text]
				if count > 0 {
					count = max(0, count-strings.Count(bodyText, " "+text+" "))
				}
				protected[text] = count
			}
			if protected[text] > 0 {
				protected[text]--
				continue
			}
			drop := make(map[int]bool, len(lines[p].blocks))
			var words []string
			for _, i := range lines[p].blocks {
				drop[i] = true
				words = append(words, pages[p].Blocks[i].Text)
			}
			kept := make([]ocr.Block, 0, len(pages[p].Blocks)-len(drop))
			for i, b := range pages[p].Blocks {
				if !drop[i] {
					kept = append(kept, b)
				}
			}
			pages[p].Blocks = kept
			found[p] = append(found[p], Furniture{Text: strings.Join(words, " "), Bounds: lines[p].bounds})
		}
	}
	return found
}
