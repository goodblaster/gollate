package sorters

// Algorithm constants for the OCR sorting process.
// These values tune the balance between accuracy and performance.
const (
	// EarlyPassThreshold is the maximum pass number for early-pass optimizations.
	// Early passes (0-3) focus on longer, more distinctive lines.
	EarlyPassThreshold = 3

	// MinChainLength is the minimum number of words required for pathfinding.
	// Lines with fewer words are skipped as they lack enough context.
	MinChainLength = 2

	// SingleWordChainLength represents a chain with only one word.
	// Single-word lines are skipped as they cannot form paths.
	SingleWordChainLength = 1

	// HoleNode marks a wildcard hole in Path.Nodes: a canonical word that
	// pathfinding bridged without consuming a block (EnableChainHoles).
	// Consumers of Path.Nodes must skip negative indices.
	HoleNode = -1

	// HoleGapAllowancePerWord is the extra primary-axis gap (normalized page
	// units) tolerated per bridged hole when validating the step that lands
	// after one or more skipped words.
	HoleGapAllowancePerWord = 0.2

	// HoleMinConfirmLength is the minimum canonical word length (runes) for
	// text-confirming a gap-fill claim by edit distance. Shorter words -
	// including single-character CJK tokens - are confirmed by spatial
	// containment alone.
	HoleMinConfirmLength = 4
)
