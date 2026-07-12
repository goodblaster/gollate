package sorters

import (
	"strings"
)

// LineServer - This serves one line at a time from a list.
// After trying to use a line, we can optionally split the line
// into multiple pieces and reinsert into the list of lines.
// This is rather specialized. We split on sentences, and then
// sort the remaining lines by length. Good enough for now.
type LineServer struct {
	lines []Line
	index int
}

func NewLineServer(lines []Line) *LineServer {
	return &LineServer{
		lines: lines,
	}
}

func (svr *LineServer) Next(line **Line) bool {
	if svr.index >= len(svr.lines) {
		return false
	}
	*line = &svr.lines[svr.index]
	svr.index++
	return true
}

func (svr *LineServer) List() []Line {
	return svr.lines
}

// SplitAndReset - Split the line and reset the index.
// If splitting doesn't yield multiple results, don't reset the index.
func (svr *LineServer) SplitAndReset(f func(line Line) []Line) bool {
	init := f(svr.lines[svr.index-1])

	var lines []Line
	for i, l := range init {
		init[i].Normalized = strings.TrimSpace(l.Normalized)
		if init[i].Normalized != "" {
			lines = append(lines, init[i])
		}
	}

	if len(lines) == 0 {
		return false
	}

	// No change?
	if len(lines) == 1 {
		if lines[0].Normalized == svr.lines[svr.index-1].Normalized {
			return false
		}
	}

	// Remove the current line.
	svr.lines[svr.index-1].Split = true

	// And add the new ones to the end.
	svr.lines = append(svr.lines, lines...)

	// Return true to signal we made a change.
	return true
}
