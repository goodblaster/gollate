package sorters

import (
	"cmp"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"slices"
)

// postSortBlocks orchestrates the post-processing of sorted blocks.
//
// This is the main entry point for all post-sort operations including:
// - Assembling found paths into lines
// - Processing leftover blocks
// - Grouping by canonical lines
// - Handling edge cases (no canonical text, etc.)
// - Serializing final output
func (s *Sorter) postSortBlocks() []Block {
	lines := s.lines.List()

	// Sort lines back to canonical order (they were sorted by length during processing)
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].OriginalLine < lines[j].OriginalLine
	})

	// Assemble found paths into lines (in canonical order, including blank
	// lines), pairing each sentence with its canonical line identity.
	// Identity must be captured here: splitLeftoversByCanonicalLines below
	// marks additional lines Found (assembled from leftovers), so a
	// positional walk over `lines` after that point would desync.
	assembledLines := s.assembleFoundPaths(lines)

	// Collect and process leftover blocks
	leftoverBlocks := s.collectLeftoverBlocks()
	s.metrics.LeftoverBlocks = len(leftoverBlocks)

	// Assemble leftover blocks into sentences
	leftoverSentences := s.assembleLeftoverSentences(leftoverBlocks)

	// Split leftover sentences that contain multiple canonical lines
	leftoverSentences = s.splitLeftoversByCanonicalLines(leftoverSentences)

	// Sort leftover sentences by position
	leftoverSentences = s.sortAndCleanLines(leftoverSentences)

	// Group all sentences by canonical paragraph and sort within each group
	// This ensures sentences from the same canonical line stay together in spatial order
	allSentences := s.groupAndSortByCanonicalLine(assembledLines, leftoverSentences, lines)

	// Special case: handle documents with no canonical text. (This used to
	// fire for any single unsplit canonical line, re-splitting its output
	// by punctuation and discarding which line it was - a one-line page,
	// such as the last few words of a book, lost its identity.)
	hasCanonicalText := false
	for _, line := range lines {
		if !line.IsBlank {
			hasCanonicalText = true
			break
		}
	}
	if !hasCanonicalText {
		allSentences = s.handleNoCanonicalText(allSentences)
		// Re-split by punctuation: the lines no longer correspond to
		// canonical lines one for one.
		s.outputCanonicalLines = make([]int, len(allSentences))
		for i := range s.outputCanonicalLines {
			s.outputCanonicalLines[i] = -1
		}
	}

	// Serialize to final output format with line separators
	return s.serializeOutput(allSentences)
}

