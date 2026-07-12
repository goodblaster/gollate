# Language Handler Architecture

The language handler architecture provides a modular, extensible system for handling language-specific requirements in OCR text sorting. This allows the core sorting algorithm to remain language-agnostic while supporting diverse languages with different characteristics.

## Overview

Different languages have different requirements for text processing:

- **English**: Space-separated words, horizontal left-to-right reading, spaces between words
- **CJK (Chinese/Japanese/Korean)**: No spaces between characters, horizontal left-to-right (modern), no spaces between CJK characters
- **Mixed Content**: Documents combining multiple scripts (e.g., English + Chinese)

The language handler architecture abstracts these differences into a clean interface.

## Architecture

### Handler Interface

The `Handler` interface defines language-specific behavior:

```go
type Handler interface {
    // Name returns the handler name for debugging/logging
    Name() string

    // DetectScript returns confidence score (0.0-1.0) that text belongs to this script
    DetectScript(text string) float64

    // ReadingOrder defines how blocks should be spatially sorted
    ReadingOrder() ReadingOrder

    // NeedsSpaceBetween determines if space should be added between two blocks in output
    NeedsSpaceBetween(current, next string) bool

    // OCRSettings provides hints for OCR engines
    OCRSettings() OCRSettings

    // Tokenize splits text into matchable units (words for English, phrases for CJK)
    Tokenize(text string) []string
}
```

### Implementations

#### English Handler

- **Script Detection**: Counts ASCII letters, numbers, and punctuation
- **Reading Order**: Horizontal left-to-right, top-to-bottom
- **Spacing**: Always adds spaces between words
- **Tokenization**: Whitespace-based (space-separated words)
- **OCR Settings**: Fast recognition, language code "en-US"

#### CJK Handler

- **Script Detection**: Counts CJK characters (Chinese/Japanese/Korean Unicode ranges)
- **Reading Order**: Horizontal left-to-right, top-to-bottom (modern usage)
- **Spacing**: No spaces between CJK characters, spaces when mixing with non-CJK
- **Tokenization**: Whitespace-based (preserves multi-character sequences)
- **OCR Settings**: Accurate recognition, character splitting enabled, language codes for zh/ja/ko

#### MixedContent Handler

- **Script Detection**: Returns highest confidence among sub-handlers
- **Reading Order**: Default horizontal left-to-right, top-to-bottom
- **Spacing**: Heuristic-based (spaces unless both blocks are CJK)
- **Tokenization**: Whitespace-based
- **OCR Settings**: Accurate recognition with all language codes

### Auto-Detection

The `Detect()` function analyzes text and returns the most appropriate handler:

```go
handler := language.Detect(canonicalText...)
```

Detection logic:
1. Tests all handlers against the text
2. Returns handler with highest confidence score
3. If no handler has >80% confidence, returns `MixedContent` handler

## Integration Points

### 1. Sorter Integration

The `Sorter` struct automatically detects and stores the language handler:

```go
// In NewOcrSorterWithConfig:
handler := language.Detect(text...)
sorter := &Sorter{
    handler: handler,
    // ... other fields
}
```

### 2. Tokenization

Replaced `strings.Fields()` calls with `handler.Tokenize()`:

```go
// Split canonical line into tokens
tokens := s.handler.Tokenize(line.Normalized)
```

This allows language-specific word boundary detection.

### 3. Spacing Rules

Block spacing uses `handler.NeedsSpaceBetween()`:

```go
// When converting blocks to lines
if handler.NeedsSpaceBetween(currentBlock.Text, nextBlock.Text) {
    line.WriteByte(' ')
}
```

This ensures:
- English words are space-separated
- CJK characters have no spaces between them
- Mixed content has appropriate spacing

### 4. OCR Settings

The handler's OCR settings influence OCR engine configuration:

```go
settings := handler.OCRSettings()
// settings.LanguageCodes -> OCR language hints
// settings.RecognitionLevel -> "fast" or "accurate"
// settings.RequiresCharSplit -> character-level tokenization
```

For Apple Vision OCR:
- Language codes passed to Vision framework
- Recognition level mapped to VNRequestTextRecognitionLevel
- Fast for English, Accurate for CJK

## Usage Examples

### Example 1: Basic Detection

