package sorters

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/goodblaster/gollate/pkg/language"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
)

type Block = ocr.Block

// Sorter performs pathfinding-based sorting of OCR blocks to match canonical text.
type Sorter struct {
	lines   *LineServer        // canonical lines of text
	input   []Block            // input OCR blocks (normalized)
	output  []Block            // output OCR blocks (normalized)
	mapped  map[string][]Block // map of words to all blocks that contain that word
	config  SorterConfig       // algorithm configuration parameters
	metrics SortMetrics        // performance metrics
	handler language.Handler   // language-specific rules and formatting

	candidatePaths []Path        // viable paths found through the chain during pathfinding
	holeSlots      []bool        // per-slot wildcard-hole markers for the current chain
	shortest       float64       // shortest path found so far
	elapsed        time.Duration // time it took to sort
	perm           int           // current permutation count
	logger         logger.Logger // logger instance
}

// NewOcrSorter creates a new sorter with default configuration.
func NewOcrSorter(ocrBlocks []Block, text []string, log logger.Logger) *Sorter {
	return NewOcrSorterWithConfig(ocrBlocks, text, log, DefaultConfig())
}

// NewOcrSorterWithConfig creates a new sorter with custom configuration.
func NewOcrSorterWithConfig(ocrBlocks []Block, text []string, log logger.Logger, config SorterConfig) *Sorter {
	// Use default logger if none provided
	if log == nil {
		log = logger.Default()
	}

	// Normalize text and parse into lines first (needed for smart hyphen splitting)
	lines := ParseLines(text)

	// Normalize all the text we received from OCR.
	ocrBlocks = normalizeOcrBlocks(ocrBlocks, config.SplitHyphenatedWords, lines)

	// Tokenization and spacing rules: from the language hint when one was
	// given, otherwise detected from the canonical text's script.
	handler := language.ForLanguage(config.Language, text...)

	// Preparation: infer text orientation from block geometry (see
	// vertical.go). Only ever switches horizontal -> vertical; needs line
	// data for the flow signal.
	if !config.DisableVerticalDetection && config.ReadingOrder.IsHorizontal() && detectVerticalText(ocrBlocks) {
		log.Debug("vertical text detected; switching reading order to VerticalTTB_RTL")
		config.ReadingOrder = VerticalTTB_RTL
		if config.EnableVerticalWrapBridging && !config.EnableWrapBridging {
			log.Debug("vertical text: enabling wrap bridging (columns wrap like lines)")
			config.EnableWrapBridging = true
		}
	}

	// Preparation: repair unrecognizable tokens using the engine's own line
	// grouping (see linerepair.go). Only the matching key changes; block
	// text stays what OCR read. No-op when blocks carry no line data.
	repairs := 0
	if !config.DisableLineRepair {
		repairs = repairLines(ocrBlocks, lines, handler)
	}

	// Start building the sorter.
	sorter := &Sorter{
		input:   ocrBlocks,
		lines:   NewLineServer(lines),
		mapped:  mapBlocks(ocrBlocks),
		config:  config,
		handler: handler,
		logger:  log,
	}
	sorter.metrics.LineRepairs = repairs

	return sorter
}

// normalizeOcrBlocks computes each block's matching key (NormedText),
// drops blocks with no matchable text, and renumbers Index in emit order
// (OriginalIndex keeps the engine's index).
//
// With splitCompound, a block whose text normalizes to several canonical
// tokens is split into one block per token. OCR engines emit "four-block",
// "$4.2", "6:30" and "12,000" as single words, but canonical text
// tokenizes them as two ("four block", "$4 2", "6 30", "12 000"), so
// unsplit they can never match. The separator each part was followed by is
// kept in HyphenAfter so output can restore the original text. A block is
// only split when every part is a word of the canonical text; anything
// else (and anything NormalizeText keeps whole: initialisms, phone
// numbers, emails, URLs) stays one block.
func normalizeOcrBlocks(ocrBlocks []Block, splitCompound bool, canonicalLines []Line) []Block {
	var processed []Block
	nextIndex := 0
	keep := func(w Block) {
		w.OriginalIndex = w.Index
		w.Index = nextIndex
		nextIndex++
		processed = append(processed, w)
	}

	// Canonical vocabulary: parts must exist here to be worth splitting.
	canonicalWords := make(map[string]bool)
	if splitCompound {
		for _, line := range canonicalLines {
			for _, word := range strings.Fields(line.Normalized) {
				canonicalWords[word] = true
			}
		}
	}

	for _, w := range attachPunctuation(ocrBlocks) {
		if strings.TrimSpace(w.NormedText) == "" {
			continue
		}

		var parts []compoundPart
		if splitCompound && strings.Contains(w.NormedText, " ") {
			parts = splitCompoundText(w.Text, strings.Fields(w.NormedText), canonicalWords)
		}
		if len(parts) < 2 {
			keep(w)
			continue
		}

		// Divide the box along its width in proportion to each part's
		// share of the text (separators included).
		total := 0
		for _, part := range parts {
			total += part.runes
		}
		left := w.BoundingBox.Left
		for _, part := range parts {
			sub := w
			sub.Text = part.text
			sub.NormedText = part.normalized
			sub.HyphenAfter = part.separator
			sub.BoundingBox.Left = left
			sub.BoundingBox.Width = w.BoundingBox.Width * float64(part.runes) / float64(total)
			left += sub.BoundingBox.Width
			keep(sub)
		}
	}

	return processed
}

