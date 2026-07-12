package sorters

import (
	"fmt"
	"time"
)

// SorterConfig contains configurable parameters for the sorting algorithm.
type SorterConfig struct {
	// MaxPermutations limits recursion depth to prevent exponential blowup.
	// Higher values allow exploring more complex paths but increase computation time.
	// Default: 500000
	MaxPermutations int

	// PrecurseLength is the number of words to analyze before full recursion.
	// This optimization helps identify the best starting point for the search.
	// Set to 0 to disable precurse optimization.
	// Default: 8
	PrecurseLength int

	// MinWordsForEarlyPasses determines the minimum line length to process in early passes.
	// Early passes ignore shorter lines to focus computational budget on distinctive text.
	// Default: 16
	MinWordsForEarlyPasses int

	// MaxPasses is the maximum number of sorting passes to attempt.
	// Multiple passes allow handling split lines and complex documents.
	// Default: 8
	MaxPasses int

	// MaxWordDistance is the maximum spatial distance between consecutive words.
	// Word pairs farther apart than this threshold are rejected as invalid paths.
	// Default: 0.5 (50% of page dimensions)
	MaxWordDistance float64

	// SplitHyphenatedWords splits hyphenated words into separate blocks during normalization.
	// This improves matching when OCR engines handle hyphens inconsistently.
	// Default: true
	SplitHyphenatedWords bool

	// RotationOptimization reorders search candidates to try spatially nearest blocks first.
	// This dramatically reduces the number of paths explored.
	// Default: true
	RotationOptimization bool

	// PermutationsPerPass sets the permutation limit for each individual pass.
	// If 0, uses MaxPermutations for all passes.
	// Default: 10000
	PermutationsPerPass int

	// ReadingOrder specifies the text flow direction.
	// This affects distance calculations and block ordering.
	// Common values:
	//   - HorizontalLTR_TTB: English, most European languages (default)
	//   - HorizontalRTL_TTB: Arabic, Hebrew
	//   - VerticalTTB_RTL: Traditional Chinese, Japanese
	//   - VerticalTTB_LTR: Mongolian
	// Default: HorizontalLTR_TTB
	ReadingOrder ReadingOrder

	// ColumnJumpPenalty is the base penalty for jumping to the next column.
	// Higher values discourage column jumps, preferring to stay in the same column.
	// This is critical for multi-column layouts (newspapers, magazines).
	// Language-specific tuning may be needed:
	//   - English/European: 25.0 (word-based, wider spacing)
	//   - CJK: May need different value (character-based, denser text)
	// Default: 25.0
	ColumnJumpPenalty float64

	// OCR-error-tolerance mechanisms.
	// All default off in DefaultConfig; ConfigForLanguage enables the
	// combination each script measurably benefits from. These describe
	// algorithm strategy, not page layout, so they remain compatible with
	// the language-only-hint rule (like RotationOptimization).

	// EnableWrapBridging lets pathfinding follow a canonical line across a
	// visual line wrap. The distance function already classifies legitimate
	// wraps (isWrappedToNextLine) but their cost (BaseLineWrap 1.0 + gap)
	// always exceeds MaxWordDistance, so without this flag no multi-visual-
	// line path can ever complete (TESTING.md issue #3). The wrap cost still
	// lands in path length, so compact paths win. Experimental.
	// Default: false
	EnableWrapBridging bool

	// EnableChainHoles keeps a line whole when a small fraction of its words
	// are missing from OCR: missing words become wildcard slots that
	// pathfinding bridges with HolePathPenalty, instead of splitting the line.
	// After a path is accepted, an unclaimed block spatially inside the gap
	// may be claimed with exact-text confirmation (spatial containment alone
	// for short/CJK tokens); misread words are line repair's job.
	// Holes across a wrap require EnableWrapBridging.
	// Default: false
	EnableChainHoles bool

	// MaxHoleFraction is the maximum fraction of a line's words that may be
	// holes before the line is split as it is today.
	// Default: 0.2
	MaxHoleFraction float64

	// HolePathPenalty is the path-length cost of bridging one hole.
	// Default: 1.0
	HolePathPenalty float64

	// EnableShortLineAnchoring fixes TESTING.md issues #1+#2 together (each
	// alone is measured-harmful or incomplete): the pass loop no longer
	// exits before the early-pass filter relaxes (so short lines actually
	// get attempted), and near-tied candidate paths for short lines
	// tie-break by spatial proximity to the matched blocks of the line's
	// nearest canonical neighbors (so duplicate short lines like "Learn
	// more"/"Buy" pick the instance in the right region). Experimental.
	// Default: false
	EnableShortLineAnchoring bool

	// AnchorTieEpsilon is how close (in path length) a candidate path must
	// be to the shortest one to compete on anchor proximity instead.
	// Default: 0.5
	AnchorTieEpsilon float64

	// EnableReconciliationPass runs a post-pass over unfound line fragments:
	// anchored by where the fragment's canonical neighbors landed, it searches
	// unclaimed blocks within ReconSpatialWindow, requiring at least
	// ReconMinExactAnchors exact word matches. Experimental; depends on
	// context-anchoring work (TESTING.md issues #1/#2).
	// Default: false
	EnableReconciliationPass bool

	// ReconSpatialWindow is the search radius (normalized page coordinates)
	// around the neighbor anchor for the reconciliation pass.
	// Default: 0.15
	ReconSpatialWindow float64

	// ReconMinExactAnchors is the minimum number of exactly-matched words a
	// fragment must contribute for a reconciliation match to be accepted.
	// At 1, single-word lines ("Buy") can be rescued when spatially pinned
	// by their anchors.
	// Default: 2
	ReconMinExactAnchors int

	// DisableVerticalDetection turns off orientation inference (vertical.go):
	// when a clear majority of the OCR engine's own lines flow vertically,
	// a horizontal reading order is switched to VerticalTTB_RTL (tategaki).
	// Inert without line data. Never switches vertical -> horizontal.
	DisableVerticalDetection bool

	// DisableLineRepair turns off the line-repair preparation step
	// (linerepair.go): when blocks carry the OCR engine's line grouping,
	// misread tokens flanked by exactly-matched neighbors are rekeyed to
	// the canonical word at that position. Automatically inert when blocks
	// have no LineId. Repairs never change Block.Text.
	//
	// On by default (noise fixtures +11.4/+3.2, apple.com benchmark +0.7
	// after the U+2019 normalization fix); ConfigForLanguage disables it
	// for Arabic and CJK, where it measured net-negative on noisy
	// Tesseract output.
	DisableLineRepair bool
}

