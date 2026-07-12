package sorters

import (
	"testing"
)

// TestSpanishAccents tests that Spanish accented characters are normalized.
func TestSpanishAccents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"acute vowels", "María José", "maria jose"},
		{"all acute vowels", "á é í ó ú", "a e i o u"},
		{"ñ character", "España mañana", "espana manana"},
		{"mixed", "José García vivió en México", "jose garcia vivio en mexico"},
		{"uppercase", "JOSÉ GARCÍA", "jose garcia"},
		{"question marks", "¿Cómo estás?", "como estas"},
		{"exclamation", "¡Hola!", "hola"},
		{"sentence", "El niño comió en la montaña", "el nino comio en la montana"},
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

// TestFrenchAccents tests that French accented characters are normalized.
func TestFrenchAccents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"grave accents", "à è ù", "a e u"},
		{"acute accents", "é", "e"},
		{"circumflex", "â ê î ô û", "a e i o u"},
		{"cedilla", "ç français", "c francais"},
		{"diaeresis", "ï ë", "i e"},
		{"mixed", "Café résumé", "cafe resume"},
		{"sentence", "Le château est très beau", "le chateau est tres beau"},
		{"complex", "L'événement à l'hôtel", "levenement a lhotel"}, // Apostrophes are removed
		{"uppercase", "FRANÇOIS RENÉ", "francois rene"},
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

// TestPortugueseAccents tests that Portuguese accented characters are normalized.
func TestPortugueseAccents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde", "ã õ", "a o"},
		{"acute", "á é í ó ú", "a e i o u"},
		{"circumflex", "â ê ô", "a e o"},
		{"cedilla", "ç", "c"},
		{"mixed", "São Paulo", "sao paulo"},
		{"sentence", "A educação é importante", "a educacao e importante"},
		{"complex", "Não compreensão", "nao compreensao"},
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

// TestGermanAccents tests that German special characters are normalized.
func TestGermanAccents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"umlaut a", "ä", "a"},
		{"umlaut o", "ö", "o"},
		{"umlaut u", "ü", "u"},
		{"eszett", "ß", "ss"},
		{"mixed", "Müller Größe", "muller grosse"},
		{"sentence", "Das Mädchen über der Brücke", "das madchen uber der brucke"},
		{"uppercase", "MÜNCHEN KÖLN", "munchen koln"},
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

// TestItalianAccents tests that Italian accented characters are normalized.
func TestItalianAccents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"grave accents", "à è ì ò ù", "a e i o u"},
		{"acute e", "é", "e"},
		{"mixed", "perché città", "perche citta"},
		{"sentence", "È vero", "e vero"},
		{"complex", "università qualità", "universita qualita"},
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

// TestRomanianAccents tests Romanian special characters.
func TestRomanianAccents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"a breve", "ă", "a"},
		{"a circumflex", "â", "a"},
		{"i circumflex", "î", "i"},
		{"s comma", "ș", "s"},
		{"t comma", "ț", "t"},
		{"mixed", "Română", "romana"},
		{"sentence", "Bună ziua", "buna ziua"},
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

// TestSpanishTextMatching tests that Spanish text with/without accents can match.
func TestSpanishTextMatching(t *testing.T) {
	// Test that accented and unaccented versions normalize to same form
	accented := "Hola cómo estás"
	unaccented := "Hola como estas"

	norm1 := NormalizeText(accented)
	norm2 := NormalizeText(unaccented)

	if norm1 != norm2 {
		t.Errorf("Spanish text normalization mismatch: %q vs %q", norm1, norm2)
	}

	expected := "hola como estas"
	if norm1 != expected {
		t.Errorf("Expected %q, got %q", expected, norm1)
	}
}

// TestFrenchTextMatching tests that French text with/without accents can match.
func TestFrenchTextMatching(t *testing.T) {
	// Test that accented and unaccented versions normalize to same form
	accented := "Café français très élégant"
	unaccented := "Cafe francais tres elegant"

	norm1 := NormalizeText(accented)
	norm2 := NormalizeText(unaccented)

	if norm1 != norm2 {
		t.Errorf("French text normalization mismatch: %q vs %q", norm1, norm2)
	}

	expected := "cafe francais tres elegant"
	if norm1 != expected {
		t.Errorf("Expected %q, got %q", expected, norm1)
	}
}

// TestMixedAccentConsistency tests that text with and without accents match.
func TestMixedAccentConsistency(t *testing.T) {
	// OCR block has accents
	withAccents := "José García Pérez"
	// Canonical text doesn't have accents
	withoutAccents := "Jose Garcia Perez"

	norm1 := NormalizeText(withAccents)
	norm2 := NormalizeText(withoutAccents)

	if norm1 != norm2 {
		t.Errorf("Accent normalization inconsistent: %q vs %q", norm1, norm2)
	}

	expected := "jose garcia perez"
	if norm1 != expected {
		t.Errorf("Expected %q, got %q", expected, norm1)
	}
}

// TestAccentPreservationInURLs tests that accents in URLs are preserved.
func TestAccentPreservationInURLs(t *testing.T) {
	input := "Visit https://example.com/café for more"
	result := NormalizeText(input)

	// URLs should be preserved as-is
	if !contains(result, "https://example.com/café") {
		t.Errorf("URL with accent should be preserved in: %q", result)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
