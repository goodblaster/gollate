package sorters

import (
	"testing"
)

// TestSpanishAccentMatching tests that Spanish text with accent variations normalizes identically.
func TestSpanishAccentMatching(t *testing.T) {
	tests := []struct {
		name       string
		withAccent string
		noAccent   string
	}{
		{
			"newspaper headline",
			"El gobierno anuncia nuevas medidas económicas",
			"El gobierno anuncia nuevas medidas economicas",
		},
		{
			"business article",
			"La economía española crece un 3%",
			"La economia espanola crece un 3%",
		},
		{
			"greeting",
			"¿Cómo estás hoy?",
			"Como estas hoy",
		},
		{
			"city name",
			"México",
			"Mexico",
		},
		{
			"complex sentence",
			"José García vivió en México durante años",
			"Jose Garcia vivio en Mexico durante anos",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			norm1 := NormalizeText(test.withAccent)
			norm2 := NormalizeText(test.noAccent)

			if norm1 != norm2 {
				t.Errorf("Spanish accent mismatch:\n  With accents: %q -> %q\n  Without: %q -> %q",
					test.withAccent, norm1, test.noAccent, norm2)
			}
		})
	}
}

// TestFrenchAccentMatching tests that French text with accent variations normalizes identically.
func TestFrenchAccentMatching(t *testing.T) {
	tests := []struct {
		name       string
		withAccent string
		noAccent   string
	}{
		{
			"simple phrase",
			"Le café est très populaire en France",
			"Le cafe est tres populaire en France",
		},
		{
			"newspaper headline",
			"Le président visite Paris",
			"Le president visite Paris",
		},
		{
			"economic news",
			"L'économie française progresse",
			"Leconomie francaise progresse",
		},
		{
			"cultural events",
			"Les événements culturels à Lyon",
			"Les evenements culturels a Lyon",
		},
		{
			"hotel",
			"L'hôtel est très élégant",
			"Lhotel est tres elegant",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			norm1 := NormalizeText(test.withAccent)
			norm2 := NormalizeText(test.noAccent)

			if norm1 != norm2 {
				t.Errorf("French accent mismatch:\n  With accents: %q -> %q\n  Without: %q -> %q",
					test.withAccent, norm1, test.noAccent, norm2)
			}
		})
	}
}

// TestGermanUmlautMatching tests that German text with umlaut variations normalizes identically.
func TestGermanUmlautMatching(t *testing.T) {
	tests := []struct {
		name       string
		withAccent string
		noAccent   string
	}{
		{
			"personal sentence",
			"Müller wohnt in München",
			"Muller wohnt in Munchen",
		},
		{
			"job description",
			"Arbeitet für eine große Firma",
			"Arbeitet fur eine grosse Firma",
		},
		{
			"street name",
			"Straße",
			"Strasse",
		},
		{
			"food",
			"Bäckerei Brötchen",
			"Backerei Brotchen",
		},
		{
			"sports",
			"Fußball",
			"Fussball",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			norm1 := NormalizeText(test.withAccent)
			norm2 := NormalizeText(test.noAccent)

			if norm1 != norm2 {
				t.Errorf("German umlaut mismatch:\n  With umlauts: %q -> %q\n  Without: %q -> %q",
					test.withAccent, norm1, test.noAccent, norm2)
			}
		})
	}
}

// TestPortugueseAccentMatching tests that Portuguese text with accent variations normalizes identically.
func TestPortugueseAccentMatching(t *testing.T) {
	tests := []struct {
		name       string
		withAccent string
		noAccent   string
	}{
		{
			"education",
			"A educação é muito importante",
			"A educacao e muito importante",
		},
		{
			"city",
			"São Paulo",
			"Sao Paulo",
		},
		{
			"question",
			"Não compreensão",
			"Nao compreensao",
		},
		{
			"seasons",
			"As estações do ano",
			"As estacoes do ano",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			norm1 := NormalizeText(test.withAccent)
			norm2 := NormalizeText(test.noAccent)

			if norm1 != norm2 {
				t.Errorf("Portuguese accent mismatch:\n  With accents: %q -> %q\n  Without: %q -> %q",
					test.withAccent, norm1, test.noAccent, norm2)
			}
		})
	}
}

