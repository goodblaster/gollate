package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	ocrengines "github.com/goodblaster/gollate/pkg/engines"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/multipage"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// Multi-page regression suite. Fixtures live in testdata/multipage/<name>/
// (scripts/generate-multipage-fixtures.sh): canonical.txt, info.json, and
// per-page block files <source>-<page>.json.
//
// Two scores are ratcheted per fixture and source:
//
//   - accuracy: order-sensitive LCS of the whole merged text against
//     canonical.txt, as in TestOCRSorting.
//   - paragraphs (key suffix "#paragraphs"): for each canonical paragraph,
//     the LCS share recovered inside the one output paragraph matched to
//     it, averaged. Whole-document order barely notices a paragraph cut in
//     two at a page break or a page number dropped into the text - a
//     single-column book scores 99% left in raw page order - so this is
//     the score that says whether page breaks were handled.
//   - precision (key suffix "#precision"): the share of output units that
//     are canonical text in order. Accuracy ignores extra output, so page
//     numbers and running heads leaking into the text cost it nothing;
//     they cost this.
const (
	multipageDir       = "../testdata/multipage"
	multipageBaselines = "../testdata/multipage/baselines.json"
)

type multipageInfo struct {
	Language string `json:"language"`
	Pages    []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"pages"`
}

// loadMultipage reads one fixture's pages for one block source.
func loadMultipage(dir, source string) (pages []multipage.Page, canonical []string, info multipageInfo, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "info.json"))
	if err != nil {
		return nil, nil, info, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, nil, info, fmt.Errorf("parsing info.json: %w", err)
	}
	text, err := os.ReadFile(filepath.Join(dir, "canonical.txt"))
	if err != nil {
		return nil, nil, info, err
	}
	canonical = strings.Split(string(text), "\n")

	engine := source
	if source == "pdftext" {
		engine = "blocks"
	}
	for i, size := range info.Pages {
		f, err := os.Open(filepath.Join(dir, fmt.Sprintf("%s-%d.json", source, i+1)))
		if err != nil {
			return nil, nil, info, err
		}
		blocks, err := ocrengines.Read(engine, f, size.Width, size.Height)
		f.Close()
		if err != nil {
			return nil, nil, info, fmt.Errorf("page %d: %w", i+1, err)
		}
		pages = append(pages, multipage.Page{Blocks: blocks, Width: float64(size.Width), Height: float64(size.Height)})
	}
	return pages, canonical, info, nil
}

func TestMultipage(t *testing.T) {
	entries, err := os.ReadDir(multipageDir)
	if err != nil {
		t.Skip("no multi-page fixtures; run scripts/generate-multipage-fixtures.sh")
	}
	baselines := map[string]float64{}
	if data, err := os.ReadFile(multipageBaselines); err == nil {
		if err := json.Unmarshal(data, &baselines); err != nil {
			t.Fatalf("parsing baselines: %v", err)
		}
	}
	update := os.Getenv("UPDATE_BASELINES") != ""
	results := map[string]float64{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(multipageDir, entry.Name())
		for _, source := range []string{"apple", "tesseract", "pdftext"} {
			if _, err := os.Stat(filepath.Join(dir, source+"-1.json")); err != nil {
				continue
			}
			key := entry.Name() + "/" + source
			t.Run(key, func(t *testing.T) {
				pages, canonical, info, err := loadMultipage(dir, source)
				if err != nil {
					t.Fatal(err)
				}
				result, err := multipage.Sort(pages, canonical, sorters.ConfigForLanguage(info.Language), logger.Noop{})
				if err != nil {
					t.Fatalf("sorting: %v", err)
				}

				canonicalUnits := splitUnits(strings.Join(canonical, "\n"))
				sortedUnits := splitUnits(result.Document.Text)
				matches := lcsLength(canonicalUnits, sortedUnits)
				accuracy := 100 * float64(matches) / float64(len(canonicalUnits))
				results[key] = accuracy

				// Per-paragraph recovery.
				byLine := make(map[int]string)
				for _, para := range result.Document.Paragraphs {
					if para.CanonicalLine != nil {
						byLine[*para.CanonicalLine] += " " + result.Document.Text[para.Span.Start:para.Span.End]
					}
				}
				recovered, paragraphCount := 0.0, 0
				for i, line := range canonical {
					units := splitUnits(line)
					if len(units) == 0 {
						continue
					}
					paragraphCount++
					recovered += float64(lcsLength(units, splitUnits(byLine[i]))) / float64(len(units))
				}
				paragraphScore := 100 * recovered / float64(paragraphCount)
				results[key+"#paragraphs"] = paragraphScore
				results[key+"#precision"] = 100 * float64(matches) / float64(max(1, len(sortedUnits)))

				// Context for reading the score: what the pages hold at all,
				// and what plain page-then-emit order would give.
				var raw []string
				for _, page := range pages {
					for _, b := range page.Blocks {
						raw = append(raw, b.Text)
					}
				}
				rawUnits := splitUnits(strings.Join(raw, "\n"))
				emit := 100 * float64(lcsLength(canonicalUnits, rawUnits)) / float64(len(canonicalUnits))

				// Structure: a canonical paragraph should come out as one
				// paragraph, page break or not.
				canonicalParagraphs, straddles, furniture := 0, 0, 0
				for _, line := range canonical {
					if strings.TrimSpace(line) != "" {
						canonicalParagraphs++
					}
				}
				for _, a := range result.Assignments {
					if a.Straddle {
						straddles++
					}
				}
				for _, page := range result.Document.Pages {
					furniture += len(page.Furniture)
				}
				t.Logf("accuracy %.2f%% (%d/%d units, %d extra); paragraph score %.2f%%; emit order %.2f%%; paragraphs %d (canonical %d); %d straddles; %d furniture runs; %d unhandled",
					accuracy, matches, len(canonicalUnits), len(sortedUnits)-matches, paragraphScore, emit,
					len(result.Document.Paragraphs), canonicalParagraphs, straddles, furniture, len(result.Unhandled))

				for _, k := range []string{key, key + "#paragraphs", key + "#precision"} {
					baseline, ok := baselines[k]
					switch {
					case !ok && !update:
						t.Errorf("no baseline for %s (%.2f%%); run with UPDATE_BASELINES=1 to record one", k, results[k])
					case ok && results[k] < baseline-baselineTolerance:
						t.Errorf("%s: %.2f%% dropped below baseline %.2f%%", k, results[k], baseline)
					}
				}
			})
		}
	}

	if update {
		changed := false
		for key, accuracy := range results {
			if rounded := roundDown2(accuracy); rounded > baselines[key] || baselines[key] == 0 {
				baselines[key] = rounded
				changed = true
			}
		}
		if changed {
			keys := make([]string, 0, len(baselines))
			for k := range baselines {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			data, _ := json.MarshalIndent(baselines, "", "  ")
			if err := os.WriteFile(multipageBaselines, append(data, '\n'), 0644); err != nil {
				t.Fatalf("saving baselines: %v", err)
			}
		}
	}
}
