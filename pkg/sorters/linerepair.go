package sorters

import (
	"sort"

	"github.com/goodblaster/gollate/pkg/language"
)

// Line repair (preparation step, on by default, DisableLineRepair to turn
// off). When blocks carry the OCR engine's own line grouping (LineId),
// misread words can be identified by position instead of text similarity:
// the engine asserts "these tokens are one visual line, in this order", so
// an unrecognizable token flanked by exactly-matched neighbors is the
// canonical word between those neighbors with near-certainty.
//
// For each token whose normalized text exists nowhere in the canonical
// vocabulary (an "alien" - by construction, correctly-read words can never
// be touched), look at its engine-line neighbors. If the flanking tokens
// match canonical words exactly, search the canonical lines for
// "prev ? next" with exactly one word between; if every occurrence agrees
// on the same middle word, rekey the alien to it. Tokens at a line edge use
// a one-sided trigram instead. Any ambiguity means no repair - later
// mechanisms (holes, reconciliation) get their chance.
//
// Repairs change only NormedText, the matching key: Block.Text always
// remains what OCR read (the match-only invariant), and every repair is
// recorded in the block's correction metadata. This self-guards against
// engine lines that wrongly span columns: their neighbors are never
// canonically adjacent, so the flanking pattern does not exist and nothing
// fires.
func repairLines(blocks []Block, lines []Line, handler language.Handler) int {
	// Canonical vocabulary and token sequences.
	vocab := make(map[string]bool)
	var sequences [][]string
	for _, line := range lines {
		tokens := handler.Tokenize(line.Normalized)
		if len(tokens) == 0 {
			continue
		}
		sequences = append(sequences, tokens)
		for _, tok := range tokens {
			vocab[tok] = true
		}
	}
	if len(vocab) == 0 {
		return 0
	}

	// Group block positions by engine line, in emit order.
	groups := make(map[string][]int)
	var lineIds []string
	for i, b := range blocks {
		if b.LineId == "" || b.NormedText == "" {
			continue
		}
		if _, seen := groups[b.LineId]; !seen {
			lineIds = append(lineIds, b.LineId)
		}
		groups[b.LineId] = append(groups[b.LineId], i)
	}
	sort.Strings(lineIds)

	repaired := 0
	for _, id := range lineIds {
		group := groups[id]
		for pos, bi := range group {
			blk := &blocks[bi]
			if vocab[blk.NormedText] {
				continue // known word - never touched
			}

			word, ok := "", false
			switch {
			case pos > 0 && pos < len(group)-1:
				prev, next := blocks[group[pos-1]].NormedText, blocks[group[pos+1]].NormedText
				if vocab[prev] && vocab[next] {
					word, ok = uniqueMiddle(sequences, prev, next)
				}
			case pos == 0 && len(group) >= 3:
				n1, n2 := blocks[group[1]].NormedText, blocks[group[2]].NormedText
				if vocab[n1] && vocab[n2] {
					word, ok = uniqueEdge(sequences, n1, n2, false)
				}
			case pos == len(group)-1 && len(group) >= 3:
				p2, p1 := blocks[group[pos-2]].NormedText, blocks[group[pos-1]].NormedText
				if vocab[p2] && vocab[p1] {
					word, ok = uniqueEdge(sequences, p2, p1, true)
				}
			}
			if !ok {
				continue
			}

			blk.OriginalOcrText = blk.Text
			blk.SuggestedText = word
			blk.CorrectionType = "line-anchored"
			blk.EditDistance = levenshteinDistanceRunes([]rune(blk.NormedText), []rune(word))
			blk.NormedText = word
			repaired++
		}
	}
	return repaired
}

// uniqueMiddle finds the single word appearing between prev and next across
// all canonical occurrences of "prev ? next". Ambiguity (or absence) means
// no repair.
func uniqueMiddle(sequences [][]string, prev, next string) (string, bool) {
	word := ""
	for _, seq := range sequences {
		for j := 0; j+2 < len(seq); j++ {
			if seq[j] == prev && seq[j+2] == next {
				if word != "" && word != seq[j+1] {
					return "", false
				}
				word = seq[j+1]
			}
		}
	}
	return word, word != ""
}

// uniqueEdge finds the single word adjacent to an exactly-matched pair at a
// line edge: "? a b" (atEnd=false) or "a b ?" (atEnd=true).
func uniqueEdge(sequences [][]string, a, b string, atEnd bool) (string, bool) {
	word := ""
	for _, seq := range sequences {
		for j := 0; j+2 < len(seq); j++ {
			var candidate string
			if atEnd {
				if seq[j] != a || seq[j+1] != b {
					continue
				}
				candidate = seq[j+2]
			} else {
				if seq[j+1] != a || seq[j+2] != b {
					continue
				}
				candidate = seq[j]
			}
			if word != "" && word != candidate {
				return "", false
			}
			word = candidate
		}
	}
	return word, word != ""
}
