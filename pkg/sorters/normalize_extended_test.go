package sorters

import (
	"testing"
)

// TestNormalizeUnicodeEscapes tests handling of escaped Unicode sequences.
func TestNormalizeUnicodeEscapes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"right single quote", "don\\u2019t", "dont"}, // Typographic apostrophe joins, same as ASCII '
		{"left double quote", "\\u201CHello\\u201D", "hello"},
		{"ellipsis", "Wait\\u2026", "wait"},
		{"em dash", "Hello\\u2014world", "hello world"},
		{"mixed escapes", "\\u201CHello\\u2019\\u201D", "hello"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeEmailPreservation tests that email addresses are preserved.
func TestNormalizeEmailPreservation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple email", "contact@example.com", "contact@example.com"},
		{"email with text", "Email me at john.doe@company.org today", "email me at john.doe@company.org today"},
		{"email with underscores", "user_name@test.com", "user_name@test.com"},
		{"email with plus", "user+tag@example.com", "user+tag@example.com"},
		{"multiple emails", "john@a.com and jane@b.com", "john@a.com and jane@b.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizePhoneNumbers tests phone number preservation with underscore conversion.
func TestNormalizePhoneNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"US phone with plus", "+1-800-555-1234", "1_800_555_1234"},
		{"phone without plus", "800-555-1234", "800_555_1234"},
		{"international", "+44-20-7946-0958", "44_20_7946_0958"},
		{"phone in text", "Call +1-555-0123 now", "call 1_555_0123 now"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeInitialisms tests that initialisms with periods are preserved with underscores.
func TestNormalizeInitialisms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"USA", "U.S.A.", "u_s_a"},
		{"DC", "Washington D.C.", "washington d_c"},
		{"PhD", "Ph.D.", "ph_d"},
		{"am/pm", "9:00 a.m.", "9 00 a_m"},
		{"multiple initialisms", "U.S.A. and U.K.", "u_s_a and u_k"},
		{"initialism in sentence", "The U.S.A. is large", "the u_s_a is large"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeCJKPunctuation tests Chinese/Japanese/Korean punctuation handling.
func TestNormalizeCJKPunctuation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ideographic period", "你好。世界", "你好 世界"},
		{"fullwidth comma", "一、二、三", "一 二 三"},
		{"fullwidth exclamation", "すごい！", "すこい"}, // Note: NFD normalization affects dakuten marks
		{"fullwidth question", "何？", "何"},
		{"angle brackets", "《書名》", "書名"},
		{"corner brackets", "「引用」", "引用"},
		{"mixed CJK punctuation", "你好，世界！", "你好 世界"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeAllSpanishCharacters tests comprehensive Spanish character set.
func TestNormalizeAllSpanishCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"acute a", "á Á", "a a"},
		{"acute e", "é É", "e e"},
		{"acute i", "í Í", "i i"},
		{"acute o", "ó Ó", "o o"},
		{"acute u", "ú Ú", "u u"},
		{"eñe", "ñ Ñ", "n n"},
		{"umlaut u", "ü Ü", "u u"},
		{"inverted question", "¿Qué?", "que"},
		{"inverted exclamation", "¡Hola!", "hola"},
		{"all together", "¿Cómo están ustéd?", "como estan usted"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeAllFrenchCharacters tests comprehensive French character set.
func TestNormalizeAllFrenchCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"grave a", "à À", "a a"},
		{"grave e", "è È", "e e"},
		{"grave u", "ù Ù", "u u"},
		{"acute e", "é É", "e e"},
		{"circumflex a", "â Â", "a a"},
		{"circumflex e", "ê Ê", "e e"},
		{"circumflex i", "î Î", "i i"},
		{"circumflex o", "ô Ô", "o o"},
		{"circumflex u", "û Û", "u u"},
		{"cedilla", "ç Ç", "c c"},
		{"diaeresis e", "ë Ë", "e e"},
		{"diaeresis i", "ï Ï", "i i"},
		{"œ ligature", "œuf", "œuf"}, // Ligatures are preserved
		{"æ ligature", "æ", "æ"},     // Ligatures are preserved
		{"all together", "Être à l'hôtel", "etre a lhotel"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeAllPortugueseCharacters tests comprehensive Portuguese character set.
func TestNormalizeAllPortugueseCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde a", "ã Ã", "a a"},
		{"tilde o", "õ Õ", "o o"},
		{"acute a", "á Á", "a a"},
		{"acute e", "é É", "e e"},
		{"acute i", "í Í", "i i"},
		{"acute o", "ó Ó", "o o"},
		{"acute u", "ú Ú", "u u"},
		{"circumflex a", "â Â", "a a"},
		{"circumflex e", "ê Ê", "e e"},
		{"circumflex o", "ô Ô", "o o"},
		{"cedilla", "ç Ç", "c c"},
		{"grave a", "à À", "a a"},
		{"all together", "não compreensão", "nao compreensao"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeAllGermanCharacters tests comprehensive German character set.
func TestNormalizeAllGermanCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"umlaut a", "ä Ä", "a a"},
		{"umlaut o", "ö Ö", "o o"},
		{"umlaut u", "ü Ü", "u u"},
		{"eszett lowercase", "ß", "ss"},
		{"eszett uppercase", "ẞ", "ss"},
		{"eszett in word", "Straße", "strasse"},
		{"all umlauts", "Äpfel Öl Übung", "apfel ol ubung"},
		{"eszett edge case", "Fußball", "fussball"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeAllItalianCharacters tests comprehensive Italian character set.
func TestNormalizeAllItalianCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"grave a", "à À", "a a"},
		{"grave e", "è È", "e e"},
		{"grave i", "ì Ì", "i i"},
		{"grave o", "ò Ò", "o o"},
		{"grave u", "ù Ù", "u u"},
		{"acute e", "é É", "e e"},
		{"all together", "È così città", "e cosi citta"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeAllRomanianCharacters tests comprehensive Romanian character set.
func TestNormalizeAllRomanianCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"a breve", "ă Ă", "a a"},
		{"a circumflex", "â Â", "a a"},
		{"i circumflex", "î Î", "i i"},
		{"s comma", "ș Ș", "s s"},
		{"t comma", "ț Ț", "t t"},
		{"all together", "Română Bună", "romana buna"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeComplexMixedText tests complex real-world scenarios.
func TestNormalizeComplexMixedText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"Spanish with URLs",
			"Visita https://example.com/café para más información",
			"visita https://example.com/café para mas informacion",
		},
		{
			"French with email",
			"Contactez-nous à café@société.fr dès maintenant",
			"contactez nous a cafe societe fr des maintenant", // Email domain with accents isn't matched
		},
		{
			"German with phone",
			"Rufen Sie +49-123-456-789 für Größe an",
			"rufen sie 49_123_456_789 fur grosse an",
		},
		{
			"Mixed punctuation",
			"Hello—world! ¿Cómo estás?",
			"hello world como estas",
		},
		{
			"Initialisms and accents",
			"The U.S.A. and México are neighbors",
			"the u_s_a and mexico are neighbors",
		},
		{
			"All Romance accents",
			"São Paulo, París, München, România",
			"sao paulo paris munchen romania",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeEdgeCases tests edge cases and boundary conditions.
func TestNormalizeEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"only punctuation", "!!!???...", ""},
		{"only accents", "áéíóú", "aeiou"},
		{"repeated spaces", "hello    world", "hello world"},
		{"tabs and newlines", "hello\t\nworld", "hello world"},
		{"mixed whitespace", "  hello \n\t  world  ", "hello world"},
		{"apostrophes", "don't can't won't", "dont cant wont"},
		{"hyphens", "mother-in-law", "mother in law"},
		{"underscores outside special", "hello_world_test", "hello world test"},
		{"multiple punctuation", "Hello!!! World???", "hello world"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeURLsAndSpecialTokens tests that special tokens are preserved correctly.
func TestNormalizeURLsAndSpecialTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"URL with path",
			"Visit https://example.com/path/to/page",
			"visit https://example.com/path/to/page",
		},
		{
			"URL with query",
			"Go to https://example.com?param=value",
			"go to https://example.com?param=value",
		},
		{
			"Multiple URLs",
			"See https://a.com and https://b.com",
			"see https://a.com and https://b.com",
		},
		{
			"Email and phone together",
			"Call +1-555-0123 or email test@example.com",
			"call 1_555_0123 or email test@example.com",
		},
		{
			"Initialism with periods not at end",
			"U.S.A. is great",
			"u_s_a is great",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeArabicCharacters tests Arabic-specific normalization.
func TestNormalizeArabicCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple Arabic", "السلام عليكم", "السلام عليكم"},
		{"Arabic with punctuation", "مرحبا!", "مرحبا"},
		{"Arabic with numbers", "٣ أشخاص", "٣ اشخاص"}, // Note: Arabic diacriticals are stripped by NFD
		{"mixed Arabic English", "Hello مرحبا", "hello مرحبا"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeChineseCharacters tests Chinese-specific normalization.
func TestNormalizeChineseCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple Chinese", "你好世界", "你好世界"},
		{"Chinese with period", "你好。世界。", "你好 世界"},
		{"Chinese with comma", "一、二、三", "一 二 三"},
		{"traditional characters", "繁體中文", "繁體中文"},
		{"mixed Chinese English", "Hello 你好", "hello 你好"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestNormalizeJapaneseCharacters tests Japanese-specific normalization.
func TestNormalizeJapaneseCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"hiragana", "ひらがな", "ひらかな"}, // Note: NFD strips dakuten marks
		{"katakana", "カタカナ", "カタカナ"},
		{"kanji", "日本語", "日本語"},
		{"mixed Japanese", "これは日本語です", "これは日本語てす"}, // Note: NFD strips dakuten marks
		{"Japanese with punctuation", "何？", "何"},
		{"Japanese period", "こんにちは。", "こんにちは"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			if result != test.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}
