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
