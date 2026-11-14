package language

import (
	"testing"
)

func TestEnglishDetection(t *testing.T) {
	handler := &English{}

	tests := []struct {
		name     string
		text     string
		expected float64 // Minimum expected confidence
	}{
		{"pure english", "Hello World", 0.9},
		{"english with numbers", "iPhone 13 Pro", 0.9},
		{"english sentence", "Learn more about our products.", 0.9},
		{"empty", "", 0.0},
		{"chinese text", "維基百科", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := handler.DetectScript(tt.text)
			if conf < tt.expected {
				t.Errorf("DetectScript(%q) = %.2f, want >= %.2f", tt.text, conf, tt.expected)
			}
		})
	}
}

func TestCJKDetection(t *testing.T) {
	handler := &CJK{}

	tests := []struct {
		name     string
		text     string
		expected float64
	}{
		{"pure chinese", "維基百科", 0.9},
		{"chinese sentence", "1857年英國與法國藉口", 0.5}, // Has numbers
		{"empty", "", 0.0},
		{"english text", "Hello World", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := handler.DetectScript(tt.text)
			if conf < tt.expected {
				t.Errorf("DetectScript(%q) = %.2f, want >= %.2f", tt.text, conf, tt.expected)
			}
		})
	}
}

func TestSpacingRules(t *testing.T) {
	tests := []struct {
		name    string
		handler Handler
		current string
		next    string
		want    bool
	}{
		// English - always needs space
		{"english words", &English{}, "Hello", "World", true},
		{"english numbers", &English{}, "iPhone", "13", true},

		// CJK - no space between CJK chars
		{"cjk chars", &CJK{}, "維", "基", false},
		{"cjk words", &CJK{}, "維基", "百科", false},

		// Mixed - space when transitioning
		{"cjk to number", &CJK{}, "年", "1857", true},
		{"number to cjk", &CJK{}, "1857", "年", true},

		// Mixed content handler
		{"mixed cjk-cjk", &MixedContent{}, "維基", "百科", false},
		{"mixed cjk-eng", &MixedContent{}, "維基", "Wiki", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.handler.NeedsSpaceBetween(tt.current, tt.next)
			if got != tt.want {
				t.Errorf("NeedsSpaceBetween(%q, %q) = %v, want %v",
					tt.current, tt.next, got, tt.want)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantName string
	}{
		{"english document", "Hello World. This is a test.", "English"},
		{"chinese document", "維基百科是一個多語言內容自由", "CJK"},
		{"mixed content", "Hello 世界", "MixedContent"},
		{"empty", "", "English"}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Detect(tt.text)
			if handler.Name() != tt.wantName {
				t.Errorf("Detect(%q).Name() = %v, want %v",
					tt.text, handler.Name(), tt.wantName)
			}
		})
	}
}

