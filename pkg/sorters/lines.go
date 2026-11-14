package sorters

import "strings"

type Line struct {
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
	for i, line := range text {
		// Keep blank lines to preserve paragraph structure. They are marked
		// IsBlank but not Found; markIntermediateBlankLines marks them Found
		// later if surrounding content is found.
		if strings.TrimSpace(line) == "" {
			lines = append(lines, Line{OriginalLine: i, IsBlank: true})
			continue
		}

		lines = append(lines, Line{
			OriginalLine: i,
			OriginalText: line,
			Normalized:   NormalizeText(line),
		})
	}
	return lines
}
