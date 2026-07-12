// noise-inject derives a noisy OCR fixture from a clean Apple Vision OCR
// JSON file by injecting character-level misreads into a fraction of word
// blocks. Output is deterministic for a given (input, seed, rate).
//
// Only block text is mutated - bounding boxes are untouched - so ground
// truth geometry is preserved and any accuracy loss on the derived fixture
// is attributable purely to text mismatch.
//
// Usage:
//
//	noise-inject -in apple-ocr.json -out noisy-ocr.json -rate 0.05 -seed 20260706
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"unicode"
)

// Structs mirror the on-disk Apple Vision JSON exactly (see
// pkg/engines/apple) so a round-trip changes nothing but the text fields.
type word struct {
	Text   string  `json:"text"`
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type rect struct {
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type line struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Rect       rect    `json:"rect"`
	Words      []word  `json:"words"`
}

// confusions are common OCR misread patterns, applied in preference to
// random substitution when the word contains a matching sequence.
var confusions = []struct{ from, to string }{
	{"rn", "m"},
	{"m", "rn"},
	{"cl", "d"},
	{"d", "cl"},
	{"l", "1"},
	{"1", "l"},
	{"I", "l"},
	{"O", "0"},
	{"0", "O"},
	{"e", "c"},
	{"c", "e"},
	{"h", "b"},
	{"u", "v"},
	{"S", "5"},
	{"B", "8"},
}

// cjkPool is a small fixed set of common ideographs used as substitution
// targets for CJK characters, standing in for visual misreads.
var cjkPool = []rune("的一是不了人在有我他这中大来上国和生到子们地出道也时年得就那要下以")

func main() {
	in := flag.String("in", "", "input apple-ocr.json (clean)")
	out := flag.String("out", "", "output path for noisy JSON")
	rate := flag.Float64("rate", 0.05, "fraction of word blocks to corrupt (0-1)")
	seed := flag.Int64("seed", 1, "random seed (output is deterministic per input/seed/rate)")
	flag.Parse()

	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *rate < 0 || *rate > 1 {
		fmt.Fprintln(os.Stderr, "rate must be in [0, 1]")
		os.Exit(2)
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		fatal(err)
	}
	var lines []line
	if err := json.Unmarshal(data, &lines); err != nil {
		fatal(fmt.Errorf("parsing %s: %w", *in, err))
	}

	corrupted := injectNoise(lines, *rate, *seed)

	outData, err := json.MarshalIndent(lines, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*out, append(outData, '\n'), 0644); err != nil {
		fatal(err)
	}
	total := 0
	for _, l := range lines {
		total += len(l.Words)
	}
	fmt.Printf("%s: corrupted %d of %d word blocks (rate %.2f, seed %d) -> %s\n",
		*in, corrupted, total, *rate, *seed, *out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

type wordRef struct{ line, word int }

// injectNoise substitutes characters in a deterministic selection of word
// blocks, then rebuilds each affected line's text from its words. Returns
// the number of blocks corrupted.
func injectNoise(lines []line, rate float64, seed int64) int {
	rng := rand.New(rand.NewSource(seed))

	// Candidates: words containing at least one substitutable rune,
	// in document order.
	var candidates []wordRef
	for li := range lines {
		for wi := range lines[li].Words {
			if hasSubstitutableRune(lines[li].Words[wi].Text) {
				candidates = append(candidates, wordRef{li, wi})
			}
		}
	}

	// Pick an exact count by shuffling, then apply in document order so
	// each word's rng draws are position-stable.
	n := int(math.Round(rate * float64(len(candidates))))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	selected := candidates[:n]
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].line != selected[j].line {
			return selected[i].line < selected[j].line
		}
		return selected[i].word < selected[j].word
	})

	touchedLines := map[int]bool{}
	for _, ref := range selected {
		w := &lines[ref.line].Words[ref.word]
		w.Text = corrupt(w.Text, rng)
		touchedLines[ref.line] = true
	}

	// Keep line-level text consistent with the mutated words. The engine
	// reader only consumes word text, so this is hygiene, not correctness.
	for li := range touchedLines {
		rebuildLineText(&lines[li])
	}
	return len(selected)
}

func hasSubstitutableRune(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// corrupt applies one substitution to the word: a confusion-table rewrite
// when one applies, otherwise a random same-class character substitution.
func corrupt(text string, rng *rand.Rand) string {
	// Gather all applicable confusion rewrites (position, rule).
	type option struct {
		pos  int
		rule int
	}
	var options []option
	for ri, rule := range confusions {
		for pos := 0; ; {
			idx := strings.Index(text[pos:], rule.from)
			if idx < 0 {
				break
			}
			options = append(options, option{pos + idx, ri})
			pos += idx + len(rule.from)
		}
	}
	if len(options) > 0 {
		o := options[rng.Intn(len(options))]
		rule := confusions[o.rule]
		return text[:o.pos] + rule.to + text[o.pos+len(rule.from):]
	}

	// Fallback: substitute one letter/digit/CJK rune with a same-class rune.
	runes := []rune(text)
	var positions []int
	for i, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			positions = append(positions, i)
		}
	}
	pos := positions[rng.Intn(len(positions))]
	runes[pos] = substituteRune(runes[pos], rng)
	return string(runes)
}

func substituteRune(r rune, rng *rand.Rand) rune {
	switch {
	case isCJK(r):
		for {
			if sub := cjkPool[rng.Intn(len(cjkPool))]; sub != r {
				return sub
			}
		}
	case unicode.IsDigit(r):
		for {
			if sub := rune('0' + rng.Intn(10)); sub != r {
				return sub
			}
		}
	default:
		for {
			sub := rune('a' + rng.Intn(26))
			if unicode.IsUpper(r) {
				sub = unicode.ToUpper(sub)
			}
			if sub != r {
				return sub
			}
		}
	}
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3040 && r <= 0x309F) ||
		(r >= 0x30A0 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7AF)
}

// rebuildLineText reassembles line.Text from word texts, preserving the
// original separator convention (space-joined for Latin scripts, directly
// concatenated for CJK). Substitutions never touch spaces, so the original
// text still reveals which convention the engine used.
func rebuildLineText(l *line) {
	texts := make([]string, len(l.Words))
	for i, w := range l.Words {
		texts[i] = w.Text
	}
	sep := ""
	if strings.Contains(l.Text, " ") {
		sep = " "
	}
	l.Text = strings.Join(texts, sep)
}
