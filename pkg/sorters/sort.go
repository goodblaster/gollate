package sorters

import (
	"cmp"
	"fmt"
	"sort"
	"time"

	"golang.org/x/exp/slices"
)

// Sort performs multi-pass pathfinding to reconstruct reading order from OCR blocks.
//
// The algorithm works by:
// 1. Breaking canonical text into lines and normalizing them
// 2. For each line, building a "chain" of all possible OCR blocks matching each word
// 3. Using recursive pathfinding to find the shortest spatial path through the chain
// 4. Making multiple passes, starting with longest lines (most distinctive)
// 5. Early passes skip short lines to focus computational budget on unique text
// 6. Later passes handle remaining lines with fewer permutations to explore
//
// The multi-pass strategy is necessary because:
// - Long documents have too many permutations to solve in one pass
// - Finding longer lines first eliminates those blocks from the search space
// - Lines may need to be split into smaller segments if words are missing
// - Split lines get re-processed on subsequent passes
func (s *Sorter) Sort() ([]Block, error) {
	start := time.Now()
	defer func() {
		s.elapsed = time.Since(start)
		s.metrics.ElapsedTime = s.elapsed
	}()

	var line *Line
	var linesFoundThisPass int
	previousLineCount := len(s.lines.lines)

	// With short-line anchoring, the loop must survive until the early-pass
	// filter relaxes, or short lines are never attempted at all (issue #1).
	minPass := 0
	if s.config.EnableShortLineAnchoring {
		minPass = EarlyPassThreshold + 1
	}

	for pass := 0; pass <= s.config.MaxPasses; pass++ {
		s.metrics.PassesCompleted = pass + 1
		s.logger.Debugf("Pass %d", pass)
		s.logger.Debugf("last pass found %d", linesFoundThisPass)

		currentLineCount := len(s.lines.lines)

		if shouldExitPassLoop(pass, linesFoundThisPass, previousLineCount, currentLineCount, minPass) {
			s.logger.Debug("No lines found on last pass; exiting.")

			// Dump all the lines that were not found.
			for _, line := range s.lines.lines {
				if !line.Found {
					s.logger.Debugf("Missing line: %s", line.OriginalText)
				}
			}

			break
		}

		linesFoundThisPass = 0
		previousLineCount = currentLineCount

		s.lines.index = 0

		// Always re-sort by length of line.
		sort.Slice(s.lines.lines, func(i, j int) bool {
			return len(s.lines.lines[i].Normalized) > len(s.lines.lines[j].Normalized)
		})

		// Set permutation limit for this pass
		passPermutations := s.getPassPermutationLimit()

		for s.lines.Next(&line) {
			if line.Split || line.Found {
				continue
			}

			// Split the line into tokens (words for English, characters for CJK).
			canonicalWords := s.handler.Tokenize(line.Normalized)

			// Early passes: ignore short lines to focus on finding longer, more distinctive text first
			if shouldSkipLineInEarlyPass(pass, len(canonicalWords), s.config.MinWordsForEarlyPasses) {
				continue
			}

			// Reset pathfinding state for this line
			s.initializePathfinding()
			var chain [][]Block // The chain is the mandatory word order and contains all possible permutations.

			// Build the chain. The chain is all the options that will be used to test all permutations.
			// If the phrase you are looking for is "the quick brown fox", the chain will contain lists
			// of all those words, in order: [[]the, []quick, []brown, []fox]
			chainResult := s.buildChainForLine(canonicalWords)
			chain = chainResult.chain
			missingWords := chainResult.missingWords
			s.holeSlots = chainResult.holeSlots

			if len(missingWords) > 0 {
				keys := make([]string, 0, len(missingWords))
				for k := range missingWords {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				s.logger.Debugf("missing required term(s): '%v' in line: '%s'", keys, line.OriginalText)
				s.splitLine(missingWords)
				continue
			}

			// Skip chains that are too short for pathfinding
			if shouldSkipChain(chain) {
				if len(chain) == 0 {
					s.logger.Debugf("Skipping line with no words: %s", line.OriginalText)
				} else {
					s.logger.Debugf("Skipping line with single word: %s", line.OriginalText)
				}
				continue
			}

			// PRECURSE - Analyze limited chain first to find the best starting path
			// This optimization helps the full recursion use rotation more effectively
			chain = s.applyPrecurseOptimization(chain, passPermutations)

			// Find all valid paths through the chain and sort by spatial distance
			s.candidatePaths = s.findBestPathsForLine(chain, passPermutations)

			// We found nothing. See if this is a paragraph we can split into smaller parts.
			if len(s.candidatePaths) == 0 {
				s.splitLine(nil)
				continue
			}

			// Use the shortest path; for short lines with anchoring enabled,
			// near-tied paths compete on proximity to the matched blocks of
			// the line's canonical neighbors (issue #2: duplicate lines like
			// "Learn more" must pick the instance in the right region).
			path := s.candidatePaths[0]
			if s.config.EnableShortLineAnchoring && len(canonicalWords) < s.config.MinWordsForEarlyPasses {
				path = s.anchorRerank(line, s.candidatePaths)
			}
			linesFoundThisPass++
			s.metrics.LinesFound++

			// Account for bridged holes, then try to claim blocks sitting
			// spatially inside each gap (mutates path.Nodes in place).
			if s.config.EnableChainHoles {
				for _, node := range path.Nodes {
					if node == HoleNode {
						s.metrics.HolesBridged++
					}
				}
				s.fillHoles(&path, canonicalWords)
			}

			if s.debugPrintPathNodes {
				fmt.Print("FOUND -- ")
				for _, index := range path.Nodes {
					if index < 0 {
						fmt.Print("<hole> ")
						continue
					}
					fmt.Print(s.input[index].Text, " ")
				}
				fmt.Println()
			}

			// Link the line to the found path.
			line.FoundPath = path
			line.Found = true

			// If we've found a path, make sure none of the words in it can be used again.
			s.unmapWords(path)
		}
	}
	s.logger.Debugf("last pass found %d", linesFoundThisPass)

	// Reconciliation pass (experimental): anchor-gated rescue of unfound
	// leaf fragments, now that found neighbors exist to anchor against.
	s.reconciliationPass()

	// Mark blank lines as found if they appear between found content lines
	// This preserves paragraph structure while avoiding orphan blank lines
	s.markIntermediateBlankLines()

	s.output = s.postSortBlocks()
	return s.output, nil
}

// splitLine breaks an unmatchable line into smaller segments for later
// passes: complete sentences first, then - when specific words are known to
// be absent from OCR - around those absent words.
//
// Measured (see TESTING.md): with
// the promoted per-language config this cascade is inert on most of the
// suite but still worth 3-7 points on noisy Latin pages, so it stays.
//
// Known limitation: if an absent word is at the beginning of a line, so
// splitting yields a single changed line, SplitAndReset ignores the change.
func (s *Sorter) splitLine(missingWords map[string]bool) {
	s.metrics.LinesSplit++
	s.lines.SplitAndReset(func(line Line) []Line {
		segments := SplitParagraph(line.OriginalText)
		if len(segments) <= 1 && len(missingWords) > 0 {
			segments = s.splitByAbsentWords(line.OriginalText, missingWords)
		}

		var lines []Line
		for _, segment := range segments {
			lines = append(lines, Line{
				OriginalLine: line.OriginalLine,
				OriginalText: segment,
				Normalized:   NormalizeText(segment),
			})
		}
		return lines
	})
}

// markIntermediateBlankLines marks blank lines as found if they appear between found content lines.
// This preserves paragraph structure while avoiding orphan blank lines at the start/end
// or between large blocks of unfound content.
//
// Note: Lines are sorted by length during processing, so we use OriginalLine to determine
// canonical position rather than array index.
func (s *Sorter) markIntermediateBlankLines() {
	lines := s.lines.List()

	// Find first and last found content line BY CANONICAL POSITION (OriginalLine)
	firstFoundOriginal := -1
	lastFoundOriginal := -1
	for _, line := range lines {
		if line.Found && !line.IsBlank {
			if firstFoundOriginal == -1 || line.OriginalLine < firstFoundOriginal {
				firstFoundOriginal = line.OriginalLine
			}
			if line.OriginalLine > lastFoundOriginal {
				lastFoundOriginal = line.OriginalLine
			}
		}
	}

	// Mark blank lines between first and last found content lines (by canonical position)
	blankLinesMarkedNow := 0
	for i := range lines {
		line := &s.lines.lines[i] // Get mutable reference
		if line.IsBlank && !line.Found {
			// Check if this blank line falls between found lines in canonical order
			if line.OriginalLine > firstFoundOriginal && line.OriginalLine < lastFoundOriginal {
				// Look for nearby found content within 5 lines in canonical order
				hasNearbyContent := false
				lookAhead := 5

				// Check for found lines within lookAhead in canonical order
				for j := range lines {
					if lines[j].Found && !lines[j].IsBlank {
						distance := abs(lines[j].OriginalLine - line.OriginalLine)
						if distance > 0 && distance <= lookAhead {
							hasNearbyContent = true
							break
						}
					}
				}

				if hasNearbyContent {
					line.Found = true
					blankLinesMarkedNow++
				}
			}
		}
	}
}

// precurse analyzes a limited subset of the word chain to find the best starting point.
//
// This optimization helps the full recursion by identifying which OCR block instance
// should be tried first. The rotation optimization in recurse() can then reorder
// the search to start from this most promising path, dramatically reducing the
// number of permutations explored.
//
// Returns the index of the best starting OCR block, or -1 if no path was found.
func (s *Sorter) precurse(chain [][]Block, maxPermutations int) int {
	s.recurse(chain, Block{}, Path{}, 0, maxPermutations, 0)

	if len(s.candidatePaths) == 0 {
		return -1
	}

	sort.Slice(s.candidatePaths, func(i, j int) bool {
		return s.candidatePaths[i].Length < s.candidatePaths[j].Length
	})

	return s.candidatePaths[0].Nodes[0]
}

// recurse performs depth-first search through the word chain to find valid reading paths.
//
// The algorithm explores permutations of OCR blocks, finding spatially coherent sequences
// that match the expected word order from the canonical text.
//
// Parameters:
//   - chain: For each word position, a list of all OCR blocks containing that word
//   - previousBlock: The OCR block selected in the previous recursion level
//   - path: The accumulated path from the start to previousBlock
//   - chainPosition: Current position in the chain (which word we're looking for)
//   - maxPermutations: Maximum number of permutations to explore before giving up
//
// Optimizations:
//   - Permutation limit: Stops search after maxPermutations to prevent exponential blowup
//   - Shortest path pruning: Abandons paths longer than the current best
//   - Rotation optimization: Reorders candidates to try spatially nearest blocks first
//   - Distance filtering: Rejects word pairs that are too far apart spatially
//
// The function populates s.candidatePaths with all valid complete paths found.
func (s *Sorter) recurse(chain [][]Block, previousBlock Block, path Path, chainPosition int, maxPermutations int, pendingHoles int) bool {
	if chainPosition == len(chain) {
		s.candidatePaths = append(s.candidatePaths, path)
		if path.Length < s.shortest {
			s.shortest = path.Length
		}

		// Early exit optimization: if we found a perfect path with zero/near-zero distance,
		// stop searching immediately as this is a "sure thing"
		const nearZeroThreshold = 0.001
		if path.Length < nearZeroThreshold {
			return false // Signal to stop searching (found perfect match)
		}

		return true // Continue searching for better paths
	}

	// Wildcard hole slot: bridge the missing word without consuming a block.
	// Not a branching choice, so it doesn't count against the permutation
	// budget; the hole penalty lands in path length instead.
	if chainPosition < len(s.holeSlots) && s.holeSlots[chainPosition] {
		pathCopy := path.Copy()
		pathCopy.AppendHole(s.config.HolePathPenalty)
		return s.recurse(chain, previousBlock, pathCopy, chainPosition+1, maxPermutations, pendingHoles+1)
	}

	s.perm++
	s.metrics.TotalPermutationsExplored++
	if s.perm > maxPermutations {
		s.logger.Debug("Permutation limit reached")
		return false
	}

	if path.Length > s.shortest {
		return false
	}

	candidateBlocks := chain[chainPosition]

	// Rotation optimization: reorder candidates to start with most likely next word
	// This dramatically reduces the number of paths explored
	if s.config.RotationOptimization && chainPosition > 0 {
		previousIndex := previousBlock.Index
		rotatePos, _ := slices.BinarySearchFunc(candidateBlocks, Block{Index: previousIndex + 1}, func(a, b Block) int {
			return cmp.Compare(a.Index, b.Index)
		})
		candidateBlocks = append(candidateBlocks[rotatePos:], candidateBlocks[:rotatePos]...)
	}

	for _, candidate := range candidateBlocks {
		if path.Contains(candidate.Index) {
			continue
		}

		// Step admission: the plain threshold, plus hole-bridging and
		// wrap-bridging reclassification when those flags are on (holes.go).
		distance, ok := s.stepDistance(previousBlock, candidate, pendingHoles)
		if !ok {
			continue
		}

		pathCopy := path.Copy()
		pathCopy.Append(candidate, distance)

		continueSearching := s.recurse(chain, candidate, pathCopy, chainPosition+1, maxPermutations, 0)
		if !continueSearching {
			// Propagate stop signal up the call stack (hit limit or found perfect match)
			return false
		}
	}

	return true
}

// SortedBlocks returns the sorted output blocks.
func (s *Sorter) SortedBlocks() []Block {
	return s.output
}

// SortedLines returns the sorted output as a slice of text lines.
func (s *Sorter) SortedLines() []string {
	return convertBlocksToLines(s.output, s.handler)
}

// Metrics returns performance and diagnostic information from the sort operation.
func (s *Sorter) Metrics() SortMetrics {
	return s.metrics
}
