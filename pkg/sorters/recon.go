package sorters

// Reconciliation pass (experimental, EnableReconciliationPass) - mechanism C.
//
// After the main pass loop, unfound leaf fragments get one more chance,
// anchored by where their canonical neighbors landed: candidate paths must
// lie within ReconSpatialWindow of the anchors and contribute at least
// ReconMinExactAnchors exactly-matched words. Among qualifying paths the one
// nearest its anchors wins, so duplicate short lines resolve to the right
// region. Runs in sweeps: a reconciled line becomes an anchor for the next
// sweep, letting chains of short neighbors resolve incrementally.

// maxReconSweeps bounds the fixpoint iteration; each sweep only runs if the
// previous one reconciled something.
const maxReconSweeps = 5

func (s *Sorter) reconciliationPass() {
	if !s.config.EnableReconciliationPass {
		return
	}

	for sweep := 0; sweep < maxReconSweeps; sweep++ {
		reconciled := 0
		for i := range s.lines.lines {
			if s.reconcileLine(&s.lines.lines[i]) {
				reconciled++
			}
		}
		s.logger.Debugf("reconciliation sweep %d: %d line(s) reconciled", sweep, reconciled)
		if reconciled == 0 {
			return
		}
	}
}

// reconcileLine attempts to match one unfound leaf line against the
// remaining block pool, gated by anchor proximity. Returns true if the line
// was reconciled.
func (s *Sorter) reconcileLine(line *Line) bool {
	if line.Found || line.Split || line.IsBlank {
		return false
	}
	words := s.handler.Tokenize(line.Normalized)
	if len(words) == 0 {
		return false
	}

	anchors := s.anchorBlocks(line)
	if len(anchors) == 0 {
		return false // nothing matched nearby yet; a later sweep may help
	}

	chainResult := s.buildChainForLine(words)
	if len(chainResult.missingWords) > 0 {
		return false // recon never splits; the line stays unhandled
	}
	s.holeSlots = chainResult.holeSlots
	chain := chainResult.chain

	var accepted Path
	found := false

	if len(chain) == 1 {
		// Single-word lines cannot be pathfound (no spatial sequence), but
		// with anchors they are spatially pinned. Only allowed when the
		// exact-anchor requirement permits a single contributing word.
		if s.config.ReconMinExactAnchors > 1 {
			return false
		}
		bestScore := s.config.ReconSpatialWindow
		for _, candidate := range chain[0] {
			p := Path{Nodes: []int{candidate.Index}}
			if score := s.anchorDistance(anchors, p); score <= bestScore {
				accepted, bestScore, found = p, score, true
			}
		}
	} else {
		s.initializePathfinding()
		passPermutations := s.getPassPermutationLimit()
		chain = s.applyPrecurseOptimization(chain, passPermutations)
		paths := s.findBestPathsForLine(chain, passPermutations)

		bestScore := s.config.ReconSpatialWindow
		for _, p := range paths {
			if s.exactMatchCount(p) < s.config.ReconMinExactAnchors {
				continue
			}
			if score := s.anchorDistance(anchors, p); score <= bestScore {
				accepted, bestScore, found = p, score, true
			}
		}
	}

	if !found {
		return false
	}

	if s.config.EnableChainHoles {
		for _, node := range accepted.Nodes {
			if node == HoleNode {
				s.metrics.HolesBridged++
			}
		}
		s.fillHoles(&accepted, words)
	}

	s.logger.Debugf("reconciled: '%s'", line.OriginalText)
	line.FoundPath = accepted
	line.Found = true
	s.metrics.LinesFound++
	s.metrics.LinesReconciled++
	s.unmapWords(accepted)
	return true
}

// exactMatchCount counts path nodes that consumed a block via exact lookup
// (holes contribute nothing).
func (s *Sorter) exactMatchCount(path Path) int {
	count := 0
	for _, node := range path.Nodes {
		if node >= 0 {
			count++
		}
	}
	return count
}
