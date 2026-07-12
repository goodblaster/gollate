package sorters

import (
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// BenchmarkEnglish50Lines benchmarks English text sorting with 50 lines
func BenchmarkEnglish50Lines(b *testing.B) {
	canonical, blocks := generateEnglishDocument(50)
	config := DefaultConfig()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		// Report metrics on first iteration
		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
			b.ReportMetric(float64(sorter.metrics.LeftoverBlocks), "leftover_blocks")
		}
	}
}

// BenchmarkEnglish100Lines benchmarks English text sorting with 100 lines
func BenchmarkEnglish100Lines(b *testing.B) {
	canonical, blocks := generateEnglishDocument(100)
	config := DefaultConfig()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkEnglish200Lines benchmarks English text sorting with 200 lines
func BenchmarkEnglish200Lines(b *testing.B) {
	canonical, blocks := generateEnglishDocument(200)
	config := DefaultConfig()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkSpanish50Lines benchmarks Spanish text with accents (50 lines)
func BenchmarkSpanish50Lines(b *testing.B) {
	canonical, blocks := generateSpanishDocument(50)
	config := DefaultConfig()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkChinese100Chars benchmarks Chinese text sorting (100 characters)
func BenchmarkChinese100Chars(b *testing.B) {
	canonical, blocks := generateChineseDocument(100)
	config := CJKConfig()
	config.ReadingOrder = HorizontalLTR_TTB // Modern horizontal Chinese

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkChinese500Chars benchmarks Chinese text sorting (500 characters)
func BenchmarkChinese500Chars(b *testing.B) {
	canonical, blocks := generateChineseDocument(500)
	config := CJKConfig()
	config.ReadingOrder = HorizontalLTR_TTB

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkChinese1000Chars benchmarks Chinese text sorting (1000 characters)
func BenchmarkChinese1000Chars(b *testing.B) {
	canonical, blocks := generateChineseDocument(1000)
	config := CJKConfig()
	config.ReadingOrder = HorizontalLTR_TTB

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkJapanese500Chars benchmarks Japanese mixed-script text (500 characters)
func BenchmarkJapanese500Chars(b *testing.B) {
	canonical, blocks := generateJapaneseDocument(500)
	config := CJKConfig()
	config.ReadingOrder = HorizontalLTR_TTB // Modern horizontal Japanese

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkArabic50Lines benchmarks Arabic RTL text (50 lines)
func BenchmarkArabic50Lines(b *testing.B) {
	canonical, blocks := generateArabicDocument(50)
	config := RTLConfig()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// BenchmarkHindi50Lines benchmarks Hindi Devanagari text (50 lines)
func BenchmarkHindi50Lines(b *testing.B) {
	canonical, blocks := generateHindiDocument(50)
	config := DefaultConfig()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.ReportMetric(float64(sorter.metrics.LinesFound), "lines_found")
		}
	}
}

// generateEnglishDocument creates a synthetic English document with N lines
func generateEnglishDocument(numLines int) ([]string, []ocr.Block) {
	sentences := []string{
		"The quick brown fox jumps over the lazy dog near the riverbank.",
		"A journey of a thousand miles begins with a single step forward.",
		"Time flies like an arrow and fruit flies like a banana always.",
		"Knowledge is power but enthusiasm pulls the switch every single time.",
		"The early bird catches the worm but the second mouse gets cheese.",
		"Actions speak louder than words in all aspects of daily life.",
		"Fortune favors the bold and the prepared mind sees opportunities.",
		"Practice makes perfect when you dedicate yourself to improvement daily.",
		"Every cloud has a silver lining if you look hard enough today.",
		"Rome was not built in a day so patience is very important.",
	}

	canonical := make([]string, numLines)
	var allWords []string

	for i := 0; i < numLines; i++ {
		sentence := sentences[i%len(sentences)]
		canonical[i] = sentence
		words := strings.Fields(sentence)
		allWords = append(allWords, words...)
	}

	// Create OCR blocks from all words
	blocks := make([]ocr.Block, len(allWords))
	x, y := 0.05, 0.05

	for i, word := range allWords {
		blocks[i] = ocr.Block{
			Text:       word,
			NormedText: NormalizeText(word),
			Confidence: 0.85 + float64(i%10)*0.01,
			Extractor:  "benchmark",
			Index:      i,
			PageWidth:  1920,
			PageHeight: 1080,
			BoundingBox: ocr.BoundingBox{
				Left:   x,
				Top:    y,
				Width:  0.08,
				Height: 0.02,
			},
		}

		x += 0.09
		if x > 0.85 {
			x = 0.05
			y += 0.03
		}
	}

	return canonical, blocks
}

// generateSpanishDocument creates a synthetic Spanish document with accents
func generateSpanishDocument(numLines int) ([]string, []ocr.Block) {
	sentences := []string{
		"El médico español trabaja en el hospital público todos los días.",
		"La reunión comenzó a las tres de la tarde con gran éxito.",
		"José María visitó la exposición de arte contemporáneo ayer.",
		"La música clásica es muy popular en esta región del país.",
		"El señor González vive en una casa muy bonita cerca aquí.",
		"¿Dónde está la estación de autobús más próxima de la ciudad?",
		"El niño pequeño juega en el jardín con su perro favorito.",
		"La familia cenó en un restaurante típico de la zona anoche.",
		"El corazón de la nación late con fuerza y pasión siempre.",
		"La historia de España es rica y variada a través de los siglos.",
	}

	canonical := make([]string, numLines)
	var allWords []string

	for i := 0; i < numLines; i++ {
		sentence := sentences[i%len(sentences)]
		canonical[i] = sentence
		words := strings.Fields(sentence)
		allWords = append(allWords, words...)
	}

	blocks := make([]ocr.Block, len(allWords))
	x, y := 0.05, 0.05

	for i, word := range allWords {
		blocks[i] = ocr.Block{
			Text:       word,
			NormedText: NormalizeText(word),
			Confidence: 0.82 + float64(i%15)*0.01,
			Extractor:  "benchmark",
			Index:      i,
			PageWidth:  1920,
			PageHeight: 1080,
			BoundingBox: ocr.BoundingBox{
				Left:   x,
				Top:    y,
				Width:  0.08,
				Height: 0.02,
			},
		}

		x += 0.09
		if x > 0.85 {
			x = 0.05
			y += 0.03
		}
	}

	return canonical, blocks
}

// generateChineseDocument creates a synthetic Chinese document with N characters
func generateChineseDocument(numChars int) ([]string, []ocr.Block) {
	// Sample Chinese text (from Chinese column test)
	baseText := "早晨的阳光照亮了安静的街道鸟儿在树上轻声歌唱" +
		"与此同时夜晚的雨开始轻轻落下人们撑着雨伞匆匆回家" +
		"中国是一个历史悠久的文明古国有着丰富的文化传统" +
		"科技发展日新月异改变着我们的生活方式和工作方法"

	// Repeat to get desired length
	fullText := ""
	for len(fullText) < numChars {
		fullText += baseText
	}
	fullText = fullText[:numChars]

	// Split into lines of ~20 characters each
	charsPerLine := 20
	numLines := (numChars + charsPerLine - 1) / charsPerLine
	canonical := make([]string, numLines)

	for i := 0; i < numLines; i++ {
		start := i * charsPerLine
		end := start + charsPerLine
		if end > numChars {
			end = numChars
		}
		canonical[i] = fullText[start:end]
	}

	// Create OCR blocks (one per character for CJK)
	blocks := make([]ocr.Block, numChars)
	x, y := 0.05, 0.05

	for i, r := range fullText {
		char := string(r)
		blocks[i] = ocr.Block{
			Text:       char,
			NormedText: NormalizeText(char),
			Confidence: 0.88 + float64(i%8)*0.01,
			Extractor:  "benchmark",
			Index:      i,
			PageWidth:  1920,
			PageHeight: 1080,
			BoundingBox: ocr.BoundingBox{
				Left:   x,
				Top:    y,
				Width:  0.02,
				Height: 0.025,
			},
		}

		x += 0.025
		if x > 0.90 {
			x = 0.05
			y += 0.03
		}
	}

	return canonical, blocks
}

// generateJapaneseDocument creates a synthetic Japanese document with mixed scripts
func generateJapaneseDocument(numChars int) ([]string, []ocr.Block) {
	// Mixed hiragana, katakana, and kanji
	baseText := "私は日本語を勉強しています。" +
		"コンピューターは便利な道具です。" +
		"春になると桜の花が咲きます。" +
		"この本はとても面白いです。" +
		"東京は日本の首都です。"

	fullText := ""
	for len(fullText) < numChars {
		fullText += baseText
	}
	fullText = fullText[:numChars]

	// Split into lines
	charsPerLine := 15
	numLines := (numChars + charsPerLine - 1) / charsPerLine
	canonical := make([]string, numLines)

	for i := 0; i < numLines; i++ {
		start := i * charsPerLine
		end := start + charsPerLine
		if end > numChars {
			end = numChars
		}
		canonical[i] = fullText[start:end]
	}

	blocks := make([]ocr.Block, numChars)
	x, y := 0.05, 0.05

	for i, r := range fullText {
		char := string(r)
		blocks[i] = ocr.Block{
			Text:       char,
			NormedText: NormalizeText(char),
			Confidence: 0.87 + float64(i%10)*0.01,
			Extractor:  "benchmark",
			Index:      i,
			PageWidth:  1920,
			PageHeight: 1080,
			BoundingBox: ocr.BoundingBox{
				Left:   x,
				Top:    y,
				Width:  0.02,
				Height: 0.025,
			},
		}

		x += 0.025
		if x > 0.90 {
			x = 0.05
			y += 0.03
		}
	}

	return canonical, blocks
}

// generateArabicDocument creates a synthetic Arabic RTL document
func generateArabicDocument(numLines int) ([]string, []ocr.Block) {
	sentences := []string{
		"السلام عليكم ورحمة الله وبركاته",
		"الكتاب مفيد جدا للطلاب في المدرسة",
		"الطقس جميل اليوم في مدينة القاهرة",
		"أحب القراءة والكتابة في وقت الفراغ",
		"العلم نور والجهل ظلام دائما",
		"الصحة تاج على رؤوس الأصحاء",
		"الوقت كالسيف إن لم تقطعه قطعك",
		"من جد وجد ومن سار على الدرب وصل",
		"العمل الجاد يؤدي إلى النجاح دائما",
		"التعليم أساس التقدم في كل مجتمع",
	}

	canonical := make([]string, numLines)
	var allWords []string

	for i := 0; i < numLines; i++ {
		sentence := sentences[i%len(sentences)]
		canonical[i] = sentence
		words := strings.Fields(sentence)
		allWords = append(allWords, words...)
	}

	blocks := make([]ocr.Block, len(allWords))
	x, y := 0.85, 0.05 // Start from right for RTL

	for i, word := range allWords {
		blocks[i] = ocr.Block{
			Text:       word,
			NormedText: NormalizeText(word),
			Confidence: 0.84 + float64(i%12)*0.01,
			Extractor:  "benchmark",
			Index:      i,
			PageWidth:  1920,
			PageHeight: 1080,
			BoundingBox: ocr.BoundingBox{
				Left:   x - 0.08,
				Top:    y,
				Width:  0.08,
				Height: 0.02,
			},
		}

		x -= 0.09
		if x < 0.15 {
			x = 0.85
			y += 0.03
		}
	}

	return canonical, blocks
}

// generateHindiDocument creates a synthetic Hindi Devanagari document
func generateHindiDocument(numLines int) ([]string, []ocr.Block) {
	sentences := []string{
		"नमस्ते आप कैसे हैं आज का दिन बहुत अच्छा है",
		"भारत एक महान देश है जिसकी संस्कृति बहुत समृद्ध है",
		"शिक्षा जीवन का सबसे महत्वपूर्ण हिस्सा है हमेशा",
		"परिश्रम सफलता की कुंजी है इस बात को याद रखें",
		"स्वास्थ्य धन से बढ़कर है इसलिए इसका ध्यान रखें",
		"समय बहुत कीमती है इसका सदुपयोग करना चाहिए",
		"पुस्तकें ज्ञान का भंडार हैं नियमित रूप से पढ़ें",
		"मेहनत का फल मीठा होता है यह सत्य है",
		"सच्चाई और ईमानदारी सबसे बड़ी पूंजी है जीवन में",
		"प्रकृति की रक्षा करना हमारा कर्तव्य है हमेशा",
	}

	canonical := make([]string, numLines)
	var allWords []string

	for i := 0; i < numLines; i++ {
		sentence := sentences[i%len(sentences)]
		canonical[i] = sentence
		words := strings.Fields(sentence)
		allWords = append(allWords, words...)
	}

	blocks := make([]ocr.Block, len(allWords))
	x, y := 0.05, 0.05

	for i, word := range allWords {
		blocks[i] = ocr.Block{
			Text:       word,
			NormedText: NormalizeText(word),
			Confidence: 0.83 + float64(i%13)*0.01,
			Extractor:  "benchmark",
			Index:      i,
			PageWidth:  1920,
			PageHeight: 1080,
			BoundingBox: ocr.BoundingBox{
				Left:   x,
				Top:    y,
				Width:  0.10,
				Height: 0.02,
			},
		}

		x += 0.11
		if x > 0.85 {
			x = 0.05
			y += 0.03
		}
	}

	return canonical, blocks
}

// BenchmarkMultiColumnEnglish benchmarks multi-column layout performance
func BenchmarkMultiColumnEnglish(b *testing.B) {
	canonical, blocks := generateEnglishDocument(50)
	config := MultiColumnConfig()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sorter := NewOcrSorterWithConfig(blocks, canonical, nil, config)
		_, err := sorter.Sort()
		if err != nil {
			b.Fatalf("Sort failed: %v", err)
		}

		if i == 0 {
			b.ReportMetric(float64(sorter.metrics.TotalPermutationsExplored), "permutations")
			b.Logf("Multi-column config: %d permutations, %d lines found",
				sorter.metrics.TotalPermutationsExplored, sorter.metrics.LinesFound)
		}
	}
}

// Benchmark usage:
//   go test -bench=. -benchmem ./pkg/sorters
//
// Expected targets:
//   50 lines:  < 1 second
//   200 lines: < 5 seconds
//
// Metrics tracked:
//   - ns/op: nanoseconds per operation
//   - B/op: bytes allocated per operation
//   - allocs/op: allocations per operation
//   - permutations: total permutations explored
//   - lines_found: canonical lines successfully matched
