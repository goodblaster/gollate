package sorters

// ConfigForLanguage maps a language name to a sorter configuration.
//
// Language is the only external hint the sorter accepts: layout information
// (column counts, orientation, font sizes) must never be supplied by callers.
// The configuration reflects language-level knowledge only; anything
// layout-specific has to be inferred by the algorithm from block geometry.
//
// Each script also gets the error-tolerance mechanisms it measurably
// benefits from:
//
//   - Latin/Hindi (default): the full structural combo - wrap bridging,
//     chain holes, short-line anchoring, reconciliation incl. single-word
//     rescue (grid 99.3/98.6, noise fixtures +8 to +26, hindi-3col +40.8).
//   - Arabic: short-line anchoring (+11.3/+5.0 on noisy Tesseract) and
//     wrap bridging. Bridging originally measured -9.5 on RTL, but that
//     was an artifact: isWrappedToNextLine only recognized the LTR wrap
//     shape, so bridging admitted junk steps and no legitimate ones.
//     With the RTL-aware classifier it measures +15.8/+38.6/+43.0
//     (pdftext single/two/three-column) and +19.9/+10.1 on noisy
//     Tesseract multi-column, -3.0 single (2026-07). Chain holes
//     re-measured with the fix: still net-negative on Tesseract Arabic
//     (-2.7/-12.2/-3.0); stays off.
//   - CJK: chain holes only (+7/+8.4 on noisy pages); wrap bridging and
//     anchoring wreck dense noisy character grids (-16 to -31).
//
// These choose algorithm strategy per script, never page layout.
//
// Recognized languages: english, spanish, chinese, japanese, arabic, hindi.
// Unrecognized values fall back to the default (horizontal LTR) config.
func ConfigForLanguage(lang string) SorterConfig {
	var config SorterConfig
	switch lang {
	case "chinese", "japanese":
		config = CJKConfig()
		// Modern CJK text defaults to horizontal; vertical documents must
		// be detected by the algorithm, not declared by the caller.
		config.ReadingOrder = HorizontalLTR_TTB
		config.EnableChainHoles = true
		// Line repair measured net-negative on noisy Tesseract CJK
		// (chinese-single -0.8 vs three-column +0.5); see PLAN log.
		config.DisableLineRepair = true
	case "arabic":
		config = RTLConfig()
		config.EnableShortLineAnchoring = true
		config.EnableWrapBridging = true
		// Line repair measured net-negative on noisy Tesseract Arabic
		// (-1.8/-2.1 multi-column vs +0.6 single); see PLAN log.
		config.DisableLineRepair = true
	default:
		config = DefaultConfig()
		config.EnableWrapBridging = true
		config.EnableChainHoles = true
		config.EnableShortLineAnchoring = true
		config.EnableReconciliationPass = true
		config.ReconMinExactAnchors = 1
	}
	return config
}

// FastConfig returns a configuration optimized for speed over accuracy.
// Best for clean OCR output where speed is critical.
//
// Use cases:
//   - High-quality OCR with few errors
//   - Real-time processing requirements
//   - Large batch processing
//   - Preview/draft mode
func FastConfig() SorterConfig {
	config := DefaultConfig()
	config.MaxPermutations = 100000     // Reduced for speed
	config.PrecurseLength = 5           // Shorter lookahead
	config.MinWordsForEarlyPasses = 8   // Process shorter lines earlier
	config.MaxPasses = 5                // Fewer passes
	config.SplitHyphenatedWords = false // Skip hyphenation processing
	return config
}

// AccurateConfig returns a configuration optimized for accuracy over speed.
// Best for complex documents or noisy OCR output where accuracy is critical.
//
// Use cases:
//   - Poor quality OCR with many errors
//   - Historical documents
//   - Complex multi-column layouts
//   - Final/production processing
func AccurateConfig() SorterConfig {
	config := DefaultConfig()
	config.MaxPermutations = 5000000   // Allow more exploration
	config.PrecurseLength = 10         // Longer lookahead
	config.MinWordsForEarlyPasses = 12 // Prioritize long lines
	config.MaxPasses = 12              // More passes
	config.MaxWordDistance = 0.6       // Allow larger gaps
	config.PermutationsPerPass = 50000 // Higher per-pass limit
	return config
}