// groupAndSortByCanonicalLine groups all sentences by their canonical line
// and orders the sentences within each group by where their text falls in
// that line, falling back to page position (top to bottom) when unknown.
// This ensures that sentences from the same canonical paragraph stay together
// in their natural reading order, even when some are found via pathfinding
// and others are assembled from leftovers.
func (s *Sorter) groupAndSortByCanonicalLine(foundSentences []assembledSentence, leftoverSentences [][]Block, lines []Line) [][]Block {
	// Map to track which OriginalLine each sentence belongs to
	type sentenceInfo struct {
		blocks       []Block
		originalLine int
		topPosition  float64 // primary reading-order key (see sentencePos)
		tiebreak     float64 // within-column order for vertical text
		isBlank      bool

		// offset is where this sentence's text starts within its canonical
		// line (in space-stripped normalized characters), or -1 if unknown.
		// It orders the pieces of one canonical paragraph. Page position
		// cannot: a paragraph that wraps into the next column continues
		// ABOVE where it left off.
		offset int

		// seq is the sentence's position among the leftover sentences in
		// emit order, or -1 for sentences found by pathfinding. It orders
		// leftovers that share an offset.
		seq int
	}

	var allSentences []sentenceInfo

	// Create a deduplicated map of canonical lines by OriginalLine
	// For split canonical lines, keep only the longest version for matching
	canonicalByOriginalLine := make(map[int]Line)
	for _, line := range lines {
		if line.IsBlank {
			continue
		}
		existing, ok := canonicalByOriginalLine[line.OriginalLine]
		if !ok || len(line.OriginalText) > len(existing.OriginalText) {
			canonicalByOriginalLine[line.OriginalLine] = line
		}
	}

	// Candidate canonical lines in ascending OriginalLine order, so overlap
	// ties resolve to the earliest line in the document (deterministic, and
	// consistent with human reading order when placement is ambiguous).
	// Built once: the space-stripped text is reused for every leftover
	// sentence. (Line.Normalized is NormalizeText(OriginalText) by
	// construction - see ParseLines and splitLine.)
	lineNumbers := make([]int, 0, len(canonicalByOriginalLine))
	for lineNum := range canonicalByOriginalLine {
		lineNumbers = append(lineNumbers, lineNum)
	}
	sort.Ints(lineNumbers)
	canonicalNoSpaces := make([]string, len(lineNumbers))
	noSpacesByLine := make(map[int]string, len(lineNumbers))
	for i, lineNum := range lineNumbers {
		canonicalNoSpaces[i] = strings.ReplaceAll(canonicalByOriginalLine[lineNum].Normalized, " ", "")
		noSpacesByLine[lineNum] = canonicalNoSpaces[i]
	}

	// Found sentences carry their canonical line identity from assembly time.
	for _, sentence := range foundSentences {
		if sentence.isBlank {
			allSentences = append(allSentences, sentenceInfo{
				blocks:       []Block{},
				originalLine: sentence.originalLine,
				topPosition:  blankSentencePos, // Blanks sort first within their group
				isBlank:      true,
				offset:       -1,
				seq:          -1,
			})
			continue
		}

		// Ordering key from the first block, along the reading order's
		// axis (top for horizontal text, column position for vertical).
		topPos, tie := 1.0, 0.0
		if len(sentence.blocks) > 0 {
			topPos = sentencePos(sentence.blocks[0], s.config.ReadingOrder)
			tie = sentencePosTiebreak(sentence.blocks[0], s.config.ReadingOrder)
		}

		allSentences = append(allSentences, sentenceInfo{
			blocks:       sentence.blocks,
			originalLine: sentence.originalLine,
			topPosition:  topPos,
			tiebreak:     tie,
			isBlank:      false,
			offset: strings.Index(noSpacesByLine[sentence.originalLine],
				strings.ReplaceAll(sentence.normalized, " ", "")),
			seq: -1,
		})
	}

	// Add leftover sentences - match them to canonical lines by text content
	firstLeftover := len(allSentences)
	var leftoverText []string // normalized text per leftover sentence
	var leftoverHomes [][]int // for ambiguous scraps: every line containing them
	for _, sentence := range leftoverSentences {
		if len(sentence) == 0 {
			continue
		}

		// Ordering key from the first block (reading-order axis).
		topPos := sentencePos(sentence[0], s.config.ReadingOrder)
		tie := sentencePosTiebreak(sentence[0], s.config.ReadingOrder)

		// Match this sentence to a canonical line by comparing text content
		originalLine := -1
		var sentenceText strings.Builder
		for i, block := range sentence {
			sentenceText.WriteString(block.Text)
			if i < len(sentence)-1 && !sentence[i+1].IsSeparator() {
				if s.handler.NeedsSpaceBetween(block.Text, sentence[i+1].Text) {
					sentenceText.WriteByte(' ')
				}
			}
		}
		normalizedSentence := NormalizeText(sentenceText.String())
		normalizedSentenceNoSpaces := strings.ReplaceAll(normalizedSentence, " ", "")

		// Find the canonical line with the best match (highest overlap)
		// Match against the deduplicated canonical lines (longest version of each OriginalLine)
		bestMatch := -1
		bestOverlap := 0.0
		bestOffset := -1
		var containing []int // canonical lines that contain this sentence whole
		for i, lineNum := range lineNumbers {
			normalizedNoSpaces := canonicalNoSpaces[i]

			// Calculate overlap: how much of the sentence is in this canonical line
			// and how much of the canonical line is in this sentence
			sentenceInLine := strings.Contains(normalizedNoSpaces, normalizedSentenceNoSpaces)
			lineInSentence := strings.Contains(normalizedSentenceNoSpaces, normalizedNoSpaces)

			if sentenceInLine {
				containing = append(containing, lineNum)
			}
			if sentenceInLine || lineInSentence {
				// Calculate overlap ratio
				var overlap float64
				if sentenceInLine {
					overlap = float64(len(normalizedSentenceNoSpaces)) / float64(len(normalizedNoSpaces))
				} else {
					overlap = float64(len(normalizedNoSpaces)) / float64(len(normalizedSentenceNoSpaces))
				}

				if overlap > bestOverlap {
					bestOverlap = overlap
					bestMatch = lineNum
					// Where the sentence sits inside the line (0 when the
					// sentence swallows the whole line).
					bestOffset = max(0, strings.Index(normalizedNoSpaces, normalizedSentenceNoSpaces))
				}
			}
		}

		if bestMatch != -1 {
			originalLine = bestMatch
		}
		// A scrap of a few characters that several canonical lines contain
		// identifies none of them. Leave it unplaced here; it is placed
		// beside its neighbors below, and must not anchor anything itself.
		var homes []int
		if len(containing) > 1 && len(s.handler.Tokenize(normalizedSentence)) < leftoverMinAnchorTokens {
			originalLine, bestOffset = -1, -1
			homes = containing
		}
		leftoverText = append(leftoverText, normalizedSentence)
		leftoverHomes = append(leftoverHomes, homes)

		allSentences = append(allSentences, sentenceInfo{
			blocks:       sentence,
			originalLine: originalLine,
			topPosition:  topPos,
			tiebreak:     tie,
			isBlank:      false,
			offset:       bestOffset,
			seq:          len(leftoverText) - 1,
		})
	}

	// Place leftover sentences that matched no canonical line exactly -
	// typically a visual line with one misread in it - beside their
	// emit-order neighbors instead of banishing them to the end.
	leftovers := allSentences[firstLeftover:]

	// Ambiguous scraps first: of the lines that contain the scrap, take the
	// one whose already-placed text lies nearest on the page.
	for i := range leftovers {
		if leftoverHomes[i] == nil || len(leftovers[i].blocks) == 0 {
			continue
		}
		x, y := leftovers[i].blocks[0].Center()
		home := make(map[int]bool, len(leftoverHomes[i]))
		for _, line := range leftoverHomes[i] {
			home[line] = true
		}
		best, bestDist := -1, math.MaxFloat64
		for _, other := range allSentences {
			if !home[other.originalLine] {
				continue
			}
			for _, b := range other.blocks {
				bx, by := b.Center()
				if d := math.Hypot(bx-x, by-y); d < bestDist {
					best, bestDist = other.originalLine, d
				}
			}
		}
		if best != -1 {
			leftovers[i].originalLine = best
			leftovers[i].offset = max(0, strings.Index(noSpacesByLine[best], strings.ReplaceAll(leftoverText[i], " ", "")))
		}
	}

	firstLine, lastLine := 0, 0
	if len(lineNumbers) > 0 {
		firstLine, lastLine = lineNumbers[0], lineNumbers[len(lineNumbers)-1]
	}
	for i := range leftovers {
		if leftovers[i].originalLine != -1 {
			continue
		}
		prev, next := -1, -1
		// Scraps never anchor anything: wherever they were put, it was a
		// guess among several lines.
		for j := i - 1; j >= 0; j-- {
			if leftovers[j].originalLine != -1 && leftoverHomes[j] == nil {
				prev = leftovers[j].originalLine
				break
			}
		}
		for j := i + 1; j < len(leftovers); j++ {
			if leftovers[j].originalLine != -1 && leftoverHomes[j] == nil {
				next = leftovers[j].originalLine
				break
			}
		}
		line, offset := s.placeByNeighbors(leftoverText[i], prev, next, firstLine, lastLine, canonicalByOriginalLine, noSpacesByLine)
		if line != -1 && offset == -1 {
			// Position within the line unknown: sit beside the nearest
			// emit-order neighbor placed in the same line (seq then keeps
			// emit order among equals). Every member of a matched group
			// must have an offset, or the group's ordering is not a
			// consistent order at all.
			offset = 0
			for j := i - 1; j >= 0; j-- {
				if leftovers[j].originalLine == line {
					offset = leftovers[j].offset
					break
				}
			}
		}
		leftovers[i].originalLine, leftovers[i].offset = line, offset
	}

	// Group by OriginalLine
	groups := make(map[int][]sentenceInfo)
	var originalLineOrder []int
	seenLines := make(map[int]bool)

	for _, sent := range allSentences {
		if !seenLines[sent.originalLine] {
			originalLineOrder = append(originalLineOrder, sent.originalLine)
			seenLines[sent.originalLine] = true
		}
		groups[sent.originalLine] = append(groups[sent.originalLine], sent)
	}

	// Add blank lines that fall between matched content lines
	// This preserves paragraph structure even when lines are matched from leftovers
	if len(originalLineOrder) > 0 {
		// Find min and max matched line numbers (excluding -1 for unmatched)
		minLine := -1
		maxLine := -1
		for _, lineNum := range originalLineOrder {
			if lineNum != -1 {
				if minLine == -1 || lineNum < minLine {
					minLine = lineNum
				}
				if lineNum > maxLine {
					maxLine = lineNum
				}
			}
		}

		// Add blank canonical lines that fall between matched lines
		if minLine != -1 && maxLine != -1 {
			for _, line := range lines {
				if line.IsBlank && line.OriginalLine > minLine && line.OriginalLine < maxLine {
					// Check if not already in groups
					if !seenLines[line.OriginalLine] {
						originalLineOrder = append(originalLineOrder, line.OriginalLine)
						seenLines[line.OriginalLine] = true
						// Add an empty sentence for this blank line
						groups[line.OriginalLine] = []sentenceInfo{{
							blocks:       []Block{},
							originalLine: line.OriginalLine,
							topPosition:  blankSentencePos,
							isBlank:      true,
							offset:       -1,
							seq:          -1,
						}}
					}
				}
			}
		}
	}

	// Sort sentences within each group by vertical position
	for originalLine := range groups {
		group := groups[originalLine]
		sort.Slice(group, func(i, j int) bool {
			// Blank lines first
			if group[i].isBlank != group[j].isBlank {
				return group[i].isBlank
			}
			// Then by where the text falls in the canonical line, when
			// both are known.
			if group[i].offset >= 0 && group[j].offset >= 0 && group[i].offset != group[j].offset {
				return group[i].offset < group[j].offset
			}
			// Leftovers sharing an offset keep emit order.
			if group[i].offset == group[j].offset && group[i].seq >= 0 && group[j].seq >= 0 {
				return group[i].seq < group[j].seq
			}
			// Then by position, with the within-column tiebreak for
			// vertical text (equal keys were previously left to the
			// unstable sort, scrambling same-column sentences).
			if group[i].topPosition != group[j].topPosition {
				return group[i].topPosition < group[j].topPosition
			}
			return group[i].tiebreak < group[j].tiebreak
		})
		groups[originalLine] = group
	}

	// Reassemble in canonical order (original line order, with -1 at end)
	var result [][]Block
	var resultLines []int // canonical line of each result entry, -1 if none
	emit := func(blocks []Block, canonicalLine int) {
		result = append(result, blocks)
		resultLines = append(resultLines, canonicalLine)
	}

	// Sort originalLineOrder by the actual line numbers to maintain canonical order
	sort.Slice(originalLineOrder, func(i, j int) bool {
		// -1 (unmatched leftovers) should sort last
		if originalLineOrder[i] == -1 {
			return false
		}
		if originalLineOrder[j] == -1 {
			return true
		}
		return originalLineOrder[i] < originalLineOrder[j]
	})

	// Process all groups in canonical order
	// Merge all sentences from the same canonical line into a single block sequence
	for _, originalLine := range originalLineOrder {
		// Special handling for unmatched leftovers (originalLine == -1)
		// Insert blank lines based on vertical spacing to preserve paragraph structure
		if originalLine == -1 && len(groups[originalLine]) > 1 {
			var havePrev bool
			var prevLast Block
			for _, sent := range groups[originalLine] {
				if len(sent.blocks) == 0 {
					continue
				}

				// If there's a significant gap from the last sentence along
				// the reading order's advance axis, insert a blank line.
				firstBlock := sent.blocks[0]
				if havePrev {
					ref := advanceRef(prevLast, firstBlock, s.config.ReadingOrder)
					// Only apply gap detection with meaningful size data.
					const minRef = 0.01 // 1% of the page minimum
					const paragraphBreakThreshold = 1.5
					if ref >= minRef && advanceGap(prevLast, firstBlock, s.config.ReadingOrder) > paragraphBreakThreshold*ref {
						emit([]Block{}, -1)
					}
				}

				// Append this sentence's blocks
				emit(sent.blocks, -1)

				prevLast = sent.blocks[len(sent.blocks)-1]
				havePrev = true
			}
		} else {
			// Normal handling for matched lines
			var mergedBlocks []Block
			isBlank := false
			for _, sent := range groups[originalLine] {
				mergedBlocks = append(mergedBlocks, sent.blocks...)
				if sent.isBlank {
					isBlank = true
				}
			}
			// Append if we have blocks OR if this is a blank line (paragraph separator)
			if len(mergedBlocks) > 0 || isBlank {
				emit(mergedBlocks, originalLine)
			}
		}
	}

	s.outputCanonicalLines = resultLines
	return result
}

