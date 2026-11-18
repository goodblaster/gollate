package integration

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/goodblaster/gollate/internal/bookoverlay"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/multipage"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// Exact bit-parallel LCS, equivalent to lcsLength but O(n*m/64). Book-scale
// comparisons need this: a quadratic word-by-word table obscures sort timing.
func bookLCS(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	width := (len(b) + 63) / 64
	masks := make(map[string][]uint64)
	for j, word := range b {
		if masks[word] == nil {
			masks[word] = make([]uint64, width)
		}
		masks[word][j/64] |= uint64(1) << uint(j%64)
	}
	state := make([]uint64, width)
	for _, word := range a {
		match := masks[word]
		if match == nil {
			continue
		}
		carry, borrow := uint64(1), uint64(0)
		for j, old := range state {
			x := old | match[j]
			y := old<<1 | carry
			carry = old >> 63
			difference, nextBorrow := bits.Sub64(x, y, borrow)
			borrow = nextBorrow
			state[j] = x & ^difference
		}
	}
	count := 0
	for _, word := range state {
		count += bits.OnesCount64(word)
	}
	return count
}

func TestBookLCS(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	for _, n := range []int{0, 1, 63, 64, 65, 127, 128, 129, 257} {
		for _, m := range []int{0, 1, 63, 64, 65, 129, 257} {
			a, b := make([]string, n), make([]string, m)
			for i := range a {
				a[i] = fmt.Sprint(rng.Intn(11))
			}
			for i := range b {
				b[i] = fmt.Sprint(rng.Intn(11))
			}
			if got, want := bookLCS(a, b), lcsLength(a, b); got != want {
				t.Fatalf("LCS lengths %d,%d: got %d want %d", n, m, got, want)
			}
		}
	}
}

type bookScore struct {
	Pages          int     `json:"pages"`
	CanonicalUnits int     `json:"canonical_units"`
	InputUnits     int     `json:"input_units"`
	OutputUnits    int     `json:"output_units"`
	InputRecall    float64 `json:"input_bag_recall"`
	RawOrder       float64 `json:"raw_order_accuracy"`
	Accuracy       float64 `json:"sorted_order_accuracy"`
	Precision      float64 `json:"sorted_precision"`
	Paragraphs     float64 `json:"paragraph_recovery"`
	Localized      float64 `json:"canonical_span_coverage"`
	Unhandled      int     `json:"unhandled_runs"`
	Furniture      int     `json:"furniture_runs"`
	SortSeconds    float64 `json:"sort_seconds"`
	ScoreSeconds   float64 `json:"score_seconds"`
}

type bookUnit struct{ start, end int }

func bookUnitRanges(s string) []bookUnit {
	var out []bookUnit
	start := -1
	for i, r := range s {
		word := unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r)
		if word && start < 0 {
			start = i
		}
		if !word && start >= 0 {
			if len(splitUnits(s[start:i])) > 0 {
				out = append(out, bookUnit{start, i})
			}
			start = -1
		}
	}
	if start >= 0 && len(splitUnits(s[start:])) > 0 {
		out = append(out, bookUnit{start, len(s)})
	}
	return out
}

