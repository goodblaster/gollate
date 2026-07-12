package sorters

import "math"

// Context anchoring (experimental, EnableShortLineAnchoring; also the shared
// machinery for the reconciliation pass). See TESTING.md issue #2.
//
// A short duplicated line ("Learn more", "Buy") produces several candidate
// paths that are near-ties on internal compactness - the only signal that
// distinguishes the right instance is where the line's canonical neighbors
// were matched. Anchors are the matched blocks of the nearest found
// neighbors (by canonical line number); candidates within AnchorTieEpsilon
// of the shortest path compete on proximity to them instead.

// anchorBlocks returns the matched blocks of the found lines canonically
// nearest to target: all found, non-blank lines at the minimal
// |OriginalLine - target.OriginalLine| (fragments of the same paragraph win
// at delta 0). Empty when nothing relevant has been matched yet.
func (s *Sorter) anchorBlocks(target *Line) []Block {
	minDelta := -1
	for i := range s.lines.lines {
		ln := &s.lines.lines[i]
		if !ln.Found || ln.IsBlank || len(ln.FoundPath.Nodes) == 0 {
			continue
		}
		delta := abs(ln.OriginalLine - target.OriginalLine)
		if minDelta == -1 || delta < minDelta {
			minDelta = delta
		}
	}
	if minDelta == -1 {
		return nil
	}

	var anchors []Block
	for i := range s.lines.lines {
		ln := &s.lines.lines[i]
		if !ln.Found || ln.IsBlank || len(ln.FoundPath.Nodes) == 0 {
			continue
		}
		if abs(ln.OriginalLine-target.OriginalLine) != minDelta {
			continue
		}
		for _, node := range ln.FoundPath.Nodes {
			if node >= 0 {
				anchors = append(anchors, s.input[node])
			}
		}
	}
	return anchors
}

// anchorDistance is the minimal center-to-center distance from the path's
// first matched block to any anchor block.
func (s *Sorter) anchorDistance(anchors []Block, path Path) float64 {
	first := -1
	for _, node := range path.Nodes {
		if node >= 0 {
			first = node
			break
		}
	}
	if first < 0 || len(anchors) == 0 {
		return math.MaxFloat64
	}
	x, y := s.input[first].Center()

	best := math.MaxFloat64
	for _, a := range anchors {
		ax, ay := a.Center()
		if d := math.Sqrt((ax-x)*(ax-x) + (ay-y)*(ay-y)); d < best {
			best = d
		}
	}
	return best
}

// anchorRerank picks among near-tied candidate paths (sorted by length) the
// one closest to the target line's anchors. Falls back to the shortest path
// when no anchors exist yet.
func (s *Sorter) anchorRerank(target *Line, paths []Path) Path {
	if len(paths) == 1 {
		return paths[0]
	}
	anchors := s.anchorBlocks(target)
	if len(anchors) == 0 {
		return paths[0]
	}

	bestIdx := 0
	bestScore := s.anchorDistance(anchors, paths[0])
	limit := paths[0].Length + s.config.AnchorTieEpsilon
	for i, p := range paths[1:] {
		if p.Length > limit {
			break // paths are sorted by length; the rest are out of contention
		}
		if score := s.anchorDistance(anchors, p); score < bestScore {
			bestIdx, bestScore = i+1, score
		}
	}

	if bestIdx != 0 {
		s.metrics.ShortLinesAnchored++
		s.logger.Debugf("anchor rerank: '%s' moved to instance at anchor distance %.3f", target.OriginalText, bestScore)
	}
	return paths[bestIdx]
}