// leftoverMinAnchorTokens is the length below which a leftover sentence
// that several canonical lines contain is considered ambiguous.
const leftoverMinAnchorTokens = 4

// leftoverMinOverlap is the share of an unmatched leftover sentence's
// tokens that must occur in a canonical line for the sentence to be placed
// there. Below it the sentence is treated as foreign to the canonical text
// (page furniture, captions) and stays with the unmatched leftovers.
const leftoverMinOverlap = 0.6

// placeByNeighbors finds a canonical line for a leftover sentence that no
// line contains exactly. Exact containment fails on a single misread
// character, and an unplaced sentence is emitted after everything else -
// so lightly noisy pages, where most visual lines still match and get
// placed, scored far worse than heavily noisy ones, where nothing matches
// and plain emit order survives (measured: 2% noise 50, 10% noise 78).
//
// Candidates are the canonical lines between those of the sentence's
// nearest matched neighbors in emit order (prev, next; -1 if none - with
// no neighbor on a side the range runs to that end of the document). The
// one sharing the most tokens with the sentence wins, if the share reaches
// leftoverMinOverlap. Returns the line and the sentence's estimated offset
// within it, or (-1, -1).
func (s *Sorter) placeByNeighbors(normalized string, prev, next, firstLine, lastLine int, canonical map[int]Line, noSpaces map[int]string) (line, offset int) {

	// With a neighbor on only one side, the range runs to that end of the
	// document: everything before the first placed sentence may belong to
	// any earlier line.
	lo, hi := prev, next
	switch {
	case prev == -1 && next == -1:
		lo, hi = firstLine, lastLine // no neighbor placed yet: any line
	case prev == -1:
		lo = firstLine
	case next == -1:
		hi = lastLine
	case lo > hi:
		lo, hi = hi, lo
	}

	tokens := s.handler.Tokenize(normalized)
	if len(tokens) == 0 {
		return -1, -1
	}

	share := func(candidate int) float64 {
		canonicalLine, ok := canonical[candidate]
		if !ok {
			return 0
		}
		available := make(map[string]int)
		for _, token := range s.handler.Tokenize(canonicalLine.Normalized) {
			available[token]++
		}
		shared := 0
		for _, token := range tokens {
			if available[token] > 0 {
				available[token]--
				shared++
			}
		}
		return float64(shared) / float64(len(tokens))
	}

	// The preceding neighbor's line is tried first and wins ties: a
	// sentence most often continues the paragraph before it.
	best, bestShare := -1, 0.0
	if prev != -1 {
		best, bestShare = prev, share(prev)
	}
	for candidate := lo; candidate <= hi; candidate++ {
		if sh := share(candidate); sh > bestShare {
			best, bestShare = candidate, sh
		}
	}
	if best == -1 || bestShare < leftoverMinOverlap {
		return -1, -1
	}
	return best, estimateOffset(noSpaces[best], strings.ReplaceAll(normalized, " ", ""))
}