// Opt-in real-book suite: rendering and OCR are done once by the script;
// repeated evaluations read only the saved blocks and EPUB-derived text.
// BOOK_LABEL keeps A/B artifacts apart without touching the OCR inputs.
func TestBookCorpus(t *testing.T) {
	root := os.Getenv("BOOK_CORPUS")
	if root == "" {
		t.Skip("set BOOK_CORPUS to experiments/meditations (see scripts/meditations-corpus.py)")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	label := os.Getenv("BOOK_LABEL")
	if label == "" {
		label = "current"
	}
	if filepath.Base(label) != label || label == "." || label == ".." {
		t.Fatal("BOOK_LABEL must be one path component")
	}
	scores := make(map[string]bookScore)
	baselineFile := filepath.Join("..", "testdata", "books", "meditations", "baselines.json")
	baselines := make(map[string]bookScore)
	if data, err := os.ReadFile(baselineFile); err == nil {
		if err := json.Unmarshal(data, &baselines); err != nil {
			t.Fatal(err)
		}
	}
	for _, layout := range []string{"single-column", "double-column"} {
		for _, source := range []string{"apple", "tesseract", "pdftext"} {
			key := layout + "/" + source
			t.Run(key, func(t *testing.T) {
				dir := filepath.Join(root, layout)
				pages, canonical, info, err := loadMultipage(dir, source)
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				result, err := multipage.Sort(pages, canonical, sorters.ConfigForLanguage(info.Language), logger.Noop{})
				if err != nil {
					t.Fatal(err)
				}
				sortSeconds := time.Since(start).Seconds()
				start = time.Now()
				truth := splitUnits(strings.Join(canonical, "\n"))
				actual := splitUnits(result.Document.Text)
				var rawText strings.Builder
				for _, page := range pages {
					for _, b := range page.Blocks {
						rawText.WriteString(b.Text)
						rawText.WriteByte(' ')
					}
				}
				raw := splitUnits(rawText.String())
				available := make(map[string]int)
				for _, w := range raw {
					available[w]++
				}
				present := 0
				for _, w := range truth {
					if available[w] > 0 {
						present++
						available[w]--
					}
				}
				matches := bookLCS(truth, actual)
				score := bookScore{Pages: len(pages), CanonicalUnits: len(truth), InputUnits: len(raw), OutputUnits: len(actual), InputRecall: 100 * float64(present) / float64(len(truth)), RawOrder: 100 * float64(bookLCS(truth, raw)) / float64(len(truth)), Accuracy: 100 * float64(matches) / float64(len(truth)), Precision: 100 * float64(matches) / float64(max(1, len(actual))), Unhandled: len(result.Unhandled), SortSeconds: sortSeconds}
				byLine := make(map[int]string)
				covered := make([][]bool, len(canonical))
				ranges := make([][]bookUnit, len(canonical))
				for i, line := range canonical {
					ranges[i] = bookUnitRanges(line)
					covered[i] = make([]bool, len(ranges[i]))
				}
				for _, para := range result.Document.Paragraphs {
					if para.CanonicalLine == nil {
						continue
					}
					i := *para.CanonicalLine
					if i < 0 || i >= len(canonical) {
						t.Fatalf("invalid canonical line %d", i)
					}
					byLine[i] += " " + result.Document.Text[para.Span.Start:para.Span.End]
					for _, tok := range para.Tokens {
						if tok.Canonical == nil {
							continue
						}
						span := tok.Canonical
						if span.Start < 0 || span.End > len(canonical[i]) || span.End < span.Start {
							t.Fatalf("invalid canonical span %+v on line %d", span, i)
						}
						want := splitUnits(canonical[i][span.Start:span.End])
						got := splitUnits(result.Document.Text[tok.Span.Start:tok.Span.End])
						if strings.Join(want, " ") != strings.Join(got, " ") {
							continue
						}
						for j, r := range ranges[i] {
							if r.start >= span.Start && r.end <= span.End {
								covered[i][j] = true
							}
						}
					}
				}
				type failure struct {
					Line      int     `json:"line"`
					Recovery  float64 `json:"recovery"`
					Canonical string  `json:"canonical"`
					Output    string  `json:"output"`
				}
				var failures []failure
				recovered := 0.0
				paragraphs, linked := 0, 0
				for i, line := range canonical {
					units := splitUnits(line)
					if len(units) == 0 {
						continue
					}
					paragraphs++
					share := float64(bookLCS(units, splitUnits(byLine[i]))) / float64(len(units))
					recovered += share
					if share < .999999 {
						failures = append(failures, failure{i, 100 * share, line, strings.TrimSpace(byLine[i])})
					}
					for _, yes := range covered[i] {
						if yes {
							linked++
						}
					}
				}
				score.Paragraphs = 100 * recovered / float64(paragraphs)
				score.Localized = 100 * float64(linked) / float64(len(truth))
				for _, page := range result.Document.Pages {
					score.Furniture += len(page.Furniture)
				}
				score.ScoreSeconds = time.Since(start).Seconds()
				scores[key] = score
				sort.SliceStable(failures, func(i, j int) bool { return failures[i].Recovery < failures[j].Recovery })
				artifacts := filepath.Join(dir, label)
				if err := os.MkdirAll(artifacts, 0755); err != nil {
					t.Fatal(err)
				}
				write := func(name string, value any) {
					data, err := json.MarshalIndent(value, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(artifacts, name), append(data, '\n'), 0644); err != nil {
						t.Fatal(err)
					}
				}
				write(source+"-sorted.json", result)
				write(source+"-failures.json", failures)
				if err := os.WriteFile(filepath.Join(artifacts, source+"-sorted.txt"), []byte(result.Document.Text), 0644); err != nil {
					t.Fatal(err)
				}
				// Keep image rendering outside measured sort/scoring time. Normal
				// evaluation stays quick; BOOK_OVERLAYS=1 adds both diagnostic views.
				if os.Getenv("BOOK_OVERLAYS") == "1" {
					if err := bookoverlay.Write(result.Document, canonical, bookoverlay.Options{
						ImageDir: dir, OutputDir: filepath.Join(artifacts, source+"-overlays"),
						Title: key, Seed: 117, Samples: 2,
					}); err != nil {
						t.Fatal(err)
					}
				}
				t.Logf("pages=%d units=%d input-recall=%.3f%% raw-order=%.3f%% sorted=%.3f%% precision=%.3f%% paragraphs=%.3f%% span-coverage=%.3f%% sort=%.3fs scoring=%.3fs furniture=%d unhandled=%d", score.Pages, score.CanonicalUnits, score.InputRecall, score.RawOrder, score.Accuracy, score.Precision, score.Paragraphs, score.Localized, score.SortSeconds, score.ScoreSeconds, score.Furniture, score.Unhandled)
				if baseline, ok := baselines[key]; ok && label == "current" {
					for metric, pair := range map[string][2]float64{"accuracy": {score.Accuracy, baseline.Accuracy}, "paragraphs": {score.Paragraphs, baseline.Paragraphs}, "precision": {score.Precision, baseline.Precision}, "span coverage": {score.Localized, baseline.Localized}} {
						if pair[0] < pair[1]-.05 {
							t.Errorf("%s regressed: %.3f < %.3f", metric, pair[0], pair[1])
						}
					}
				}
			})
		}
	}
	data, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "report-"+label+".json"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_BASELINES") != "" && !t.Failed() {
		for key, score := range scores {
			old := baselines[key]
			score.Accuracy = max(score.Accuracy, old.Accuracy)
			score.Precision = max(score.Precision, old.Precision)
			score.Paragraphs = max(score.Paragraphs, old.Paragraphs)
			score.Localized = max(score.Localized, old.Localized)
			baselines[key] = score
		}
		data, err := json.MarshalIndent(baselines, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(baselineFile), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(baselineFile, append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
