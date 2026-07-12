package sorters

import (
	"cmp"
	"math"
	"sort"

	"golang.org/x/exp/slices"
)

// chainBuildResult contains the result of building a candidate block chain.
type chainBuildResult struct {
	chain        [][]Block
	missingWords map[string]bool
	holeSlots    []bool // slots bridged as wildcard holes
}

// buildChainForLine constructs a chain of candidate blocks for each word in the line.
// Returns the chain and any words that have no matching OCR blocks.
// With EnableChainHoles, a small fraction of empty slots become wildcard
// holes instead of being reported missing (see holes.go). Only words that
// fail every mechanism are reported missing, which triggers the split path
// in Sort().
func (s *Sorter) buildChainForLine(canonicalWords []string) chainBuildResult {
	var chain [][]Block
	missingWords := make(map[string]bool)

	for _, word := range canonicalWords {
		chain = append(chain, s.mapped[word])
	}

	var holeSlots []bool
	if s.config.EnableChainHoles {
		var missingPositions []int
		for i, slot := range chain {
			if len(slot) == 0 {
				missingPositions = append(missingPositions, i)
			}
		}
		budget := int(s.config.MaxHoleFraction * float64(len(chain)))
		realSlots := len(chain) - len(missingPositions)
		if len(missingPositions) > 0 && len(missingPositions) <= budget && realSlots >= MinChainLength {
			holeSlots = make([]bool, len(chain))
			for _, i := range missingPositions {
				holeSlots[i] = true
			}
			s.logger.Debugf("bridging %d hole(s) in %d-word line", len(missingPositions), len(chain))
		}
	}

	// Report missing words only for slots that no mechanism could cover.
	for i, slot := range chain {
		if len(slot) == 0 && (holeSlots == nil || !holeSlots[i]) {
			missingWords[canonicalWords[i]] = true
		}
	}

	return chainBuildResult{
		chain:        chain,
		missingWords: missingWords,
		holeSlots:    holeSlots,
	}
}

// shouldSkipLineInEarlyPass determines if a line should be skipped in early passes.
// Early passes focus on longer, more distinctive lines.
func shouldSkipLineInEarlyPass(pass int, wordCount int, minWords int) bool {
	return pass <= EarlyPassThreshold && wordCount < minWords
}

// initializePathfinding resets the pathfinding state for a new line.
func (s *Sorter) initializePathfinding() {
	s.candidatePaths = nil
	s.shortest = math.MaxFloat64
	s.perm = 0
}

// getPassPermutationLimit returns the permutation limit for the current pass.
func (s *Sorter) getPassPermutationLimit() int {
	if s.config.PermutationsPerPass == 0 {
		return s.config.MaxPermutations
	}
	return s.config.PermutationsPerPass
}

// shouldExitPassLoop determines if we should exit the multi-pass loop.
// Exits when no progress is being made (no new lines found and no lines
// split), but never at pass <= minPass. minPass is 0 by default; with
// EnableShortLineAnchoring it is EarlyPassThreshold+1 so the loop survives
// long enough for the early-pass filter to relax and short lines to get
// their first attempt (TESTING.md issue #1).
func shouldExitPassLoop(pass int, linesFoundThisPass int, previousLineCount int, currentLineCount int, minPass int) bool {
	if pass <= minPass {
		return false
	}
	return pass > 0 && linesFoundThisPass == 0 && previousLineCount == currentLineCount
}

// shouldSkipChain determines if a chain should be skipped during pathfinding.
// Returns true if the chain is too short (empty or single word).
func shouldSkipChain(chain [][]Block) bool {
	if len(chain) == 0 {
		return true
	}
	if len(chain) == SingleWordChainLength {
		return true
	}
	return false
}

// applyPrecurseOptimization analyzes a limited subset of the chain to find the best starting point.
// This optimization rotates the first word's candidates to start from the most promising block.
// Returns the (potentially rotated) chain.
func (s *Sorter) applyPrecurseOptimization(chain [][]Block, passPermutations int) [][]Block {
	if s.config.PrecurseLength == 0 || len(chain) <= s.config.PrecurseLength {
		return chain
	}

	limitedChain := chain[:s.config.PrecurseLength]
	bestStartIndex := s.precurse(limitedChain, passPermutations)

	// If we found a viable starting point, rotate the first word options to start there
	if bestStartIndex > -1 {
		rotatePos, _ := slices.BinarySearchFunc(chain[0], Block{Index: bestStartIndex}, func(a, b Block) int {
			return cmp.Compare(a.Index, b.Index)
		})
		chain[0] = append(chain[0][rotatePos:], chain[0][:rotatePos]...)
	}

	// Reset pathfinding state after precurse
	s.initializePathfinding()

	return chain
}

// findBestPathsForLine performs pathfinding and returns sorted paths.
// Executes the recurse algorithm and sorts paths by spatial distance (shortest first).
func (s *Sorter) findBestPathsForLine(chain [][]Block, passPermutations int) []Path {
	// Recurse through the chain and find all valid paths
	s.recurse(chain, Block{}, Path{}, 0, passPermutations, 0)

	// Sort the found paths by length. Shortest is first and best.
	sort.Slice(s.candidatePaths, func(i, j int) bool {
		return s.candidatePaths[i].Length < s.candidatePaths[j].Length
	})

	return s.candidatePaths
}