// estimateOffset locates text inside line when it is not contained whole,
// by finding its opening (or failing that, closing) run of characters.
// Returns -1 if neither end can be found.
func estimateOffset(line, text string) int {
	runes := []rune(text)
	for size := min(len(runes), 12); size >= 2; size-- {
		if i := strings.Index(line, string(runes[:size])); i >= 0 {
			return i
		}
	}
	for size := min(len(runes), 12); size >= 2; size-- {
		tail := string(runes[len(runes)-size:])
		if i := strings.Index(line, tail); i >= 0 {
			return max(0, i-(len(text)-len(tail)))
		}
	}
	return -1
}

// assembledSentence is a found line's blocks paired with the canonical line
// they belong to, so downstream grouping never depends on positional sync
// with the lines array.
type assembledSentence struct {
	originalLine int
	isBlank      bool
	blocks       []Block
	normalized   string // the matched fragment's canonical text
}

// assembleFoundPaths converts found paths into block lines.
func (s *Sorter) assembleFoundPaths(lines []Line) []assembledSentence {
	var assembledLines []assembledSentence

	for _, line := range lines {
		if !line.Found {
			continue
		}

		// For blank lines (paragraph separators), add an empty line
		// This will result in consecutive empty blocks in serializeOutput,
		// preserving paragraph structure
		if line.IsBlank {
			assembledLines = append(assembledLines, assembledSentence{
				originalLine: line.OriginalLine,
				isBlank:      true,
			})
			continue
		}

		var blockLine []Block
		for _, index := range line.FoundPath.Nodes {
			if index < 0 {
				continue // unfilled hole (EnableChainHoles) - word absent from OCR
			}
			blockLine = append(blockLine, s.input[index])
		}

		assembledLines = append(assembledLines, assembledSentence{
			originalLine: line.OriginalLine,
			blocks:       blockLine,
			normalized:   line.Normalized,
		})
	}

	return assembledLines
}

