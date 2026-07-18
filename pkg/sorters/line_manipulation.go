package sorters

import (
	"regexp"
	"strings"
)

// westernSentenceBoundary matches the end of a Western sentence: terminal
// punctuation (. ! ?), any closing quotes or brackets, then whitespace.
// Legal, quoted, and dialogue text routinely ends a sentence with a quote -
// "...an 'infant.' At one time..." - which a plain ". " search misses,
// leaving the sentence welded to its neighbors and unmatchable.
var westernSentenceBoundary = regexp.MustCompile(`[.!?]['"` + "‘’“”" + `)\]}\x{00BB}]*\s`)

// abs returns the absolute value of an integer.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// SplitParagraph splits a paragraph at the sentence-ending punctuation closest to its center.
//
// This is used when a line cannot be found as a whole, typically because it contains
// multiple logical sentences that should be processed separately.
//
// The function searches for Western (". ", "! ", "? ") and CJK ("。", "！", "？") sentence
// endings and splits at whichever is closest to the midpoint. The punctuation is retained
// at the end of the first part.
//
// Returns:
//   - A single-element slice containing the original paragraph if no split point found
//   - A two-element slice with the parts before and after the split
//
// Note: Also normalizes OCR spacing errors like " . " to ". " to help future passes.
func SplitParagraph(paragraph string) []string {
	midpoint := len(paragraph) / 2

	bestSplitPos := -1                         // where the second sentence begins
	bestDistanceFromMidpoint := len(paragraph) // worst-case distance

	consider := func(punctIndex, splitPos int) {
		// Skip abbreviations (e.g. "U.S. ", "Dr. ", "Ph.D. "): the abbrev
		// check looks at the text through the terminal punctuation.
		if !endsWithSentencePunctuation(paragraph[:punctIndex+1]) {
			return
		}
		if d := abs(punctIndex - midpoint); d < bestDistanceFromMidpoint {
			bestDistanceFromMidpoint = d
			bestSplitPos = splitPos
		}
	}

	// Western boundaries: punctuation + closing quotes/brackets + whitespace.
	for _, loc := range westernSentenceBoundary.FindAllStringIndex(paragraph, -1) {
		consider(loc[0], loc[1]) // loc[0] = punctuation, loc[1] = past the whitespace
	}

	// CJK boundaries stand on their own (no trailing space required).
	for _, token := range []string{"。", "！", "？"} {
		searchStart := 0
		for {
			idx := strings.Index(paragraph[searchStart:], token)
			if idx == -1 {
				break
			}
			absoluteIndex := searchStart + idx
			consider(absoluteIndex, absoluteIndex+len(token))
			searchStart = absoluteIndex + len(token)
		}
	}

	// If no boundary was found, return the entire paragraph unchanged.
	if bestSplitPos == -1 {
		return []string{paragraph}
	}

	sentences := []string{
		paragraph[:bestSplitPos],
		paragraph[bestSplitPos:],
	}

	// OCR sometimes returns " . " instead of ". " at the end of a line.
	// Make sure that's not an issue on the next pass, in case we need to
	// split again on punctuation.
	for i := range sentences {
		sentences[i] = strings.ReplaceAll(sentences[i], " .", ".")
		sentences[i] = strings.ReplaceAll(sentences[i], " !", "!")
		sentences[i] = strings.ReplaceAll(sentences[i], " ?", "?")
		sentences[i] = strings.TrimSpace(sentences[i])
	}

	return sentences
}

