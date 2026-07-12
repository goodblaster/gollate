package sorters

import "sort"

// Wildcard chain holes and wrap bridging (experimental, EnableChainHoles /
// EnableWrapBridging).
//
// A hole is a canonical word with no matching block (exact or approximate):
// instead of splitting the line, pathfinding bridges the slot for
// HolePathPenalty and, after a path is accepted, may claim an unclaimed
// block sitting spatially inside the gap. Geometry decides; text similarity
// only confirms. Match-only - block text is never rewritten.
//
// Wrap bridging is the fix for TESTING.md issue #3: the distance function
// already recognizes legitimate wraps (isWrappedToNextLine) but their cost
// (BaseLineWrap 1.0 + gap) always exceeds MaxWordDistance, so the flat
// threshold in recurse made multi-visual-line paths unreachable in any
// config.

// stepDistance computes the path cost of stepping previous -> candidate and
// whether the step is admissible. pendingHoles is the number of wildcard
// slots bridged since the last consumed block.
func (s *Sorter) stepDistance(previous, candidate Block, pendingHoles int) (float64, bool) {
	distance := s.distance(previous, candidate)
	if distance <= s.config.MaxWordDistance {
		return distance, true
	}

	// The gap left by skipped words can misclassify an otherwise sequential
	// step (the withinDistance gate fails), so reclassify with an allowance
	// proportional to the number of holes being bridged.
	if pendingHoles > 0 {
		if d, ok := s.holeBridgeDistance(previous, candidate, pendingHoles); ok {
			return d, true
		}
	}

	// A structurally legitimate wrap to the next visual line: admit the step
	// and let its real cost (already BaseLineWrap + gap) compete in path
	// length, so paths with fewer wraps still win.
	if s.config.EnableWrapBridging && previous.Engine() != "" &&
		isWrappedToNextLine(previous, candidate, s.config.ReadingOrder) {
		return distance, true
	}

	return distance, false
}

// holeBridgeDistance classifies a step that lands after pendingHoles skipped
// words. Same-line sequential steps are admitted with a per-hole gap
// allowance; steps across a visual wrap are admitted only when wrap bridging
// is enabled. Column jumps and unclassifiable steps stay rejected.
func (s *Sorter) holeBridgeDistance(previous, candidate Block, pendingHoles int) (float64, bool) {
	order := s.config.ReadingOrder

	if onSameLineWithOrder(previous, candidate, order) && maybeSequentialWithOrder(previous, candidate, order) {
		gap := primaryAxisDistance(previous, candidate, order)
		allowance := float64(pendingHoles) * HoleGapAllowancePerWord
		if gap >= 0 && BaseSequential+gap <= s.config.MaxWordDistance+allowance {
			return BaseSequential + gap, true
		}
	}

	if s.config.EnableWrapBridging && isWrappedToNextLine(previous, candidate, order) {
		return BaseLineWrap + secondaryAxisDistance(previous, candidate, order), true
	}

	return 0, false
}

// fillHoles attempts to claim a block for each hole in an accepted path.
// A claimable block must be unclaimed (still mapped), sit spatially between
// the hole's matched neighbors (same visual line, sequential on both sides -
// wrap-adjacent holes are left empty), and pass text confirmation: exact
// normalized text for words of at least HoleMinConfirmLength runes, spatial
// containment alone for shorter words (including single-character CJK
// tokens). An edit-distance relaxation used to sit here; it was measured
// worthless (suite -0.02, apple.com benchmark exactly 0) and removed -
// misread words are line repair's job (linerepair.go), which identifies
// them by position instead of similarity. Nodes are updated in place;
// claimed blocks are unmapped later with the rest of the path.
func (s *Sorter) fillHoles(path *Path, canonicalWords []string) {
	order := s.config.ReadingOrder

	for i, node := range path.Nodes {
		if node != HoleNode {
			continue
		}

		prevIdx, nextIdx := -1, -1
		for j := i - 1; j >= 0; j-- {
			if path.Nodes[j] >= 0 {
				prevIdx = path.Nodes[j]
				break
			}
		}
		for j := i + 1; j < len(path.Nodes); j++ {
			if path.Nodes[j] >= 0 {
				nextIdx = path.Nodes[j]
				break
			}
		}
		if prevIdx < 0 || nextIdx < 0 {
			s.metrics.HolesLeftEmpty++
			continue
		}
		prev := s.input[prevIdx]
		next := s.input[nextIdx]
		word := []rune(canonicalWords[i])

		// Deterministic scan: sorted keys, best candidate by tightest fit,
		// ties broken by block Index.
		keys := make([]string, 0, len(s.mapped))
		for key := range s.mapped {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		bestIndex := -1
		var bestCost float64
		for _, key := range keys {
			for _, candidate := range s.mapped[key] {
				if path.Contains(candidate.Index) {
					continue
				}
				if !onSameLineWithOrder(prev, candidate, order) || !maybeSequentialWithOrder(prev, candidate, order) {
					continue
				}
				if !onSameLineWithOrder(candidate, next, order) || !maybeSequentialWithOrder(candidate, next, order) {
					continue
				}
				if len(word) >= HoleMinConfirmLength && candidate.NormedText != string(word) {
					continue
				}
				cost := primaryAxisDistance(prev, candidate, order) + primaryAxisDistance(candidate, next, order)
				if bestIndex == -1 || cost < bestCost || (cost == bestCost && candidate.Index < bestIndex) {
					bestIndex = candidate.Index
					bestCost = cost
				}
			}
		}

		if bestIndex >= 0 {
			s.logger.Debugf("hole filled: '%s' <- '%s'", canonicalWords[i], s.input[bestIndex].Text)
			path.Nodes[i] = bestIndex
			s.metrics.HolesFilled++
		} else {
			s.metrics.HolesLeftEmpty++
		}
	}
}

// levenshteinDistanceRunes is rune-based edit distance, used to text-confirm
// gap-fill claims. The byte-based levenshteinDistance overcounts
// substitutions of multi-byte characters (accented Latin, etc.), which
// matters when comparing against a distance threshold of 1.
func levenshteinDistanceRunes(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}