// collectLeftoverBlocks gathers blocks not matched to any found path.
func (s *Sorter) collectLeftoverBlocks() []Block {
	var leftoverBlocks []Block

	for _, blocks := range s.mapped {
		leftoverBlocks = append(leftoverBlocks, blocks...)
	}

	// Sort by original index for spatial ordering
	sort.Slice(leftoverBlocks, func(i, j int) bool {
		return leftoverBlocks[i].Index < leftoverBlocks[j].Index
	})

	return leftoverBlocks
}

// assembleLeftoverSentences groups leftover blocks into sentences.
//
// This uses two strategies:
// 1. Spatial proximity - group consecutive blocks together
// 2. Punctuation - split on sentence-ending punctuation
func (s *Sorter) assembleLeftoverSentences(leftoverBlocks []Block) [][]Block {
	if len(leftoverBlocks) == 0 {
		return nil
	}

	// First, group spatially contiguous blocks
	// Grouping deliberately keeps the vertical-gap paragraph check even
	// under a vertical reading order: tesseract's vertical line ids span
	// columns, and splitting them at column wraps (the "correct" advance
	// axis) shreds sentences into fragments that fail canonical matching
	// (measured -21 on japanese-single-vertical/tesseract; see TESTING.md).
	contiguousLines := AssembleContiguousLines(leftoverBlocks)

	// Then split on punctuation to form sentences
	return splitLinesOnPunctuation(contiguousLines)
}

