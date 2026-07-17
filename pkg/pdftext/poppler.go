package pdftext

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/goodblaster/errors"
)

// popplerBackend extracts via poppler's pdftotext (-tsv). Cross-platform
// and the only backend that reads Devanagari correctly (PDFKit drops
// matras). Weaknesses: poppler's boxed output emits RTL scripts in
// visual order — repaired below well enough for Devanagari-adjacent
// cases, but Arabic still scores below PDFKit — and vertical CJK order
// is poor (~20% vs PDFKit's 63-76% on the fixtures).
type popplerBackend struct{}

func init() { backends = append(backends, popplerBackend{}) }

func (popplerBackend) Name() string { return "poppler" }

func (popplerBackend) Available() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}

func (popplerBackend) Extract(path string) ([]Page, error) {
	out, err := exec.Command("pdftotext", "-tsv", path, "-").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, errors.Newf("pdftotext failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, errors.Wrap(err, "pdftotext failed")
	}

	pages, err := parseTSV(out)
	if err != nil {
		return nil, err
	}
	for p := range pages {
		pages[p].Words = repairRTL(pages[p].Words)
	}
	foldText(pages)
	return pages, nil
}

// parseTSV reads pdftotext -tsv output: one row per element, with
// ###PAGE###/###FLOW###/###LINE### marker rows carrying geometry for
// their level and word rows carrying text. Coordinates are points with
// a top-left origin; page rows define the media box size.
func parseTSV(tsv []byte) ([]Page, error) {
	const (
		colPage = 1
		colPara = 2
		colBlok = 3
		colLine = 4
		colLeft = 6
		colTop  = 7
		colWide = 8
		colHigh = 9
		colText = 11
	)

	var pages []Page
	lines := bytes.Split(tsv, []byte("\n"))
	if len(lines) > 0 {
		lines = lines[1:] // header row
	}
	for _, row := range lines {
		cols := strings.Split(string(row), "\t")
		if len(cols) < colText+1 {
			continue
		}
		text := cols[colText]

		geo := make([]float64, 4)
		for i, c := range []int{colLeft, colTop, colWide, colHigh} {
			v, err := strconv.ParseFloat(cols[c], 64)
			if err != nil {
				return nil, errors.Wrapf(err, "bad pdftotext -tsv geometry %q", string(row))
			}
			geo[i] = v
		}
		left, top, width, height := geo[0], geo[1], geo[2], geo[3]

		if text == "###PAGE###" {
			if width <= 0 || height <= 0 {
				return nil, errors.Newf("pdftotext -tsv page with invalid size %gx%g", width, height)
			}
			pages = append(pages, Page{Width: width, Height: height})
			continue
		}
		if strings.HasPrefix(text, "###") || strings.TrimSpace(text) == "" {
			continue
		}
		if len(pages) == 0 {
			return nil, errors.New("pdftotext -tsv word before any page row")
		}

		p := &pages[len(pages)-1]
		l, t := clamp01(left/p.Width), clamp01(top/p.Height)
		p.Words = append(p.Words, Word{
			Text:   text,
			Top:    t,
			Left:   l,
			Width:  min(width/p.Width, 1-l),
			Height: min(height/p.Height, 1-t),
			// All four hierarchy columns are needed: depending on the
			// document, poppler advances par_num or block_num per
			// visual line (hindi-three-column increments par_num with
			// block_num pinned at 0 — dropping par_num collapsed 484
			// words into one "line").
			LineId: fmt.Sprintf("%s-%s-%s-%s", cols[colPage], cols[colPara], cols[colBlok], cols[colLine]),
		})
	}
	return pages, nil
}

// repairRTL fixes poppler's visual-order emission of RTL scripts:
// characters within an RTL word arrive reversed, and words within an
// RTL line arrive leftmost-first. Both are flipped back so text and
// emit order are logical. This is a heuristic, not a full bidi pass —
// mixed-direction lines (RTL text with embedded numbers/Latin) keep
// their LTR runs untouched but may misorder around them.
func repairRTL(words []Word) []Word {
	for i, w := range words {
		if isRTLMajority(w.Text) {
			words[i].Text = reverseRunes(w.Text)
		}
	}

	out := make([]Word, 0, len(words))
	for start := 0; start < len(words); {
		end := start + 1
		for end < len(words) && words[end].LineId == words[start].LineId {
			end++
		}
		line := words[start:end]
		rtl := 0
		for _, w := range line {
			if isRTLMajority(w.Text) {
				rtl++
			}
		}
		if rtl > len(line)/2 {
			for i := len(line) - 1; i >= 0; i-- {
				out = append(out, line[i])
			}
		} else {
			out = append(out, line...)
		}
		start = end
	}
	return out
}

// isRTLMajority reports whether most of the string's runes are from
// right-to-left scripts (Hebrew, Arabic, and their presentation forms).
func isRTLMajority(s string) bool {
	total, rtl := 0, 0
	for _, r := range s {
		total++
		if (r >= 0x0590 && r <= 0x08FF) || (r >= 0xFB1D && r <= 0xFEFC) {
			rtl++
		}
	}
	return total > 0 && rtl > total/2
}

func reverseRunes(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
