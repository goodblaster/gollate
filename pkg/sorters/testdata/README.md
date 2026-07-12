# Test Data for Multilingual OCR Sorting

This directory contains test documents for verifying OCR sorting across different languages and layouts.

## File Structure

Each test consists of up to four files:
- `{language}-{type}-canonical.txt` - Ground truth text with proper accents/diacritics
- `{language}-{type}-ocr.json` - Simulated OCR output with:
  - Bounding boxes (top, left, width, height as percentages 0-1)
  - Text blocks (potentially missing or incorrect accents)
  - Multi-column layouts where applicable
- `{language}-{type}.pdf` - PDF rendering of the canonical text for visualization
- `{language}-{type}.jpg` - JPEG rendering of the canonical text for OCR testing

## Test Documents

### English Newspaper (english-newspaper-*)
- **Words**: ~300
- **Layout**: Two-column newspaper
- **Features**: Headlines, body text, wrapping sentences, technology news

### Spanish Newspaper (spanish-newspaper-*)
- **Words**: ~350
- **Layout**: Two-column newspaper
- **Accent issues**: Missing acute accents (á→a, é→e, í→i, ó→o, ú→u), missing ñ→n
- **Features**: Headlines, body text, wrapping sentences

### French Article (french-article-*)
- **Words**: ~300
- **Layout**: Single column
- **Accent issues**: Missing grave (à, è, ù), acute (é), circumflex (â, ê, î, ô, û), cedilla (ç)
- **Features**: Long paragraphs, complex sentences, education news

### Chinese Article (chinese-article-*)
- **Words**: ~300 characters (simplified Chinese)
- **Layout**: Single column
- **Features**: Chinese characters, economy and technology news
- **Script**: Simplified Chinese

### Japanese Article (japanese-article-*)
- **Words**: ~300 characters
- **Layout**: Single column (horizontal left-to-right)
- **Features**: Mixed kanji/hiragana/katakana, economy and technology news
- **Script**: Japanese

### Arabic Newspaper (arabic-newspaper-*)
- **Words**: ~300 (Arabic)
- **Layout**: Two-column RTL
- **Features**: Right-to-left text flow, Arabic script, economy news

### Hindi Article (hindi-article-*)
- **Words**: ~300
- **Layout**: Single column
- **Features**: Devanagari script, Indian economy news