// attachPunctuation computes NormedText for every block and folds blocks
// that are pure punctuation into a neighboring word.
//
// Engines often emit punctuation as its own token: PDF text layers split
// the closing quote off `downtown."`, and per-character CJK output makes
// every 。 and 、 a block. Such blocks have no matching key, so they used to
// be discarded - and with them the punctuation in the sorted text, plus
// the sentence boundaries leftover assembly looks for. Attaching them to
// the adjacent word keeps the text intact without giving the matcher
// anything new to match.
//
// Opening punctuation joins the following word, closing punctuation the
// preceding one, and anything else whichever is nearer. A mark with no
// word on its own line is dropped, as before.
func attachPunctuation(blocks []Block) []Block {
	out := make([]Block, len(blocks))
	copy(out, blocks)
	for i := range out {
		out[i].NormedText = NormalizeText(out[i].Text)
	}
	isWord := func(i int) bool { return strings.TrimSpace(out[i].NormedText) != "" }

	// gap is the empty space between two boxes, or -1 if they are not
	// neighbors on one line (different engine lines, or too far apart).
	gap := func(a, b Block) float64 {
		if a.LineId != "" && b.LineId != "" && a.LineId != b.LineId {
			return -1
		}
		dx := max(a.Left(), b.Left()) - min(a.Right(), b.Right())
		dy := max(a.Top(), b.Top()) - min(a.Bottom(), b.Bottom())
		d := max(dx, 0) + max(dy, 0)
		if d > 1.5*max(a.Height(), b.Height(), min(a.Width(), b.Width())) {
			return -1
		}
		return d
	}

	// Each mark is merged into a word; pending prefixes wait for theirs.
	prefix := make(map[int]string)
	lastWord := -1
	for i := range out {
		if isWord(i) {
			lastWord = i
			continue
		}
		text := strings.TrimSpace(out[i].Text)
		if text == "" {
			continue
		}
		next := -1
		for j := i + 1; j < len(out); j++ {
			if isWord(j) {
				next = j
				break
			}
		}
		prevGap, nextGap := -1.0, -1.0
		if lastWord >= 0 {
			prevGap = gap(out[lastWord], out[i])
		}
		if next >= 0 {
			nextGap = gap(out[i], out[next])
		}

		first, _ := utf8.DecodeRuneInString(text)
		opening := unicode.In(first, unicode.Ps, unicode.Pi) || first == '¿' || first == '¡'
		closing := unicode.In(first, unicode.Pe, unicode.Pf)

		toPrev := prevGap >= 0 && (nextGap < 0 || prevGap <= nextGap)
		if opening && nextGap >= 0 {
			toPrev = false
		} else if closing && prevGap >= 0 {
			toPrev = true
		}

		// Only the text moves. The word keeps its own box: geometry is what
		// pathfinding and reading-order assembly measure, and a box grown
		// to cover a neighboring mark shifts column and line edges (measured:
		// -35 on vertical Japanese, where same-column sentences are told
		// apart by an exactly shared edge).
		switch {
		case toPrev:
			out[lastWord].Text += text
		case nextGap >= 0:
			prefix[next] += text
		}
	}
	for i, p := range prefix {
		out[i].Text = p + out[i].Text
	}
	return out
}

// compoundPart is one token-sized piece of a compound OCR word.
type compoundPart struct {
	text       string // original text of this piece (display)
	normalized string // its matching key
	separator  string // what followed it in the original ("-", ".", ":"); "" for the last piece
	runes      int    // rune length of text+separator, for box apportioning
}

