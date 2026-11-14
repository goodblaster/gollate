package multipage

import (
	"strings"
	"testing"
)

func toks(s string) []string { return strings.Fields(s) }

func tokLists(ss ...string) [][]string {
	out := make([][]string, len(ss))
	for i, s := range ss {
		out[i] = toks(s)
	}
	return out
}

func pagesOf(a []Assignment) []int {
	out := make([]int, len(a))
	for i, x := range a {
		out[i] = x.Page
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSegmentCleanSplit(t *testing.T) {
	lines := tokLists(
		"alpha bravo charlie delta echo",
		"",
		"foxtrot golf hotel india juliet",
		"kilo lima mike november oscar",
		"papa quebec romeo sierra tango",
	)
	pages := tokLists(
		"alpha bravo charlie delta echo foxtrot golf hotel india juliet",
		"kilo lima mike november oscar papa quebec romeo sierra tango",
	)
	got := Segment(lines, pages)
	if want := []int{0, 0, 0, 1, 1}; !equalInts(pagesOf(got), want) {
		t.Fatalf("pages = %v, want %v", pagesOf(got), want)
	}
	for i, a := range got {
		if a.Straddle {
			t.Errorf("line %d wrongly marked as straddling", i)
		}
	}
}

func TestSegmentStraddle(t *testing.T) {
	lines := tokLists(
		"alpha bravo charlie delta echo",
		"foxtrot golf hotel india juliet kilo lima mike", // breaks after "india"
		"november oscar papa quebec romeo",
	)
	pages := tokLists(
		"alpha bravo charlie delta echo foxtrot golf hotel india",
		"juliet kilo lima mike november oscar papa quebec romeo",
	)
	got := Segment(lines, pages)
	if !got[1].Straddle || got[1].Page != 0 || got[1].Cut != 4 {
		t.Fatalf("line 1 = %+v, want straddle from page 0 with cut 4", got[1])
	}
	if got[0].Page != 0 || got[2].Page != 1 || got[0].Straddle || got[2].Straddle {
		t.Errorf("neighbors misplaced: %+v", got)
	}
}

// A line repeated on every page cannot be told apart by its words; the
// k-th copy belongs with the k-th page's other lines.
func TestSegmentRepeatedLineFollowsNeighbors(t *testing.T) {
	lines := tokLists(
		"alpha bravo charlie delta echo",
		"learn more",
		"foxtrot golf hotel india juliet",
		"learn more",
		"kilo lima mike november oscar",
		"learn more",
	)
	pages := tokLists(
		"alpha bravo charlie delta echo learn more",
		"foxtrot golf hotel india juliet learn more",
		"kilo lima mike november oscar learn more",
	)
	if got, want := pagesOf(Segment(lines, pages)), []int{0, 0, 1, 1, 2, 2}; !equalInts(got, want) {
		t.Fatalf("pages = %v, want %v", got, want)
	}
}

// Lines found on no page (edition drift: canonical has text the scan lacks)
// must not drag their neighbors off the right page.
func TestSegmentMissingRunDoesNotSmear(t *testing.T) {
	lines := tokLists(
		"alpha bravo charlie delta echo",
		"zzz1 zzz2 zzz3 zzz4 zzz5",
		"zzz6 zzz7 zzz8 zzz9 zzz10",
		"zzz11 zzz12 zzz13 zzz14 zzz15",
		"foxtrot golf hotel india juliet",
		"kilo lima mike november oscar",
	)
	pages := tokLists(
		"alpha bravo charlie delta echo foxtrot golf hotel india juliet",
		"kilo lima mike november oscar",
	)
	got := Segment(lines, pages)
	if got[0].Page != 0 || got[4].Page != 0 || got[5].Page != 1 {
		t.Fatalf("found lines misplaced: %v", pagesOf(got))
	}
	for i := 1; i <= 3; i++ {
		if got[i].Coverage != 0 {
			t.Errorf("line %d coverage = %v, want 0", i, got[i].Coverage)
		}
	}
}

// A page that OCR read badly scores low against everything. Its lines
// must still land on it, by position, rather than on its neighbors.
func TestSegmentLowCoveragePage(t *testing.T) {
	lines := tokLists(
		"alpha bravo charlie delta echo",
		"foxtrot golf hotel india juliet",
		"kilo lima mike november oscar",
		"papa quebec romeo sierra tango",
		"uniform victor whiskey xray yankee",
		"zulu able baker cast dog",
	)
	pages := tokLists(
		"alpha bravo charlie delta echo foxtrot golf hotel india juliet",
		"kiio 1ima mlke novernber 0scar papa qvebec rorneo slerra tang0", // garbage, one hit
		"uniform victor whiskey xray yankee zulu able baker cast dog",
	)
	if got, want := pagesOf(Segment(lines, pages)), []int{0, 0, 1, 1, 2, 2}; !equalInts(got, want) {
		t.Fatalf("pages = %v, want %v", got, want)
	}
}

func TestSegmentDegenerate(t *testing.T) {
	if got := Segment(tokLists("a b"), nil); got[0].Page != -1 {
		t.Errorf("no pages: got %+v", got)
	}
	if got := Segment(nil, tokLists("a b")); len(got) != 0 {
		t.Errorf("no lines: got %+v", got)
	}
	// More pages than text: everything on the page that has it.
	got := Segment(tokLists("alpha bravo charlie delta echo"), tokLists("", "alpha bravo charlie delta echo", ""))
	if got[0].Page != 1 {
		t.Errorf("got page %d, want 1", got[0].Page)
	}
}
