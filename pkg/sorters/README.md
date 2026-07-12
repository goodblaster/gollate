# OCR Sorting Algorithm

The `pkg/sorters` package implements the core sorting algorithm that matches unordered OCR blocks to canonical text using spatial pathfinding. This document explains how to use the algorithm and how it works internally.

## Table of Contents

- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [Algorithm Overview](#algorithm-overview)
- [Core Concepts](#core-concepts)
- [Configuration Options](#configuration-options)
- [Performance Optimization](#performance-optimization)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/goodblaster/gollate/pkg/sorters"
    "github.com/goodblaster/gollate/pkg/norm"
    "github.com/goodblaster/gollate/internal/logger"
)

func main() {
    // 1. Prepare OCR blocks (normalized from OCR engine output)
    ocrBlocks := []norm.Block{
        {Text: "Hello", BoundingBox: norm.BoundingBox{Left: 0.1, Top: 0.1, Width: 0.1, Height: 0.05}},
        {Text: "World", BoundingBox: norm.BoundingBox{Left: 0.2, Top: 0.1, Width: 0.1, Height: 0.05}},
    }

    // 2. Provide canonical text (expected reading order)
    canonicalLines := []string{
        "Hello World",
    }

    // 3. Create sorter (optional logger for debug output)
    log := logger.Default() // or logger.Noop{} for no logging
    sorter := sorters.NewOcrSorter(ocrBlocks, canonicalLines, log)

    // 4. Sort the blocks
    sortedBlocks, err := sorter.Sort()
    if err != nil {
        panic(err)
    }

    // 5. Access results
    fmt.Println("Sorted lines:", sorter.SortedLines())
    fmt.Println("Unhandled lines:", sorter.UnhandledLines())
    fmt.Println("Blocks:", len(sortedBlocks))
}
```

### With Custom Configuration

```go
// Create custom configuration
config := sorters.SorterConfig{
    MaxPermutations:       1000000,  // Max paths to explore per line
    SplitHyphenatedWords:  true,     // Split "twenty-three" into "twenty" and "three"
}

// Create sorter with config
sorter := sorters.NewOcrSorterWithConfig(ocrBlocks, canonicalLines, log, config)
sortedBlocks, err := sorter.Sort()
```

## How It Works

The sorting algorithm solves the problem: **"Given unordered OCR text blocks and expected canonical text, reconstruct the reading order."**

### The Challenge

OCR engines return text blocks (words) with bounding boxes but may not preserve reading order:

```
OCR Output:               Canonical Text:
What is                   What is Lorem Ipsum?
Lorem Ipsum?              Lorem Ipsum is simply dummy text
Lorem Ipsum is simply     of the printing and typesetting industry.
dummy text of the
printing and typesetting
industry.
```

The algorithm must:
1. Match OCR blocks to canonical text
2. Determine correct spatial reading order
3. Handle missing or extra text
4. Preserve line breaks

## Algorithm Overview

### Phase 1: Initialization

```
Input:
  - OCR blocks: [{text, bbox, confidence}, ...]
  - Canonical lines: ["line 1", "line 2", ...]

Steps:
  1. Normalize all text (lowercase, remove punctuation)
  2. Sort canonical lines by length (longest first)
  3. Build mapping: normalized_text -> [blocks]
  4. Detect language (English, CJK, Mixed)
  5. Split hyphenated words (if enabled)
```

**Why longest-first?** Longer lines provide more context and reduce ambiguity. Matching "Lorem Ipsum is simply dummy text of the printing" (8 words) is more accurate than matching "Lorem Ipsum" (2 words).

### Phase 2: Pathfinding for Each Canonical Line

For each canonical line in the sorted order:

```
Canonical Line: "lorem ipsum is simply dummy text"
Normalized Tokens: ["lorem", "ipsum", "is", "simply", "dummy", "text"]

Block Mapping:
  "lorem" -> [Block2, Block4]
  "ipsum" -> [Block3, Block5]
  "is"    -> [Block1, Block6]
  "simply"-> [Block7]
  "dummy" -> [Block8]
  "text"  -> [Block9]

Possible Paths:
  Path 1: Block2 -> Block3 -> Block1 -> Block7 -> Block8 -> Block9
  Path 2: Block2 -> Block3 -> Block6 -> Block7 -> Block8 -> Block9
  Path 3: Block2 -> Block5 -> Block1 -> Block7 -> Block8 -> Block9
  Path 4: Block2 -> Block5 -> Block6 -> Block7 -> Block8 -> Block9
  Path 5: Block4 -> Block3 -> Block1 -> Block7 -> Block8 -> Block9
  ...

Scoring:
  For each path, calculate total spatial distance:
    distance(Block2, Block3) + distance(Block3, Block6) + ...

  Choose path with minimum total distance (most spatially cohesive)

Selected Path: Block2 -> Block5 -> Block6 -> Block7 -> Block8 -> Block9
  Total Distance: 15.3 pixels

Result:
  - These blocks are marked as "used" (removed from mapping)
  - Output includes these blocks in this order
  - Empty block inserted as line break
```

### Phase 3: Line Splitting (If Needed)

If a line cannot be matched (too many permutations or missing words):

```
1. Check for absent words:
   - Words in canonical text not found in OCR mapping
   - Split line at absent word boundaries

2. Check for sentence boundaries:
   - Split at ". ", "! ", "? "

3. Retry matching on smaller fragments

Example:
  Original: "This is a long sentence that has missing words and exceeds permutation limit."

  Split by absent words:
    -> "This is a long sentence that has"
    -> "words and exceeds permutation limit"

  Split by sentences:
    -> "This is a long sentence that has missing words"
    -> "and exceeds permutation limit"
```

### Phase 4: Post-Processing

After all canonical lines are processed:

```
1. splitLeftoversByCanonicalLines()
   - OCR blocks that weren't matched may still be valid
   - Try to match them against canonical text fragments
   - Example: OCR has "1857 年" merged, canonical has them separate

2. handleUnmatchedBlocks()
   - Remaining blocks are grouped by spatial proximity
   - Connected components become "leftover lines"

3. Output Assembly
   - Combine matched blocks + leftover blocks
   - Insert empty blocks as line breaks
   - Sort by original OCR index for natural reading order
```

## Core Concepts

### 1. Text Normalization

All text is normalized before matching to handle OCR errors and variations:

```go
// Input:  "Julie's iPhone 13!"
// Output: "julies iphone 13"

Steps:
  1. Lowercase: "julie's iphone 13!"
  2. Remove punctuation: "julies iphone 13"
  3. Unicode normalization (NFD -> NFC)
```

**Implementation:** `normalize()` function in `sort.go`

### 2. Distance Calculation

Spatial distance measures how "connected" two blocks are in reading order:

```go
// Horizontal distance (primary)
dx := abs(block2.Left - (block1.Left + block1.Width))

// Vertical distance (secondary)
dy := abs(block2.Top - block1.Top)

// Combined distance (weighted heavily toward horizontal)
distance := dx + (dy * 10)
```

**Why?** Text is read left-to-right primarily, so horizontal gaps matter more than vertical alignment.

**Implementation:** `pkg/sorters/distance.go`

### 3. Pathfinding with Rotation Optimization

When exploring block permutations, the search is optimized:

```
Without Rotation:
  Try blocks in mapping order: [Block0, Block1, Block2, Block3, Block4]

With Rotation:
  Start from most likely next position (closest to previous block)
  Try blocks in distance-sorted order: [Block3, Block4, Block2, Block0, Block1]
```

This dramatically reduces average path length to optimal solution.

**Implementation:** `Recurse()` function with rotation parameter

### 4. Precurse Optimization

For very long lines (>10 words), finding the optimal starting block is critical:

```
Problem:
  Line: "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10"
  "word1" -> [Block5, Block12, Block43, Block89]

  Without precurse: Try all 4 starting blocks with full recursion
  With precurse: Search only first 3 words to find best start

Precurse:
  1. Try each possible starting block
  2. Recurse only 3 words deep
  3. Choose starting block with shortest path
  4. Now recurse full depth from optimal start
```

**Implementation:** `precurse()` function in `sort.go`

### 5. Permutation Limits

To prevent exponential blowup, path exploration is capped:

```go
Default: 1,000,000 permutations per line

If exceeded:
  1. Line is marked for splitting
  2. Split by absent words or sentence boundaries
  3. Retry with shorter fragments
```

**Configuration:** `SorterConfig.MaxPermutations`

### 6. Language-Aware Processing

The algorithm adapts to different languages:

```go
English:
  - Tokenization: whitespace-separated
  - Spacing: always add spaces between words
  - Example: "Hello" + "World" -> "Hello World"

CJK (Chinese/Japanese/Korean):
  - Tokenization: whitespace-separated (words/phrases kept together)
  - Spacing: no spaces between CJK characters
  - Example: "維基" + "百科" -> "維基百科"

Mixed Content:
  - Tokenization: whitespace-separated
  - Spacing: add spaces when mixing scripts
  - Example: "維基" + "Wiki" -> "維基 Wiki"
```

**Implementation:** Language handlers in `pkg/language/`

### 7. Hyphenated Word Splitting

When enabled, hyphenated words are split for better matching:

```go
Input Block:
  Text: "twenty-three"
  BoundingBox: {Left: 0.1, Top: 0.2, Width: 0.2, Height: 0.05}

Output Blocks:
  {Text: "twenty", BoundingBox: {Left: 0.1, Top: 0.2, Width: 0.1, Height: 0.05}}
  {Text: "three",  BoundingBox: {Left: 0.2, Top: 0.2, Width: 0.1, Height: 0.05}}
```

**Why?** OCR may recognize "twenty-three" as one block, but canonical text may have it as two words.

**Configuration:** `SorterConfig.SplitHyphenatedWords`

### 8. Line Breaks in Output

Empty blocks serve as line separators:

```go
Output: [
  Block{Text: "Hello", ...},
  Block{Text: "World", ...},
  Block{},  // Empty block = line break
  Block{Text: "Next", ...},
  Block{Text: "Line", ...},
]

Text Representation:
  "Hello World"
  "Next Line"
```

This preserves document structure for downstream consumers.

## Configuration Options

`SorterConfig` (config.go) is the single source of truth for all fields and
defaults; every field carries a doc comment. The main groups:

- **Search budget**: `MaxPermutations`, `PermutationsPerPass`, `MaxPasses`,
  `PrecurseLength`, `MinWordsForEarlyPasses`.
- **Geometry**: `MaxWordDistance`, `ReadingOrder`, `ColumnJumpPenalty`.
- **Text handling**: `SplitHyphenatedWords`, spelling-correction settings
  (metadata-only suggestions, never applied).
- **Error tolerance** (see below): `EnableWrapBridging`, `EnableChainHoles`,
  `EnableShortLineAnchoring`, `EnableReconciliationPass` + their tunables.

Prefer `ConfigForLanguage(lang)` (presets.go) over hand-building a config:
it maps each script to its measured-best combination. Task-oriented presets
(`FastConfig`, `AccurateConfig`, `NoisyOCRConfig`, …) derive from
`DefaultConfig` and override only what they need.

### Error-tolerance mechanisms

Real OCR misreads and drops words; these mechanisms keep matching working
anyway. All are off in `DefaultConfig`; `ConfigForLanguage` enables the
combination each script measurably benefits from:

| Mechanism | What it does | Enabled for |
|---|---|---|
| `EnableWrapBridging` | lets a path follow a canonical line across a visual line wrap (at real wrap cost); without it, no multi-visual-line path can complete | Latin, Hindi |
| `EnableChainHoles` | a small fraction of words missing from OCR become wildcard slots bridged by pathfinding instead of splitting the line; blocks spatially inside a gap can be claimed with exact-text confirmation | Latin, Hindi, CJK |
| `EnableShortLineAnchoring` | the pass loop survives long enough to attempt short lines, and near-tied paths for duplicated short lines ("Learn more") tie-break by proximity to the matched blocks of canonical neighbors | Latin, Hindi, Arabic |
| line repair (`DisableLineRepair`) | preparation step: misread tokens identified by position in the engine's own line grouping (flanked by exact matches) and rekeyed to the canonical word - never changes emitted text | Latin, Hindi |
| `EnableReconciliationPass` | post-loop rescue of unfound fragments, anchor-gated; at `ReconMinExactAnchors=1` it also rescues single-word lines ("Buy") | Latin, Hindi |

These mechanisms follow a few hard design invariants:

- **Match-only.** Output text is always what OCR read; canonical text never
  enters the output. (Anything else inflates accuracy scores by pasting the
  reference into the thing being measured.)
- **Geometry first, text second.** Text similarity may only *confirm* a
  spatially-determined candidate, never search for one globally. No
  mechanism accepts a match that isn't spatially pinned.
- **Language is the only hint.** Mechanisms describe algorithm strategy, not
  page layout; layout and orientation are inferred from block geometry.
- **Winners are promoted and baselines ratchet; losers are deleted**, not
  left as dead flags.

Pure text-similarity matching was tried three separate ways (a global fuzzy
matcher, edit-distance gap-fill, and confidence-based corrections) and each
was measured worthless-or-harmful and removed. Don't reintroduce
similarity-based matching without evidence it beats the positional and
structural mechanisms above.

## Performance Optimization

### Complexity Analysis

- **Best Case:** O(n) - All words match exactly, single path per line
- **Average Case:** O(n * k) - Where k is average paths per word (~2-5)
- **Worst Case:** O(n * k^m) - Where m is line length, k is paths per word
  - Mitigated by: permutation limits, precurse, rotation optimization

### Optimization Strategies

1. **Longest Lines First**
   - Reduces permutations for shorter lines (more blocks already used)
   - Front-loads expensive computation when mapping is fullest

2. **Block Mapping**
   - O(1) lookup: normalized_text -> blocks
   - Avoids linear scanning through all blocks

3. **Rotation Optimization**
   - Tries spatially nearest blocks first
   - Average 2-3x speedup on natural reading order

4. **Precurse**
   - Finds optimal starting block for long lines
   - Prevents exploring wrong paths deeply

5. **Shortest Path Pruning**
   - Abandons paths exceeding current best
   - Reduces unnecessary exploration

6. **Line Splitting**
   - Breaks exponential permutations into manageable chunks
   - Each split reduces k^m to k^(m/2) + k^(m/2)

### Performance Monitoring

```go
// Access metrics after sorting
sorter.Metrics()

Output:
  SortMetrics{
      TotalPermutations: 245671,
      MaxPermutations:   89234,
      AvePermutations:   4012,
      Lines:             61,
      Elapsed:           time.Duration,
  }
```

## Troubleshooting

### Problem: Long Processing Time

**Symptoms:** Sort takes many seconds or minutes

**Causes:**
1. Very long canonical lines (>20 words)
2. Many duplicate words in lines
3. Low MaxPermutations limit causing excessive splitting

**Solutions:**
```go
// Increase permutation limit
config.MaxPermutations = 5000000

// Pre-split long lines in canonical text
canonicalLines := []string{
    "First sentence here.",
    "Second sentence here.",
}
// Instead of:
// "First sentence here. Second sentence here."
```

### Problem: Inaccurate Matching

**Symptoms:** Blocks in wrong order, missing text

**Causes:**
1. OCR errors not matching canonical
2. Missing words in OCR

**Solutions:**
```go
// Enable hyphenation splitting
config.SplitHyphenatedWords = true

// Check UnhandledLines for missing text
fmt.Println("Unhandled:", sorter.UnhandledLines())
```

### Problem: Wrong Line Breaks

**Symptoms:** Lines merged or split incorrectly

**Causes:**
1. Canonical text doesn't match document structure
2. OCR blocks missing position data

**Solutions:**
- Ensure canonical text has one logical unit per line
- Verify OCR blocks have accurate bounding boxes
- Check `splitLeftoversByCanonicalLines()` output

### Problem: CJK Text Formatting Issues

**Symptoms:** Extra spaces in Chinese/Japanese text

**Causes:**
1. Language not detected correctly
2. Mixed content handling

**Solutions:**
- Verify language detection: Check `sorter.handler.Name()`
- Ensure canonical text is properly formatted (no extra spaces in CJK)
- Language handler automatically manages spacing rules

## Advanced Usage

### Custom Logger

```go
// Implement logger.Logger interface
type MyLogger struct{}

func (l *MyLogger) Debug(a ...any) { /* custom logging */ }
func (l *MyLogger) Debugf(format string, args ...any) { /* custom logging */ }
func (l *MyLogger) Error(a ...any) { /* custom logging */ }
func (l *MyLogger) Errorf(format string, args ...any) { /* custom logging */ }
func (l *MyLogger) Fatal(a ...any) { /* custom logging */ }
func (l *MyLogger) WithError(err error) logger.Logger { return l }

// Use custom logger
sorter := sorters.NewOcrSorter(blocks, lines, &MyLogger{})
```

### Accessing Intermediate Results

```go
sorter := sorters.NewOcrSorter(blocks, lines, log)
sortedBlocks, err := sorter.Sort()

// Get sorted text lines
sortedLines := sorter.SortedLines()

// Get lines that couldn't be matched
unhandledLines := sorter.UnhandledLines()

// Get performance metrics
metrics := sorter.Metrics()
fmt.Printf("Total permutations: %d\n", metrics.TotalPermutations)
fmt.Printf("Processing time: %v\n", metrics.Elapsed)
```

### Working with Different OCR Engines

The sorter accepts normalized blocks. Use engine-specific readers from `pkg/engines`:

```go
import (
    "github.com/goodblaster/gollate/pkg/engines/apple"
    "github.com/goodblaster/gollate/pkg/norm"
)

// Read Apple Vision OCR output
appleBlocks, err := apple.Read(ocrJsonReader)

// Normalize blocks
var normalized []norm.Block
for _, block := range appleBlocks {
    normalized = append(normalized,
        norm.NormalizeAppleBlock(block, pageWidth, pageHeight))
}

// Sort with normalized blocks
sorter := sorters.NewOcrSorter(normalized, canonicalLines, log)
```

## Summary

The OCR sorting algorithm:

✅ Matches unordered OCR blocks to canonical text
✅ Uses spatial pathfinding to determine reading order
✅ Handles missing words, OCR errors, and ambiguity
✅ Preserves line breaks and document structure
✅ Adapts to multiple languages (English, CJK, Mixed)
✅ Provides configurable accuracy vs. performance trade-offs
✅ Includes comprehensive error handling and metrics

The algorithm is production-ready and has been tested with English and Chinese documents, handling complex layouts including phone numbers, dates, and mixed-script content.
