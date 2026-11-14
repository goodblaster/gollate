package sorters

import (
	"unicode"

	"github.com/goodblaster/gollate/pkg/language"
)

// Canonical spans tie output tokens back to the canonical text: for a
// token in a paragraph matched to canonical line L, Token.Canonical is the
// byte range of that line the token corresponds to. This is the link
// passage highlighting needs - mark a sentence in the canonical text, look
// up the tokens whose spans it covers, draw their boxes.
//
// The link is recovered at output time rather than carried through
// matching, because most output does not come from one mechanism: a
// paragraph may be assembled from pathfinding pieces, placed leftovers and
// repaired or re-joined blocks. Whatever produced it, its blocks' matching
// keys are aligned (longest common subsequence) against the line's tokens.

// canonicalToken is one matching token of a canonical line and where it
// sits in the line's original text.
type canonicalToken struct {
	token string
	span  Span
}

// maxAlignCells bounds the alignment table; larger paragraphs go without
// canonical spans rather than allocate without limit.
const maxAlignCells = 16_000_000

// canonicalTokenSpans tokenizes text exactly as matching does and records
// each token's byte range in the original. Returns nil if the ranges
// cannot be established (the per-word tokens do not reproduce the line's).
func canonicalTokenSpans(text string, handler language.Handler) []canonicalToken {
	var out []canonicalToken
	wordStart := -1
	flush := func(end int) {
		if wordStart < 0 {
			return
		}
		word := text[wordStart:end]
		base := wordStart
		wordStart = -1

		tokens := handler.Tokenize(NormalizeText(word))
		switch {
		case len(tokens) == 0:
			return
		case len(tokens) == 1:
			out = append(out, canonicalToken{tokens[0], Span{base, end}})
			return
		}

		// Per-character tokens (CJK): each is the next character that
		// normalizes to it.
		perRune := true
		for _, t := range tokens {
			if len([]rune(t)) != 1 {
				perRune = false
				break
			}
		}
		if perRune {
			next := 0
			for i, r := range word {
				if next == len(tokens) {
					break
				}
				size := len(string(r))
				if NormalizeText(string(r)) == tokens[next] {
					out = append(out, canonicalToken{tokens[next], Span{base + i, base + i + size}})
					next++
				}
			}
			for ; next < len(tokens); next++ { // unlocated: fall back to the word
				out = append(out, canonicalToken{tokens[next], Span{base, end}})
			}
			return
		}

		// A compound word ("four-block", "$4.2"): one range per part.
		if parts := splitCompoundText(word, tokens, nil); parts != nil {
			offset := base
			for _, part := range parts {
				out = append(out, canonicalToken{part.normalized, Span{offset, offset + len(part.text)}})
				offset += len(part.text) + len(part.separator)
			}
			return
		}
		for _, t := range tokens {
			out = append(out, canonicalToken{t, Span{base, end}})
		}
	}
	for i, r := range text {
		if unicode.IsSpace(r) {
			flush(i)
		} else if wordStart < 0 {
			wordStart = i
		}
	}
	flush(len(text))

	// The ranges are only trustworthy if tokenizing word by word gives the
	// same tokens as tokenizing the line.
	whole := handler.Tokenize(NormalizeText(text))
	if len(whole) != len(out) {
		return nil
	}
	for i := range whole {
		if whole[i] != out[i].token {
			return nil
		}
	}
	return out
}

// alignToCanonical aligns each block's matching tokens to the canonical
// tokens and returns, per block, the canonical byte range its tokens
// landed on (nil where none aligned).
func alignToCanonical(blockTokens [][]string, canonical []canonicalToken) []*Span {
	spans := make([]*Span, len(blockTokens))
	var flat []string
	var owner []int
	for b, tokens := range blockTokens {
		for _, t := range tokens {
			flat = append(flat, t)
			owner = append(owner, b)
		}
	}
	n, m := len(flat), len(canonical)
	if n == 0 || m == 0 || (n+1)*(m+1) > maxAlignCells {
		return spans
	}

	// lcs[i][j]: length of the LCS of flat[i:] and canonical[j:].
	width := m + 1
	lcs := make([]int32, (n+1)*width)
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if flat[i] == canonical[j].token {
				lcs[i*width+j] = lcs[(i+1)*width+j+1] + 1
			} else {
				lcs[i*width+j] = max(lcs[(i+1)*width+j], lcs[i*width+j+1])
			}
		}
	}
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case flat[i] == canonical[j].token:
			b, span := owner[i], canonical[j].span
			if spans[b] == nil {
				spans[b] = &Span{span.Start, span.End}
			} else {
				spans[b].Start = min(spans[b].Start, span.Start)
				spans[b].End = max(spans[b].End, span.End)
			}
			i++
			j++
		case lcs[(i+1)*width+j] >= lcs[i*width+j+1]:
			i++
		default:
			j++
		}
	}
	return spans
}