// CJKConfig returns a configuration optimized for Chinese, Japanese, and Korean text.
// Accounts for character-based writing systems and different spacing patterns.
// Note: Uses vertical reading order by default. For modern horizontal CJK text,
// override ReadingOrder to HorizontalLTR_TTB.
//
// Use cases:
//   - Traditional Chinese documents (vertical)
//   - Japanese documents (vertical)
//   - Korean documents
//   - Mixed CJK/English content
func CJKConfig() SorterConfig {
	config := DefaultConfig()
	config.MaxPermutations = 2000000  // More permutations (no spaces = longer tokens)
	config.PrecurseLength = 10        // Longer lookahead for character sequences
	config.MinWordsForEarlyPasses = 5 // CJK "words" can be single characters
	config.MaxPasses = 10
	config.MaxWordDistance = 0.4 // Tighter distance (characters closer together)
	config.SplitHyphenatedWords = false
	config.PermutationsPerPass = 30000
	config.ReadingOrder = VerticalTTB_RTL // Traditional vertical text
	config.ColumnJumpPenalty = 30.0       // Higher penalty for character-level text
	return config
}

// LargeDocumentConfig returns a configuration optimized for documents with 100+ lines.
// Balances thoroughness with practical time limits.
//
// Use cases:
//   - Full newspaper pages
//   - Long articles
//   - Multi-page documents
//   - Books or reports
func LargeDocumentConfig() SorterConfig {
	config := DefaultConfig()
	config.MinWordsForEarlyPasses = 12 // Process longer lines first
	config.MaxPasses = 10              // More passes for complexity
	config.PermutationsPerPass = 15000 // Moderate per-pass limit
	return config
}

// NoisyOCRConfig returns a configuration optimized for poor-quality OCR output.
//
// Use cases:
//   - Low-resolution scans
//   - Degraded source material
//   - Old or damaged documents
//   - Poor lighting conditions
func NoisyOCRConfig() SorterConfig {
	config := DefaultConfig()
	config.MaxPermutations = 2000000
	config.PrecurseLength = 10
	config.MinWordsForEarlyPasses = 10
	config.MaxPasses = 10
	config.MaxWordDistance = 0.6 // Allow larger gaps
	config.PermutationsPerPass = 30000
	return config
}

// MultiColumnConfig returns a configuration optimized for multi-column layouts.
// Handles newspaper-style columns and complex spatial arrangements.
//
// Use cases:
//   - Newspapers
//   - Magazines
//   - Academic papers with columns
//   - Brochures and pamphlets
func MultiColumnConfig() SorterConfig {
	config := DefaultConfig()
	config.MaxPermutations = 1500000
	config.MinWordsForEarlyPasses = 8 // Columns may have shorter lines
	config.MaxPasses = 10
	config.MaxWordDistance = 0.4 // Tighter (columns are close)
	config.PermutationsPerPass = 25000
	config.ColumnJumpPenalty = 30.0 // Higher penalty to prevent premature column jumps
	return config
}

// RTLConfig returns a configuration optimized for right-to-left languages.
// Uses right-to-left reading order for Arabic, Hebrew, Persian, etc.
//
// Use cases:
//   - Arabic documents
//   - Hebrew documents
//   - Persian/Farsi documents
//   - Urdu documents
func RTLConfig() SorterConfig {
	config := DefaultConfig()
	config.MaxPermutations = 1000000
	config.MinWordsForEarlyPasses = 10
	config.SplitHyphenatedWords = false
	config.PermutationsPerPass = 20000
	config.ReadingOrder = HorizontalRTL_TTB // Right-to-left reading
	return config
}