// splitLeftoversByCanonicalLines splits leftover sentences that contain multiple canonical lines.
// When the OCR merges multiple canonical lines together, this splits them back apart.
func (s *Sorter) splitLeftoversByCanonicalLines(sentences [][]Block) [][]Block {
	// Get unfound canonical lines (these might be merged in the OCR)
	unfoundLines := []Line{}
	for _, line := range s.lines.List() {
		if !line.Found && !line.IsBlank && line.OriginalText != "" {
			unfoundLines = append(unfoundLines, line)
		}
	}

	if len(unfoundLines) == 0 {
		return sentences // No unfound lines to match
	}

	// Space-stripped canonical text, computed once and reused for every
	// sentence (Line.Normalized is NormalizeText(OriginalText) by
	// construction).
	unfoundNoSpaces := make([]string, len(unfoundLines))
	for i, line := range unfoundLines {
		unfoundNoSpaces[i] = strings.ReplaceAll(line.Normalized, " ", "")
	}

	var result [][]Block
	for _, sentence := range sentences {
		// Extract full text from this sentence
		var fullText strings.Builder
		for i, block := range sentence {
			fullText.WriteString(block.Text)
			if i < len(sentence)-1 && !sentence[i+1].IsSeparator() {
				// Add space between blocks using language handler
				if s.handler.NeedsSpaceBetween(block.Text, sentence[i+1].Text) {
					fullText.WriteByte(' ')
				}
			}
		}
		sentenceText := fullText.String()

		// Find which unfound canonical lines are contained in this sentence
		var matchedLines []struct {
			line  Line
			start int
			end   int
		}

		normalizedSentence := NormalizeText(sentenceText)
		// Remove all spaces for matching (handles cases like "1857 年" vs "1857年")
		normalizedSentenceNoSpaces := strings.ReplaceAll(normalizedSentence, " ", "")

		for i, canLine := range unfoundLines {
			normalizedNoSpaces := unfoundNoSpaces[i]

			// Check if this canonical line is contained in the sentence
			if startIdx := strings.Index(normalizedSentenceNoSpaces, normalizedNoSpaces); startIdx != -1 {
				// Positions here are in the space-stripped string, so the
				// end must be too (it used the with-spaces length, pushing
				// every split boundary late by the line's space count).
				endIdx := startIdx + len(normalizedNoSpaces)
				matchedLines = append(matchedLines, struct {
					line  Line
					start int
					end   int
				}{canLine, startIdx, endIdx})
			}
		}

		// Mark matched canonical lines as found (assembled from leftovers)
		// This prevents them from appearing in UnhandledLines
		for _, matched := range matchedLines {
			// Find the line in s.lines.lines and mark it as Found
			for i := range s.lines.lines {
				if s.lines.lines[i].OriginalLine == matched.line.OriginalLine &&
					s.lines.lines[i].Normalized == matched.line.Normalized {
					s.lines.lines[i].Found = true
					break
				}
			}
		}

		// If multiple canonical lines found, check if we should split
		if len(matchedLines) > 1 {
			// Sort by start position
			slices.SortFunc(matchedLines, func(a, b struct {
				line  Line
				start int
				end   int
			}) int {
				return cmp.Compare(a.start, b.start)
			})

			// Only split if:
			// 1. The canonical lines are consecutive in the original text
			// 2. They cover a significant portion of the sentence (>50%)

			// Check if lines are consecutive
			areConsecutive := true
			for i := 0; i < len(matchedLines)-1; i++ {
				// Allow small gaps (e.g., blank lines) but not large jumps
				gap := matchedLines[i+1].line.OriginalLine - matchedLines[i].line.OriginalLine
				if gap > 2 {
					areConsecutive = false
					break
				}
			}

			// Calculate coverage (what % of sentence is matched canonical text)
			totalMatchedLength := 0
			for _, m := range matchedLines {
				totalMatchedLength += len(strings.ReplaceAll(m.line.Normalized, " ", ""))
			}
			sentenceLength := len(normalizedSentenceNoSpaces)
			coverage := float64(totalMatchedLength) / float64(sentenceLength)

			// Only split if lines are consecutive AND coverage is high
			if !areConsecutive || coverage < 0.5 {
				// Don't split - keep sentence as-is
				result = append(result, sentence)
				continue
			}

			// Split the blocks at canonical line boundaries
			currentPos := 0
			var currentLine []Block
			matchIndex := 0 // Track which canonical line we're in

			for _, block := range sentence {
				blockNorm := NormalizeText(block.Text)
				blockNormNoSpace := strings.ReplaceAll(blockNorm, " ", "")
				blockLen := len(blockNormNoSpace)

				// Check if adding this block would go past the current canonical line boundary
				if matchIndex < len(matchedLines)-1 && currentPos+blockLen > matchedLines[matchIndex].end {
					// Finish the current canonical line before adding this block
					if len(currentLine) > 0 {
						result = append(result, currentLine)
						currentLine = nil
					}
					matchIndex++
				}

				currentLine = append(currentLine, block)
				currentPos += blockLen
			}

			// Add remaining blocks
			if len(currentLine) > 0 {
				result = append(result, currentLine)
			}
		} else {
			// No split needed, keep sentence as-is
			result = append(result, sentence)
		}
	}

	return result
}

