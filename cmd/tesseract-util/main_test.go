package main

import "testing"

const tsvHeader = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n"

func TestParseTesseractTSVQuotesAreLiteral(t *testing.T) {
	// A word starting with a double quote must not swallow the rest of the
	// page (regression: encoding/csv treated it as a quoted field).
	tsv := tsvHeader +
		"5\t1\t1\t1\t1\t1\t10\t10\t50\t20\t96.5\tHello\n" +
		"5\t1\t1\t1\t1\t2\t70\t10\t50\t20\t95.0\t\"People\n" +
		"5\t1\t1\t1\t1\t3\t130\t10\t50\t20\t94.0\tthink\"\n" +
		"5\t1\t1\t1\t2\t1\t10\t40\t50\t20\t93.0\tafter\n"
	words, err := parseTesseractTSV(tsv)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 4 {
		t.Fatalf("got %d words, want 4: %+v", len(words), words)
	}
	if words[1].Text != `"People` || words[2].Text != `think"` {
		t.Errorf("quotes not preserved: %q %q", words[1].Text, words[2].Text)
	}
	if words[3].Left != 10 || words[3].Top != 40 || words[3].Confidence != 93.0 {
		t.Errorf("geometry/conf wrong after quoted word: %+v", words[3])
	}
}

func TestParseTesseractTSVLineNumsArePageUnique(t *testing.T) {
	// line_num restarts in every paragraph and block; the emitted LineNum
	// must still distinguish every visual line.
	tsv := tsvHeader +
		"4\t1\t1\t1\t1\t0\t0\t0\t0\t0\t-1\t\n" +
		"5\t1\t1\t1\t1\t1\t10\t10\t50\t20\t96\ta\n" +
		"5\t1\t1\t1\t1\t2\t70\t10\t50\t20\t96\tb\n" +
		"5\t1\t1\t1\t2\t1\t10\t40\t50\t20\t96\tc\n" +
		"5\t1\t1\t2\t1\t1\t10\t90\t50\t20\t96\td\n" + // new paragraph, line_num back to 1
		"5\t1\t2\t1\t1\t1\t10\t200\t50\t20\t96\te\n" // new block, line_num back to 1
	words, err := parseTesseractTSV(tsv)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int, len(words))
	for i, w := range words {
		got[i] = w.LineNum
	}
	want := []int{0, 0, 1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line nums %v, want %v", got, want)
		}
	}
}

func TestParseTesseractTSVRejectsGarbage(t *testing.T) {
	if _, err := parseTesseractTSV(""); err == nil {
		t.Error("empty input should error")
	}
	if _, err := parseTesseractTSV("foo\tbar\n1\t2\n"); err == nil {
		t.Error("missing columns should error")
	}
}