// DefaultConfig returns a SorterConfig with recommended default values.
func DefaultConfig() SorterConfig {
	return SorterConfig{
		MaxPermutations:        500000,
		PrecurseLength:         8,
		MinWordsForEarlyPasses: 16,
		MaxPasses:              8,
		MaxWordDistance:        0.5,
		SplitHyphenatedWords:   true,
		RotationOptimization:   true,
		PermutationsPerPass:    10000,
		ReadingOrder:           HorizontalLTR_TTB,
		ColumnJumpPenalty:      25.0,
		// OCR-error-tolerance mechanisms - off here; ConfigForLanguage
		// enables the measured-best combination per script.
		EnableWrapBridging:       false,
		EnableChainHoles:         false,
		MaxHoleFraction:          0.2,
		HolePathPenalty:          1.0,
		EnableShortLineAnchoring: false,
		AnchorTieEpsilon:         0.5,
		EnableReconciliationPass: false,
		ReconSpatialWindow:       0.15,
		ReconMinExactAnchors:     2,
		DisableLineRepair:        false,
	}
}

// Validate checks that configuration values are reasonable.
func (c SorterConfig) Validate() error {
	if c.MaxPermutations < 1 {
		return &ConfigError{Field: "MaxPermutations", Value: c.MaxPermutations, Message: "must be at least 1"}
	}
	if c.PrecurseLength < 0 {
		return &ConfigError{Field: "PrecurseLength", Value: c.PrecurseLength, Message: "cannot be negative"}
	}
	if c.MinWordsForEarlyPasses < 0 {
		return &ConfigError{Field: "MinWordsForEarlyPasses", Value: c.MinWordsForEarlyPasses, Message: "cannot be negative"}
	}
	if c.MaxPasses < 1 {
		return &ConfigError{Field: "MaxPasses", Value: c.MaxPasses, Message: "must be at least 1"}
	}
	if c.MaxWordDistance <= 0 {
		return &ConfigError{Field: "MaxWordDistance", Value: c.MaxWordDistance, Message: "must be positive"}
	}
	if c.PermutationsPerPass < 0 {
		return &ConfigError{Field: "PermutationsPerPass", Value: c.PermutationsPerPass, Message: "cannot be negative"}
	}
	if c.ColumnJumpPenalty <= 0 {
		return &ConfigError{Field: "ColumnJumpPenalty", Value: c.ColumnJumpPenalty, Message: "must be positive"}
	}
	// Validate OCR-error-tolerance settings
	if c.MaxHoleFraction < 0 || c.MaxHoleFraction >= 1 {
		return &ConfigError{Field: "MaxHoleFraction", Value: c.MaxHoleFraction, Message: "must be in [0, 1)"}
	}
	if c.HolePathPenalty < 0 {
		return &ConfigError{Field: "HolePathPenalty", Value: c.HolePathPenalty, Message: "cannot be negative"}
	}
	if c.AnchorTieEpsilon < 0 {
		return &ConfigError{Field: "AnchorTieEpsilon", Value: c.AnchorTieEpsilon, Message: "cannot be negative"}
	}
	if c.ReconSpatialWindow < 0 {
		return &ConfigError{Field: "ReconSpatialWindow", Value: c.ReconSpatialWindow, Message: "cannot be negative"}
	}
	if c.ReconMinExactAnchors < 1 {
		return &ConfigError{Field: "ReconMinExactAnchors", Value: c.ReconMinExactAnchors, Message: "must be at least 1"}
	}
	return nil
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Value   any
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("invalid config for %s (value: %v): %s", e.Field, e.Value, e.Message)
}

