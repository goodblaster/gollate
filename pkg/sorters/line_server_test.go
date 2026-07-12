package sorters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLineServer_Simple(t *testing.T) {
	base := []string{
		"line 1",
		"line 2",
		"line 3",
	}

	var lines []Line
	for _, l := range base {
		lines = append(lines, Line{
			OriginalText: l,
			Normalized:   NormalizeText(l),
		})
	}

	server := NewLineServer(lines)

	var line *Line
	var index int
	for server.Next(&line) {
		assert.Equal(t, base[index], line.OriginalText)
		index++
	}
}

func TestLineServer_Split(t *testing.T) {
	base := []string{
		"sentence 1",
		"sentence 2. sentence 3.",
		"sentence 4. sentence 5. sentence 6. ",
		"sentence 7. sentence 8. sentence 9. sentence 10.",
	}

	var lines []Line
	for _, l := range base {
		lines = append(lines, Line{
			OriginalText: l,
			Normalized:   NormalizeText(l),
		})
	}

	server := NewLineServer(lines)

	var line *Line
	var finalLines []Line

	splitFunc := func(line Line) []Line {
		base := strings.Split(line.OriginalText, ". ")
		var lines []Line
		for _, l := range base {
			lines = append(lines, Line{
				OriginalLine: line.OriginalLine,
				OriginalText: l,
				Normalized:   NormalizeText(l),
			})
		}
		return lines
	}

	for server.Next(&line) {
		if server.SplitAndReset(splitFunc) {
			continue
		}
		finalLines = append(finalLines, *line)
	}

	assert.Len(t, finalLines, 10)

	// There's no guarantee that the lines will be in the same order as the input.
	// That's just how it works out in this case, so I'm calling it good.
	for i, l := range finalLines {
		assert.Equal(t, fmt.Sprintf("sentence %d", i+1), l.Normalized)
	}
}
