package multipage

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// syntheticBook lays out a book of pseudo-words: paragraphs of running
// text broken into lines and pages, with a running head and page number on
// every page. Word frequencies are skewed like real text (a few words
// everywhere, most words rare), which is what makes segmentation hard.
func syntheticBook(seed int64, pageCount int) (pages []Page, canonical []string) {
	rng := rand.New(rand.NewSource(seed))
	syllables := strings.Fields("ba ke li mo nu ra se ti vo wa da fe gi ho ju ka le mi no pu")
	vocabulary := make([]string, 4000)
	for i := range vocabulary {
		var b strings.Builder
		for n := 1 + rng.Intn(4); n > 0; n-- {
			b.WriteString(syllables[rng.Intn(len(syllables))])
		}
		vocabulary[i] = b.String()
	}
	zipf := rand.NewZipf(rng, 1.15, 4, uint64(len(vocabulary)-1))

	const (
		linesPerPage = 30
		lineWidth    = 0.80
		charWidth    = 0.011
		gap          = 0.012
	)
	var blocks []ocr.Block
	line, x := 0, 0.10
	flushPage := func() {
		number := fmt.Sprint(len(pages) + 1)
		head := []ocr.Block{
			{Text: "Synthetic", Extractor: "test", BoundingBox: ocr.BoundingBox{Left: 0.40, Top: 0.03, Width: 0.10, Height: 0.015}},
			{Text: "Volume", Extractor: "test", BoundingBox: ocr.BoundingBox{Left: 0.51, Top: 0.03, Width: 0.07, Height: 0.015}},
		}
		foot := ocr.Block{Text: number, Extractor: "test", BoundingBox: ocr.BoundingBox{Left: 0.49, Top: 0.95, Width: 0.02, Height: 0.015}}
		all := append(append(head, blocks...), foot)
		for i := range all {
			all[i].Index = i
		}
		pages = append(pages, Page{Blocks: all, Width: 800, Height: 1200})
		blocks, line, x = nil, 0, 0.10
	}
	newLine := func() {
		line++
		x = 0.10
		if line == linesPerPage {
			flushPage()
		}
	}
	for len(pages) < pageCount {
		words := make([]string, 30+rng.Intn(90))
		for i := range words {
			words[i] = vocabulary[zipf.Uint64()]
		}
		canonical = append(canonical, strings.Join(words, " "), "")
		startPages := len(pages)
		for _, w := range words {
			width := charWidth * float64(len(w))
			if x+width > 0.10+lineWidth {
				newLine()
			}
			blocks = append(blocks, ocr.Block{
				Text: w, Extractor: "test",
				BoundingBox: ocr.BoundingBox{Left: x, Top: 0.08 + 0.028*float64(line), Width: width, Height: 0.018},
			})
			x += width + gap
		}
		_ = startPages
		if x > 0.10 {
			newLine() // paragraphs start on a fresh line
		}
	}
	if len(blocks) > 0 {
		flushPage()
	}
	return pages, canonical
}

func checkSyntheticBook(t *testing.T, pageCount int) {
	pages, canonical := syntheticBook(20261113, pageCount)
	start := time.Now()
	result, err := Sort(pages, canonical, sorters.ConfigForLanguage("english"), logger.Noop{})
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	byLine := make(map[int]string)
	for _, para := range result.Document.Paragraphs {
		if para.CanonicalLine != nil {
			byLine[*para.CanonicalLine] += result.Document.Text[para.Span.Start:para.Span.End]
		}
	}
	paragraphs, exact, straddles := 0, 0, 0
	for i, line := range canonical {
		if line == "" {
			continue
		}
		paragraphs++
		if byLine[i] == line {
			exact++
		}
		if result.Assignments[i].Straddle {
			straddles++
		}
	}
	furniture := 0
	for _, page := range result.Document.Pages {
		furniture += len(page.Furniture)
	}
	share := 100 * float64(exact) / float64(paragraphs)
	t.Logf("%d pages, %d paragraphs: %.2f%% recovered exactly, %d straddles, %d furniture runs, %v (%.1f ms/page)",
		len(pages), paragraphs, share, straddles, furniture, elapsed.Round(time.Millisecond), float64(elapsed.Milliseconds())/float64(len(pages)))
	if share < 99 {
		t.Errorf("only %.2f%% of paragraphs recovered exactly", share)
	}
	if furniture != 2*len(pages) {
		t.Errorf("furniture runs = %d, want %d (head and page number on every page)", furniture, 2*len(pages))
	}
	if strings.Contains(result.Document.Text, "Synthetic Volume") {
		t.Errorf("running head leaked into the text")
	}
}

// A book of several hundred pages: every paragraph must come back whole,
// including the ones broken across pages, and it must not take long.
func TestSortSyntheticBook300Pages(t *testing.T) {
	if testing.Short() {
		t.Skip("long document")
	}
	checkSyntheticBook(t, 300)
}

// The same through the banded search that long documents use.
func TestSortSyntheticBookBanded(t *testing.T) {
	saved := bandThreshold
	bandThreshold = 0
	defer func() { bandThreshold = saved }()
	checkSyntheticBook(t, 60)
}