// AssembleContiguousLines groups consecutive blocks into lines.
//
// Takes a slice of blocks sorted by index and assembles them into lines based on:
// 1. Consecutive indices (block.Index == lastBlock.Index + 1), OR
// 2. Same LineId (blocks from the same OCR visual line)
//
// This ensures that blocks from the same visual line stay together even if
// some intermediate blocks were used in other paths.
//
// For example, blocks with indices [5, 6, 7, 10, 11] would become:
//   - Line 1: [5, 6, 7] (consecutive)
//   - Line 2: [10, 11] (consecutive)
//
// But blocks [5, 6, 8, 9] with LineIds ["line1", "line1", "line1", "line1"] would stay together.
func AssembleContiguousLines(blocks []Block) [][]Block {
	var lines [][]Block
	var currentLine []Block

	for _, block := range blocks {
		if len(currentLine) == 0 {
			currentLine = append(currentLine, block)
			continue
		}
		lastBlock := currentLine[len(currentLine)-1]

		// Calculate vertical gap between blocks (as a fraction of block height)
		lastBottom := lastBlock.BoundingBox.Top + lastBlock.BoundingBox.Height
		currentTop := block.BoundingBox.Top
		verticalGap := currentTop - lastBottom

		// Use the larger of the two block heights as a reference
		refHeight := lastBlock.BoundingBox.Height
		if block.BoundingBox.Height > refHeight {
			refHeight = block.BoundingBox.Height
		}

		// If gap is more than 1.5x the reference height, it's likely a paragraph break
		// This threshold is chosen to detect paragraph breaks while allowing
		// for normal line spacing within a paragraph. NOTE: deliberately the
		// vertical axis even for vertical reading orders — tesseract's
		// vertical line ids span columns, and splitting at column wraps
		// (the reading-advance axis) measured -21 on
		// japanese-single-vertical/tesseract.
		const paragraphBreakThreshold = 1.5
		isLargeGap := verticalGap > paragraphBreakThreshold*refHeight

		// Determine if we should group this block with the previous line
		// Priority order:
		// 1. If LineId is available and different, start a new line (paragraph/line break from OCR)
		// 2. If there's a large vertical gap, start a new line (paragraph break from spacing)
		// 3. If indices are consecutive, continue the line
		// 4. Otherwise, start a new line

		hasLineId := block.LineId != "" && lastBlock.LineId != ""
		sameLineId := hasLineId && block.LineId == lastBlock.LineId
		consecutiveIndices := block.Index == lastBlock.Index+1

		shouldGroup := false
		if hasLineId {
			// LineId is available - use it as the primary grouping signal
			shouldGroup = sameLineId && !isLargeGap
		} else {
			// No LineId - fall back to consecutive indices and gap detection
			shouldGroup = consecutiveIndices && !isLargeGap
		}

		if shouldGroup {
			currentLine = append(currentLine, block)
		} else {
			lines = append(lines, currentLine)
			currentLine = []Block{block}
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}
	return lines
}

// splitByAbsentWords splits a line at words that have no matching OCR blocks.
//
// When a line cannot be found, it's often because certain words don't exist in
// the OCR output. This function splits the line at those missing words, allowing
// the remaining segments to be processed independently on subsequent passes.
//
// Parameters:
//   - s: The original text line to split
//   - absentWords: Set of normalized words that have no OCR blocks
//
// Returns a slice of text segments, excluding the absent words themselves.
//
// Example: "Hello world foo bar" with absentWords={"foo"} returns ["Hello world", "bar"]
func (s *Sorter) splitByAbsentWords(text string, absentWords map[string]bool) []string {
	// Use handler's tokenization instead of strings.Fields
	tokens := s.handler.Tokenize(text)
	var segments []string
	var currentSegment []string

	for _, token := range tokens {
		normalizedToken := NormalizeText(token)

		// Normalizing may convert punctuation like parentheses into spaces.
		normalizedParts := strings.Split(normalizedToken, " ")
		normalizedToken = normalizedParts[0]

		if absentWords[normalizedToken] {
			// If the token is an absent word, flush the current segment (if any)
			if len(currentSegment) > 0 {
				segments = append(segments, strings.Join(currentSegment, " "))
				currentSegment = []string{}
			}
		} else {
			// Otherwise, add the token to the current segment
			currentSegment = append(currentSegment, token)
		}
	}

	// Append any remaining tokens as the last segment.
	if len(currentSegment) > 0 {
		segments = append(segments, strings.Join(currentSegment, " "))
	}
	return segments
}
