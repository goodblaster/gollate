package multipage

import (
	"fmt"
	"reflect"
	"testing"
)

func TestLongFragmentsIgnoreMisleadingUniquePhrases(t *testing.T) {
	var line []string
	for i := 0; i < 42; i++ {
		line = append(line, fmt.Sprintf("term%d", i))
	}
	pages := [][]string{
		append([]string(nil), line[:14]...),
		append([]string(nil), line[14:28]...),
		append([]string(nil), line[28:]...),
		append([]string(nil), line[4:7]...),   // quoted elsewhere; original occurrence is misread
		append([]string(nil), line[39:42]...), // isolated false tail anchor
	}
	pages[0][5], pages[2][12] = "misread", "damaged"
	want := []Fragment{{Page: 0, Start: 0, End: 14}, {Page: 1, Start: 14, End: 28}, {Page: 2, Start: 28, End: 42}}
	got := longFragments(line, pages, indexGrams(pages))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	assignment := Segment([][]string{line}, pages)[0]
	if !reflect.DeepEqual(assignment.Fragments, want) {
		t.Fatalf("segment discarded valid pages: %+v", assignment)
	}
}

func TestLongFragmentsNeedsCorroborationOnEachPage(t *testing.T) {
	line := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	pages := [][]string{{"a", "b", "c"}, {"unrelated"}, {"d", "e", "f"}, {"unrelated"}, {"g", "h", "i"}}
	if got := longFragments(line, pages, indexGrams(pages)); got != nil {
		t.Fatalf("isolated phrases invented fragments: %+v", got)
	}
}
