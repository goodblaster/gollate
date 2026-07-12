package sorters

import "strings"

// Feature flag for sentence splitting during line parsing (currently disabled)
const _SplitSentences = false

type Line struct {
	//SplitIndex   int // If the original lines gets split into individual sentences, this is the index of the split sentence.
	OriginalLine int
	OriginalText string
	Normalized   string
	FoundPath    Path
	Found        bool
	Split        bool // Has been split into multiple lines, and should now be ignored. Retained so we don't mess up indexes.
	IsBlank      bool // This line is a blank line from canonical text (paragraph separator)
}

func ParseLines(text []string) []Line {
	var lines []Line
	//next := NextIndex{}
	for i, line := range text {
		if _SplitSentences {
			sentences := strings.Split(line, ". ")
			for _, sentence := range sentences {
				if strings.TrimSpace(line) == "" {
					continue
				}

				lines = append(lines, Line{
					OriginalLine: i,
					OriginalText: sentence,
					Normalized:   NormalizeText(sentence),
				})
			}
		} else {
			// Keep blank lines to preserve paragraph structure
			// Note: blank lines are marked IsBlank but NOT Found initially
			// They will be marked as Found later if surrounding content is found
			if strings.TrimSpace(line) == "" {
				lines = append(lines, Line{
					OriginalLine: i,
					OriginalText: "",
					Normalized:   "",
					IsBlank:      true,
					Found:        false, // Will be set to true later if needed
				})
				continue
			}

			lines = append(lines, Line{
				OriginalLine: i,
				OriginalText: line,
				Normalized:   NormalizeText(line),
			})
		}
	}
	return lines
}
