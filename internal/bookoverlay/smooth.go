package bookoverlay

import "math"

func sentenceID(mark Mark, m *Manifest) int {
	if len(mark.Words) == 0 {
		return 0
	}
	id := m.Words[mark.Words[0]-1].Sentence
	for _, n := range mark.Words[1:] {
		if m.Words[n-1].Sentence != id {
			return 0 // Do not smooth across an ambiguous sentence boundary.
		}
	}
	return id
}

// SmoothMarks connects adjacent words of the same sentence on the same visual
// line. Only interior whitespace is divided; the run's true outer edges stay
// fixed. A common vertical extent prevents jagged bands around short letters.
// The returned drawing copy never changes the source boxes in the manifest.
func SmoothMarks(marks []Mark, m *Manifest, width, height float64) []Mark {
	out := append([]Mark(nil), marks...)
	connects := func(a, b Mark) bool {
		if id := sentenceID(a, m); id == 0 || id != sentenceID(b, m) || b.OutputOrder-a.OutputOrder > 1 || b.OutputOrder < a.OutputOrder {
			return false
		}
		x, y := a.Bounds, b.Bounds
		gap := (y.Left - x.Left - x.Width) * width
		overlap := math.Min(x.Top+x.Height, y.Top+y.Height) - math.Max(x.Top, y.Top)
		return y.Left >= x.Left+x.Width && gap <= 1.25*math.Min(x.Height, y.Height)*height && overlap >= .5*math.Max(x.Height, y.Height)
	}
	for start := 0; start < len(marks); {
		end := start + 1
		top, bottom := marks[start].Bounds.Top, marks[start].Bounds.Top+marks[start].Bounds.Height
		for end < len(marks) && connects(marks[end-1], marks[end]) {
			top = math.Min(top, marks[end].Bounds.Top)
			bottom = math.Max(bottom, marks[end].Bounds.Top+marks[end].Bounds.Height)
			end++
		}
		if end == start+1 {
			start = end
			continue
		}
		for i := start; i < end; i++ {
			b := &out[i].Bounds
			b.Top, b.Height = top, bottom-top
			if i+1 < end {
				next := &out[i+1].Bounds
				mid := (b.Left + b.Width + next.Left) / 2
				b.Width = mid - b.Left
				next.Width += next.Left - mid
				next.Left = mid
			}
		}
		start = end
	}
	return out
}
