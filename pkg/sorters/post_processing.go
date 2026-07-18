package sorters

import (
	"cmp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/exp/slices"
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

	// Special case: handle documents with no canonical text
	if len(lines) <= 1 {
		allSentences = s.handleNoCanonicalText(allSentences)
	}

	// Serialize to final output format with line separators
	return s.serializeOutput(allSentences)
}

// groupAndSortByCanonicalLine groups all sentences by their canonical line
// and sorts sentences within each group spatially (top to bottom).
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
	}

	var allSentences []sentenceInfo

	// Found sentences carry their canonical line identity from assembly time.
	for _, sentence := range foundSentences {
		if sentence.isBlank {
			allSentences = append(allSentences, sentenceInfo{
				blocks:       []Block{},
				originalLine: sentence.originalLine,
				topPosition:  blankSentencePos, // Blanks sort first within their group
				isBlank:      true,
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
		})
	}

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

	// Add leftover sentences - match them to canonical lines by text content
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
			if i < len(sentence)-1 && sentence[i+1].Engine() != "" {
				if s.handler.NeedsSpaceBetween(block.Text, sentence[i+1].Text) {
					sentenceText.WriteByte(' ')
				}
			}
		}
		normalizedSentence := NormalizeText(sentenceText.String())
		normalizedSentenceNoSpaces := strings.ReplaceAll(normalizedSentence, " ", "")

		// Find the canonical line with the best match (highest overlap)
		// Match against the deduplicated canonical lines (longest version of each OriginalLine)
		// Iterate in ascending OriginalLine order so that overlap ties resolve
		// to the earliest line in the document (deterministic, and consistent
		// with human reading order when placement is ambiguous).
		lineNumbers := make([]int, 0, len(canonicalByOriginalLine))
		for lineNum := range canonicalByOriginalLine {
			lineNumbers = append(lineNumbers, lineNum)
		}
		sort.Ints(lineNumbers)

		bestMatch := -1
		bestOverlap := 0.0
		for _, lineNum := range lineNumbers {
			line := canonicalByOriginalLine[lineNum]
			normalized := NormalizeText(line.OriginalText)
			normalizedNoSpaces := strings.ReplaceAll(normalized, " ", "")

			// Calculate overlap: how much of the sentence is in this canonical line
			// and how much of the canonical line is in this sentence
			sentenceInLine := strings.Contains(normalizedNoSpaces, normalizedSentenceNoSpaces)
			lineInSentence := strings.Contains(normalizedSentenceNoSpaces, normalizedNoSpaces)

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
					bestMatch = line.OriginalLine
				}
			}
		}

		if bestMatch != -1 {
			originalLine = bestMatch
		}

		allSentences = append(allSentences, sentenceInfo{
			blocks:       sentence,
			originalLine: originalLine,
			topPosition:  topPos,
			tiebreak:     tie,
			isBlank:      false,
		})
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
						result = append(result, []Block{})
					}
				}

				// Append this sentence's blocks
				result = append(result, sent.blocks)

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
				result = append(result, mergedBlocks)
			}
		}
	}

	return result
}

// assembledSentence is a found line's blocks paired with the canonical line
// they belong to, so downstream grouping never depends on positional sync
// with the lines array.
type assembledSentence struct {
	originalLine int
	isBlank      bool
	blocks       []Block
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

	var result [][]Block
	for _, sentence := range sentences {
		// Extract full text from this sentence
		var fullText strings.Builder
		for i, block := range sentence {
			fullText.WriteString(block.Text)
			if i < len(sentence)-1 && sentence[i+1].Engine() != "" {
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

		for _, canLine := range unfoundLines {
			normalized := NormalizeText(canLine.OriginalText)
			normalizedNoSpaces := strings.ReplaceAll(normalized, " ", "")

			// Check if this canonical line is contained in the sentence
			if startIdx := strings.Index(normalizedSentenceNoSpaces, normalizedNoSpaces); startIdx != -1 {
				endIdx := startIdx + len(normalized)
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
				totalMatchedLength += len(strings.ReplaceAll(NormalizeText(m.line.OriginalText), " ", ""))
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
				upcomingStartsUpper := len(block.Text) > 0 && unicode.IsUpper(rune(block.Text[0]))
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