// splitCompoundText cuts text at its interior separators into pieces that
// normalize to exactly the given tokens, or returns nil if it cannot (the
// block then stays whole). Leading and trailing punctuation stays attached
// to the first and last piece.
func splitCompoundText(text string, tokens []string, canonicalWords map[string]bool) []compoundPart {
	for _, token := range tokens {
		if !canonicalWords[token] {
			return nil
		}
	}

	isSeparator := func(r rune) bool {
		if unicode.IsSpace(r) {
			return true
		}
		_, ok := replacements[r]
		return ok
	}

	runes := []rune(text)
	// Trim the search to the span between the first and last word runes.
	first, last := 0, len(runes)-1
	for first < len(runes) && isSeparator(runes[first]) {
		first++
	}
	for last >= 0 && isSeparator(runes[last]) {
		last--
	}
	if first >= last {
		return nil
	}

	var parts []compoundPart
	start := 0 // start of the current piece (includes leading punctuation)
	for i := first; i <= last; {
		if !isSeparator(runes[i]) {
			i++
			continue
		}
		j := i
		for j <= last && isSeparator(runes[j]) {
			j++
		}
		separator := string(runes[i:j])
		if strings.ContainsFunc(separator, unicode.IsSpace) {
			separator = "" // whitespace inside a block: output re-spaces it
		}
		parts = append(parts, compoundPart{
			text:      string(runes[start:i]),
			separator: separator,
			runes:     j - start,
		})
		start, i = j, j
	}
	parts = append(parts, compoundPart{text: string(runes[start:]), runes: len(runes) - start})

	if len(parts) != len(tokens) {
		return nil
	}
	for i := range parts {
		parts[i].normalized = NormalizeText(parts[i].text)
		if parts[i].normalized != tokens[i] {
			return nil
		}
	}
	return parts
}

func (s *Sorter) InputBlock(i int) Block {
	return s.input[i]
}

func (s *Sorter) InputBlocks() []Block {
	return s.input
}

func (s *Sorter) MappedBlocks() map[string][]Block {
	return s.mapped
}

// Print writes the path's normalized words to stdout (debugging aid).
func (s *Sorter) Print(path Path) {
	fmt.Println(path.String(s.input))
}

func (s *Sorter) LineCount() int {
	return len(s.lines.List())
}

func (s *Sorter) Line(i int) Line {
	return s.lines.List()[i]
}

// Remove the words from the chain that have been used in the path.
// This prevents them from being reused which could lead to strange outcomes.
func (s *Sorter) unmapWords(path Path) {
	for _, index := range path.Nodes {
		if index < 0 {
			continue // unfilled hole - no block to unmap
		}
		s.unmapWordIndex(index)
	}
}

// unmapWordIndex - remove word index so it doesn't get reused.
func (s *Sorter) unmapWordIndex(index int) {
	// s.mapped is keyed by NormedText and built from s.input (after line
	// repair rekeying), so the block's own key locates its list directly.
	normedText := s.input[index].NormedText
	wordList := s.mapped[normedText]
	for i, w := range wordList {
		if w.Index == index {
			// Delete the word from the list.
			s.mapped[normedText] = append(wordList[:i], wordList[i+1:]...)

			// If the list is now empty, remove it from the map.
			if len(s.mapped[normedText]) == 0 {
				delete(s.mapped, normedText)
			}
			return
		}
	}
}

func (s *Sorter) Lines() []Line {
	return s.lines.List()
}

// UnhandledLines returns unmatched lines with a debug-oriented index prefix.
// For clean output use UnhandledText.
func (s *Sorter) UnhandledLines() []string {
	var unhandled []string
	for i, line := range s.lines.lines {
		// Exclude split lines - they were successfully split into smaller pieces
		// which may or may not have been found. We only care about lines that
		// were neither found nor split.
		if !line.Found && !line.Split {
			unhandled = append(unhandled, fmt.Sprintf("%d - %s\n", i, s.Line(i).Normalized))
		}
	}
	return unhandled
}

// UnhandledText returns the readable canonical text of lines that were
// neither matched nor split, whitespace trimmed, blanks omitted. This is
// the clean form for API/output consumers (no index prefix, no newline).
// Always non-nil so it serializes as [] rather than null.
func (s *Sorter) UnhandledText() []string {
	unhandled := []string{}
	for i, line := range s.lines.lines {
		if line.Found || line.Split || line.IsBlank {
			continue
		}
		if text := strings.TrimSpace(s.Line(i).OriginalText); text != "" {
			unhandled = append(unhandled, text)
		}
	}
	return unhandled
}

func (s *Sorter) Elapsed() time.Duration {
	return s.elapsed
}