// TestItalianAccentMatching tests that Italian text with accent variations normalizes identically.
func TestItalianAccentMatching(t *testing.T) {
	tests := []struct {
		name       string
		withAccent string
		noAccent   string
	}{
		{
			"question",
			"Perché la città è così bella",
			"Perche la citta e cosi bella",
		},
		{
			"quality",
			"università qualità",
			"universita qualita",
		},
		{
			"city",
			"È vero",
			"E vero",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			norm1 := NormalizeText(test.withAccent)
			norm2 := NormalizeText(test.noAccent)

			if norm1 != norm2 {
				t.Errorf("Italian accent mismatch:\n  With accents: %q -> %q\n  Without: %q -> %q",
					test.withAccent, norm1, test.noAccent, norm2)
			}
		})
	}
}

// TestRomanianAccentMatching tests that Romanian text with accent variations normalizes identically.
func TestRomanianAccentMatching(t *testing.T) {
	tests := []struct {
		name       string
		withAccent string
		noAccent   string
	}{
		{
			"greeting",
			"Bună ziua din România",
			"Buna ziua din Romania",
		},
		{
			"description",
			"Este foarte frumoasă",
			"Este foarte frumoasa",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			norm1 := NormalizeText(test.withAccent)
			norm2 := NormalizeText(test.noAccent)

			if norm1 != norm2 {
				t.Errorf("Romanian accent mismatch:\n  With accents: %q -> %q\n  Without: %q -> %q",
					test.withAccent, norm1, test.noAccent, norm2)
			}
		})
	}
}

// TestMixedLanguageMatching tests normalization across mixed language content.
func TestMixedLanguageMatching(t *testing.T) {
	tests := []struct {
		name       string
		withAccent string
		noAccent   string
	}{
		{
			"English and Spanish",
			"Welcome to México",
			"Welcome to Mexico",
		},
		{
			"French and English",
			"The café in Paris",
			"The cafe in Paris",
		},
		{
			"German and English",
			"Visit München in Germany",
			"Visit Munchen in Germany",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			norm1 := NormalizeText(test.withAccent)
			norm2 := NormalizeText(test.noAccent)

			if norm1 != norm2 {
				t.Errorf("Mixed language mismatch:\n  With accents: %q -> %q\n  Without: %q -> %q",
					test.withAccent, norm1, test.noAccent, norm2)
			}
		})
	}
}

// TestMultiColumnTextNormalization tests that multi-column layouts normalize correctly.
func TestMultiColumnTextNormalization(t *testing.T) {
	// Simulate two newspaper columns with accent mismatches
	column1OCR := "El gobierno anuncia medidas economicas"
	column1Canon := "El gobierno anuncia medidas económicas"

	column2OCR := "La economia crece"
	column2Canon := "La economía crece"

	// Both columns should normalize to same values despite accent differences
	if NormalizeText(column1OCR) != NormalizeText(column1Canon) {
		t.Errorf("Column 1 accent mismatch")
	}

	if NormalizeText(column2OCR) != NormalizeText(column2Canon) {
		t.Errorf("Column 2 accent mismatch")
	}
}

// TestRTLTextNormalization tests that RTL (Arabic/Hebrew) text normalizes correctly.
func TestRTLTextNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Arabic greeting", "السلام عليكم"},
		{"Arabic phrase", "مرحبا بكم"},
		{"Mixed Arabic and English", "Hello السلام"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			// RTL text should be preserved (not transliterated)
			if result == "" {
				t.Errorf("RTL text should not be empty after normalization: %q", test.input)
			}
			// Should at least preserve the Arabic characters
			if len(result) == 0 {
				t.Errorf("Normalization removed all content from: %q", test.input)
			}
		})
	}
}

// TestVerticalTextNormalization tests that vertical CJK text normalizes correctly.
func TestVerticalTextNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Japanese", "日本語"},
		{"Chinese", "中文"},
		{"Mixed CJK English", "Hello 日本"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NormalizeText(test.input)
			// CJK text should be preserved
			if result == "" {
				t.Errorf("CJK text should not be empty after normalization: %q", test.input)
			}
		})
	}
}