func TestReadingOrder(t *testing.T) {
	tests := []struct {
		name    string
		handler Handler
		want    ReadingOrder
	}{
		{
			name:    "english horizontal",
			handler: &English{},
			want: ReadingOrder{
				Primary:       Horizontal,
				HorizontalDir: LeftToRight,
				Secondary:     Vertical,
				VerticalDir:   TopToBottom,
			},
		},
		{
			name:    "cjk horizontal",
			handler: &CJK{},
			want: ReadingOrder{
				Primary:       Horizontal,
				HorizontalDir: LeftToRight,
				Secondary:     Vertical,
				VerticalDir:   TopToBottom,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.handler.ReadingOrder()
			if got.Primary != tt.want.Primary ||
				got.HorizontalDir != tt.want.HorizontalDir ||
				got.Secondary != tt.want.Secondary ||
				got.VerticalDir != tt.want.VerticalDir {
				t.Errorf("ReadingOrder() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestOCRSettings(t *testing.T) {
	tests := []struct {
		name              string
		handler           Handler
		wantRecognition   string
		wantRequiresSplit bool
	}{
		{"english", &English{}, "fast", false},
		{"cjk", &CJK{}, "accurate", true},
		{"mixed", &MixedContent{}, "accurate", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := tt.handler.OCRSettings()

			if settings.RecognitionLevel != tt.wantRecognition {
				t.Errorf("RecognitionLevel = %v, want %v",
					settings.RecognitionLevel, tt.wantRecognition)
			}

			if settings.RequiresCharSplit != tt.wantRequiresSplit {
				t.Errorf("RequiresCharSplit = %v, want %v",
					settings.RequiresCharSplit, tt.wantRequiresSplit)
			}

			if len(settings.LanguageCodes) == 0 {
				t.Error("LanguageCodes should not be empty")
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name    string
		handler Handler
		text    string
		want    []string
	}{
		// English tokenization (space-separated)
		{"english simple", &English{}, "Hello World", []string{"Hello", "World"}},
		{"english sentence", &English{}, "The quick brown fox", []string{"The", "quick", "brown", "fox"}},
		{"english empty", &English{}, "", nil},
		{"english multiple spaces", &English{}, "Hello  World", []string{"Hello", "World"}},

		// CJK tokenization (character-level for OCR matching)
		{"cjk no spaces", &CJK{}, "維基百科", []string{"維", "基", "百", "科"}},
		{"cjk with spaces", &CJK{}, "維基 百科", []string{"維", "基", "百", "科"}},
		{"cjk empty", &CJK{}, "", nil},

		// Mixed content tokenization (whitespace-separated)
		// Note: MixedContent still uses whitespace splitting, unlike pure CJK
		{"mixed no spaces", &MixedContent{}, "Hello世界", []string{"Hello世界"}},
		{"mixed with spaces", &MixedContent{}, "Hello 世界", []string{"Hello", "世界"}},
		{"mixed english only", &MixedContent{}, "Hello World", []string{"Hello", "World"}},
		{"mixed cjk only", &MixedContent{}, "維基百科", []string{"維基百科"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.handler.Tokenize(tt.text)
			if len(got) != len(tt.want) {
				t.Errorf("Tokenize(%q) returned %d tokens, want %d\nGot: %v\nWant: %v",
					tt.text, len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Tokenize(%q)[%d] = %q, want %q",
						tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// Ordinary CJK prose is full of Arabic numerals, fullwidth punctuation and
// the odd Latin term. None of that may tip it out of per-character
// tokenization.
func TestDetectIgnoresDigitsAndPunctuation(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"japanese with numbers", "2023年の調査によると、通常の週末には約12,000人が訪れ、38%多い（午前9時15分）。", "CJK"},
		{"chinese with numbers", "根据市政府2023年的调查，市场在普通周末能吸引约12000名顾客，高出38%。", "CJK"},
		{"spanish with accents", "El médico español trabaja en el hospital público todos los días.", "English"},
		{"english with numbers", "Fees rose from $35 to $50 on 03/14/2024 (a 42.8% jump).", "English"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.text).Name(); got != tt.want {
				t.Errorf("Detect = %s, want %s", got, tt.want)
			}
		})
	}
}

// A stated language outranks script statistics.
func TestForLanguage(t *testing.T) {
	mostlyLatin := "iPhone 15 Pro Max の新しい USB-C ポート"
	if got := Detect(mostlyLatin).Name(); got == "CJK" {
		t.Fatalf("test premise broken: detection alone picked CJK")
	}
	if got := ForLanguage("japanese", mostlyLatin).Name(); got != "CJK" {
		t.Errorf("ForLanguage(japanese) = %s, want CJK", got)
	}
	if got := ForLanguage("", "Hello world").Name(); got != "English" {
		t.Errorf("ForLanguage(\"\") = %s, want English (detected)", got)
	}
	if got := ForLanguage("english", "維基百科是一個多語言").Name(); got != "CJK" {
		t.Errorf("ForLanguage(english) on Chinese text = %s, want CJK (detected)", got)
	}
}
