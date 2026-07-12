package sorters

import (
	"regexp"
	"strings"

	"github.com/goodblaster/gollate/pkg/language"
)

// convertBlocksToLines converts a sequence of blocks into lines of text.
//
// Blocks are grouped into lines using empty blocks as line separators.
// The language handler determines spacing between words (e.g., CJK languages don't use spaces).
//
// Empty lines from the canonical text are preserved to maintain paragraph structure.
func convertBlocksToLines(blocks []Block, handler language.Handler) []string {
	var lines []string
	var line strings.Builder
	for i, block := range blocks {
		line.WriteString(block.Text)

		// Only add space if the next block exists and is not a line separator
		if i < len(blocks)-1 && blocks[i+1].Engine() != "" {
			// Delegate spacing decision to language handler
			if handler.NeedsSpaceBetween(block.Text, blocks[i+1].Text) {
				line.WriteByte(' ')
			}
		}

		if block.Engine() == "" || i == len(blocks)-1 {
			trimmed := strings.TrimSpace(line.String())
			// Always append lines, including empty ones (blank canonical lines)
			// This preserves paragraph structure from the canonical text
			lines = append(lines, trimmed)
			line.Reset()
		}
	}
	return lines
}

// endsWithSentencePunctuation checks if text ends with sentence-ending punctuation.
//
// Supports both Western and CJK (Chinese/Japanese/Korean) punctuation.
// Excludes common abbreviations and initialisms like "U.S.", "Dr.", "etc."
//
// This is used when splitting text into sentences during post-processing.
func endsWithSentencePunctuation(text string) bool {
	text = strings.TrimSpace(text)
	// A closing quote or bracket after the terminal punctuation still ends a
	// sentence ("...an 'infant.'"). Ignore any trailing ones before checking.
	text = strings.TrimRight(text, "'\""+"‘’“”"+")]}»")
	if len(text) == 0 {
		return false
	}

	// Check last rune for sentence-ending punctuation
	lastRune := rune(text[len(text)-1])

	// CJK sentence-ending punctuation (always sentence-ending, no abbreviations in CJK)
	if lastRune == '\u3002' || // 。 Ideographic full stop
		lastRune == '\uFF01' || // ！ Fullwidth exclamation mark
		lastRune == '\uFF1F' { // ？ Fullwidth question mark
		return true
	}

	// Western exclamation mark or question mark (always sentence-ending)
	if strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") {
		return true
	}

	// Check if it ends with Western period
	if !strings.HasSuffix(text, ".") {
		return false
	}

	// Exclude common initialisms/abbreviations
	// Pattern: One or more capital letters followed by periods (e.g., "U.S.", "Ph.D.", "A.M.")
	initialismPattern := regexp.MustCompile(`\b[A-Z]\.([A-Z]\.)*$`)
	if initialismPattern.MatchString(text) {
		return false
	}

	// Exclude common abbreviations
	commonAbbrevs := []string{"Dr.", "Mr.", "Mrs.", "Ms.", "Jr.", "Sr.", "Inc.", "Ltd.", "Corp.",
		"etc.", "i.e.", "e.g.", "vs.", "Prof.", "Rev.", "Gen.", "Col.", "Sgt.", "Lt."}
	for _, abbrev := range commonAbbrevs {
		if strings.HasSuffix(text, abbrev) {
			return false
		}
	}

	// It ends with period and is not an abbreviation, so it's sentence-ending
	return true
}
