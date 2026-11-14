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
func stripFurniture(pages []Page) [][]Furniture {
	found := make([][]Furniture, len(pages))
	if len(pages) < 2 {
		return found
	}
	for _, top := range []bool{true, false} {
		lines := make([]edgeLine, len(pages))
		ok := make([]bool, len(pages))
		for p := range pages {
			lines[p], ok[p] = findEdgeLine(pages[p].Blocks, top)
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
				if lines[p].text == lines[q].text || (label && pageLabelRe.MatchString(lines[q].text)) {
					isFurniture[p] = true
					break
				}
			}
		}
		for p := range pages {
			if !isFurniture[p] {
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