// splitLinesOnPunctuation splits block lines at sentence-ending punctuation
// and at LineId changes (different OCR visual lines).
func splitLinesOnPunctuation(lines [][]Block) [][]Block {
	var sentences [][]Block

	for _, line := range lines {
		var sentence []Block
		var lastLineId string

		for i, block := range line {
			// Split if LineId changes AND either:
			// 1. Previous block ends with sentence punctuation, OR
			// 2. The upcoming segment (new LineId) looks like a header, OR
			// 3. The previous segment was a short header
			if len(sentence) > 0 && block.LineId != "" && lastLineId != "" && block.LineId != lastLineId {
				lastBlock := sentence[len(sentence)-1]

				// Check if previous sentence ends with punctuation
				prevEndsPunctuation := endsWithSentencePunctuation(lastBlock.Text)

				// Check if previous segment (with lastLineId) was a short header
				prevSegmentBlockCount := 0
				for _, b := range sentence {
					if b.LineId == lastLineId {
						prevSegmentBlockCount++
					}
				}
				prevWasHeader := prevSegmentBlockCount <= 4 && prevSegmentBlockCount > 0

				// Check if upcoming segment (new LineId) is a short header
				// A header is a standalone short phrase, not a continuation of the previous sentence
				// Count blocks with the same LineId as current block
				upcomingBlockCount := 0
				firstRune, _ := utf8.DecodeRuneInString(block.Text)
				upcomingStartsUpper := unicode.IsUpper(firstRune) // rune, not byte: "École"
				for j := i; j < len(line) && line[j].LineId == block.LineId; j++ {
					upcomingBlockCount++
				}

				// Check if previous line ends with a continuation word (preposition, conjunction, etc.)
				// that indicates the sentence continues on the next line
				prevEndsWithContinuation := false
				if len(lastBlock.Text) > 0 {
					lastWord := strings.ToLower(strings.TrimSpace(lastBlock.Text))
					continuationWords := []string{"about", "at", "in", "on", "to", "for", "with", "by", "of", "and", "or", "but", "the", "a", "an"}
					for _, cw := range continuationWords {
						if lastWord == cw {
							prevEndsWithContinuation = true
							break
						}
					}
				}

				// Only treat as header if it's short AND starts with uppercase AND previous line doesn't end mid-sentence
				isUpcomingHeader := upcomingBlockCount <= 4 && upcomingStartsUpper && !prevEndsWithContinuation

				if prevEndsPunctuation || isUpcomingHeader || prevWasHeader {
					sentences = append(sentences, sentence)
					sentence = nil
					lastLineId = ""
				}
			}

			sentence = append(sentence, block)
			if lastLineId == "" || block.LineId != "" {
				lastLineId = block.LineId
			}
		}
		// Add remaining blocks as final sentence
		if len(sentence) > 0 {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// sortAndCleanLines removes empty lines and sorts by first block position.
// This is used only for leftover sentences, not for found lines (which stay in canonical order).
func (s *Sorter) sortAndCleanLines(lines [][]Block) [][]Block {
	// Remove empty lines (shouldn't happen for leftover sentences, but just in case)
	lines = slices.DeleteFunc(lines, func(line []Block) bool {
		return len(line) == 0
	})

	// Sort by the index of the first block in each line
	slices.SortFunc(lines, func(a, b []Block) int {
		return cmp.Compare(a[0].Index, b[0].Index)
	})

	return lines
}

// handleNoCanonicalText reassembles output when no canonical text was provided.
//
// Without canonical text to guide sorting, we flatten all blocks and reassemble
// them into sentences using punctuation as the only guide.
func (s *Sorter) handleNoCanonicalText(lines [][]Block) [][]Block {
	// Flatten all blocks
	var allBlocks []Block
	for _, line := range lines {
		allBlocks = append(allBlocks, line...)
	}

	// Reassemble into sentences based on punctuation
	var sentences [][]Block
	var sentence []Block
	for i, block := range allBlocks {
		sentence = append(sentence, block)
		isLastBlock := i == len(allBlocks)-1
		if endsWithSentencePunctuation(block.Text) || isLastBlock {
			sentences = append(sentences, sentence)
			sentence = nil
		}
	}

	return sentences
}

// serializeOutput converts lines into final block sequence with separators.
//
// Empty blocks (with no engine) are inserted between lines to indicate
// line breaks in the output.
func (s *Sorter) serializeOutput(lines [][]Block) []Block {
	var output []Block

	for _, line := range lines {
		output = append(output, line...)
		output = append(output, Block{}) // Empty block as line separator
	}

	return output
}