```go
import "github.com/goodblaster/gollate/pkg/language"

// Detect from canonical text
handler := language.Detect(
    "Hello World",
    "This is English text",
)
// Returns: &English{}

handler := language.Detect(
    "維基百科",
    "中文文本",
)
// Returns: &CJK{}
```

### Example 2: Using Handler Settings

```go
handler := language.Detect(text...)

// Get spacing rules
needsSpace := handler.NeedsSpaceBetween("Hello", "World")
// English: true
// CJK: depends on content

// Get tokenization
tokens := handler.Tokenize("Hello World")
// English: ["Hello", "World"]
// CJK: whitespace-based splitting

// Get OCR settings
settings := handler.OCRSettings()
// settings.LanguageCodes: ["en-US"] or ["zh-Hant", "zh-Hans", ...]
// settings.RecognitionLevel: "fast" or "accurate"
```

### Example 3: OCR Integration

```go
import (
    "github.com/goodblaster/gollate/pkg/language"
    "github.com/goodblaster/gollate/pkg/ocr/apple"
)

// Convert handler settings to OCR parameters
handler := &language.CJK{}
langs, recognitionLevel := apple.LanguageSettingsFromHandler(handler)
// langs: ["zh-Hant", "zh-Hans", "ja-JP", "ko-KR"]
// recognitionLevel: 1 (accurate)

// Use for OCR
engine := &apple.Engine{}
lines, err := engine.ParseFile(imagePath, langs)
```

## Design Decisions

### Tokenization Strategy

**Decision**: All handlers use whitespace-based tokenization.

**Rationale**:
- Keeps the sorting algorithm simple and consistent
- CJK text without spaces is treated as a single token
- Spacing rules (not tokenization) control output formatting
- Tried character-level CJK tokenization but it disrupted sorting

### Spacing vs Tokenization

**Key Insight**: Tokenization handles word boundaries for matching; spacing rules handle output formatting.

- **Tokenization** (how to split for matching):
  - English: "Hello World" → ["Hello", "World"]
  - CJK: "維基百科" → ["維基百科"] (no spaces, single token)

- **Spacing** (how to join for output):
  - English: Always add spaces
  - CJK: No spaces between CJK characters

### Handler Selection

**Decision**: Detect language from canonical text, not OCR output.

**Rationale**:
- Canonical text is the ground truth
- OCR output may have errors or missing text
- Canonical text determines expected language

## Testing

Comprehensive test coverage in `handler_test.go`:

- **Script Detection**: Verifies confidence scores for pure and mixed text
- **Spacing Rules**: Tests all combinations (English-English, CJK-CJK, mixed)
- **Tokenization**: Validates whitespace-based splitting for all handlers
- **Reading Order**: Confirms directional settings
- **OCR Settings**: Checks language codes and recognition levels
- **Auto-Detection**: Tests language detection with threshold logic

Run tests:
```bash
go test ./pkg/language/... -v
```

## Future Enhancements

### Phase 5: Reading Order Support (Future)

The `ReadingOrder` interface supports vertical text:

```go
type ReadingOrder struct {
    Primary   Direction          // Horizontal or Vertical
    Secondary Direction          // Perpendicular to primary
    HorizontalDir HorizontalDirection // LeftToRight or RightToLeft
    VerticalDir   VerticalDirection   // TopToBottom or BottomToTop
}
```

Future work could:
- Add vertical text handler for traditional Japanese/Chinese
- Update sorting algorithm to use reading order for spatial sorting
- Support right-to-left languages (Arabic, Hebrew)

### Additional Language Support

Adding a new language:

1. Create handler file (e.g., `arabic.go`)
2. Implement `Handler` interface
3. Add script detection logic
4. Define spacing rules
5. Configure OCR settings
6. Add tests
7. Register in `Detect()` function

## Performance

The language handler architecture has minimal performance overhead:

- **Detection**: O(n) where n = text length, runs once per sort operation
- **Tokenization**: O(n) where n = text length, replaces `strings.Fields()`
- **Spacing**: O(1) per block pair, already required for output formatting
- **OCR Settings**: O(1) lookup, happens before OCR execution

## Summary

The language handler architecture successfully:

✅ Modularizes language-specific logic
✅ Keeps core algorithm language-agnostic
✅ Supports English, CJK, and mixed content
✅ Integrates cleanly with existing codebase
✅ Maintains backward compatibility
✅ Provides foundation for future languages
✅ Includes comprehensive test coverage

All tests passing, all functionality verified for both English and CJK text processing.