// SortMetrics contains performance and diagnostic information from a sort operation.
type SortMetrics struct {
	// TotalPermutationsExplored is the total number of path permutations examined.
	TotalPermutationsExplored int

	// PassesCompleted is the number of sorting passes that were executed.
	PassesCompleted int

	// LinesFound is the number of canonical lines successfully matched.
	LinesFound int

	// LinesSplit is the number of lines that were split into smaller segments.
	LinesSplit int

	// ElapsedTime is the total time spent sorting.
	ElapsedTime time.Duration

	// LeftoverBlocks is the number of OCR blocks not matched to any canonical text.
	LeftoverBlocks int

	// HolesBridged is the number of wildcard hole slots in accepted paths
	// (EnableChainHoles). Each bridged hole was subsequently either filled
	// or left empty; HolesBridged = HolesFilled + HolesLeftEmpty.
	HolesBridged int

	// HolesFilled counts bridged holes where an unclaimed block spatially
	// inside the gap passed text confirmation and was claimed.
	HolesFilled int

	// HolesLeftEmpty counts bridged holes with no claimable block; the
	// canonical word is simply absent from the output.
	HolesLeftEmpty int

	// ShortLinesAnchored counts short lines where anchor re-ranking chose a
	// different path than plain shortest-path (EnableShortLineAnchoring).
	ShortLinesAnchored int

	// LinesReconciled counts unfound lines rescued by the reconciliation
	// pass (EnableReconciliationPass).
	LinesReconciled int

	// LineRepairs counts tokens rekeyed by the line-repair preparation step
	// (see DisableLineRepair). Each repair is also recorded in that block's
	// correction metadata.
	LineRepairs int
}
