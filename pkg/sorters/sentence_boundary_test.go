package sorters

import (
	"reflect"
	"testing"
)

func TestEndsWithSentencePunctuationCJK(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"これは文です。", true},
		{"本当？", true},
		{"すごい！", true},
		{"「そうですね。」", false}, // 」 is not in the trailing-quote set; unchanged behaviour
		{"これは文です", false},
		{"東京、", false},
		{"École", false},
		{"Fin.", true},
		{"U.S.", false},
	}
	for _, tt := range tests {
		if got := endsWithSentencePunctuation(tt.text); got != tt.want {
			t.Errorf("endsWithSentencePunctuation(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestSplitParagraphCJK(t *testing.T) {
	got := SplitParagraph("これは最初の文です。これは二番目の文です。")
	want := []string{"これは最初の文です。", "これは二番目の文です。"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// The split nearest the middle wins.
	got = SplitParagraph("一。二二二二二二。三。")
	want = []string{"一。二二二二二二。", "三。"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSplitParagraphKeepsDecimalsIntact(t *testing.T) {
	// " .5" is a number, not detached terminal punctuation.
	got := SplitParagraph("Rates rose .5 percent today. Markets fell.")
	want := []string{"Rates rose .5 percent today.", "Markets fell."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// Genuinely detached terminal punctuation is still closed up, and the
	// space after it survives.
	got = SplitParagraph("First part ends here . Second part . Third one here.")
	want = []string{"First part ends here.", "Second part. Third one here."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// When OCR runs two canonical lines together, the leftover sentence is cut
// at the canonical boundary - measured in space-stripped characters, like
// every other position in that code.
func TestSplitLeftoversCutsAtCanonicalBoundary(t *testing.T) {
	canonical := []string{"a b c d e f g h", "iiii jjjj kkkk"}
	var sentence []Block
	for i, text := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "iiii", "jjjj", "kkkk"} {
		sentence = append(sentence, Block{Text: text, NormedText: text, Extractor: "test", Index: i})
	}
	sorter := NewOcrSorterWithConfig(nil, canonical, nil, DefaultConfig())

	var got [][]string
	for _, line := range sorter.splitLeftoversByCanonicalLines([][]Block{sentence}) {
		var words []string
		for _, b := range line {
			words = append(words, b.Text)
		}
		got = append(got, words)
	}
	want := [][]string{{"a", "b", "c", "d", "e", "f", "g", "h"}, {"iiii", "jjjj", "kkkk"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Arabic-script and Devanagari punctuation is punctuation: it must not
// stay glued to the word, and it ends sentences.
func TestArabicAndDevanagariPunctuation(t *testing.T) {
	for in, want := range map[string]string{
		"سبت،":    "سبت",
		"سبت؛":    "سبت",
		"لماذا؟":  "لماذا",
		"है।":     "ह",     // the vowel sign is a combining mark, stripped like an accent
		"समाप्त॥": "समापत", // nukta/virama are combining marks, stripped like accents
		"tow‐er":  "tow er",
	} {
		if got := NormalizeText(in); got != want {
			t.Errorf("NormalizeText(%q) = %q, want %q", in, got, want)
		}
	}
	for _, text := range []string{"यह एक वाक्य है।", "هل هذا سؤال؟"} {
		if !endsWithSentencePunctuation(text) {
			t.Errorf("endsWithSentencePunctuation(%q) = false", text)
		}
	}
	got := SplitParagraph("यह पहला वाक्य है। यह दूसरा वाक्य है।")
	want := []string{"यह पहला वाक्य है।", "यह दूसरा वाक्य है।"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
