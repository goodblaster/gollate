package main

import (
	"fmt"
	"log"

	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// This example demonstrates debugging and diagnostics output.
// It shows detailed information about block parsing and sorting internals.
func main() {
	canonicalText := []string{
		"The quick brown fox jumps over the lazy dog",
		"Pack my box with five dozen liquor jugs",
	}

	ocrJSON := `[
		{
			"text": "The quick brown fox jumps over the lazy dog",
			"confidence": 0.75,
			"rect": {"top": 0.1, "left": 0.1, "width": 0.7, "height": 0.04},
			"words": [
				{"text": "The", "top": 0.1, "left": 0.1, "width": 0.05, "height": 0.04},
				{"text": "quick", "top": 0.1, "left": 0.16, "width": 0.07, "height": 0.04},
				{"text": "brown", "top": 0.1, "left": 0.24, "width": 0.07, "height": 0.04},
				{"text": "fox", "top": 0.1, "left": 0.32, "width": 0.05, "height": 0.04},
				{"text": "jumps", "top": 0.1, "left": 0.38, "width": 0.07, "height": 0.04},
				{"text": "over", "top": 0.1, "left": 0.46, "width": 0.06, "height": 0.04},
				{"text": "the", "top": 0.1, "left": 0.53, "width": 0.05, "height": 0.04},
				{"text": "lazy", "top": 0.1, "left": 0.59, "width": 0.06, "height": 0.04},
				{"text": "dog", "top": 0.1, "left": 0.66, "width": 0.05, "height": 0.04}
			]
		},
		{
			"text": "Pack my box with five dozen liquor jugs",
			"confidence": 0.95,
			"rect": {"top": 0.2, "left": 0.1, "width": 0.55, "height": 0.04},
			"words": [
				{"text": "Pack", "top": 0.2, "left": 0.1, "width": 0.06, "height": 0.04},
				{"text": "my", "top": 0.2, "left": 0.17, "width": 0.04, "height": 0.04},
				{"text": "box", "top": 0.2, "left": 0.22, "width": 0.05, "height": 0.04},
				{"text": "with", "top": 0.2, "left": 0.28, "width": 0.06, "height": 0.04},
				{"text": "five", "top": 0.2, "left": 0.35, "width": 0.06, "height": 0.04},
				{"text": "dozen", "top": 0.2, "left": 0.42, "width": 0.07, "height": 0.04},
				{"text": "liquor", "top": 0.2, "left": 0.5, "width": 0.08, "height": 0.04},
				{"text": "jugs", "top": 0.2, "left": 0.59, "width": 0.06, "height": 0.04}
			]
		}
	]`

	request := api.SortRequest{
		Engine:     "apple",
		Lines:      canonicalText,
		InputJson:  ocrJSON,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	if err := request.Parse(); err != nil {
		log.Fatalf("Parse failed: %v", err)
	}

	blocks := request.Blocks()
	fmt.Printf("Blocks parsed: %d\n", len(blocks))
	fmt.Printf("First 5 blocks:\n")
	for i := 0; i < 5 && i < len(blocks); i++ {
		fmt.Printf("  [%d]: Text='%s', NormedText='%s'\n", i, blocks[i].Text, blocks[i].NormedText)
	}

	config := sorters.SorterConfig{
		MaxPermutations:        100000,
		PrecurseLength:         8,
		MinWordsForEarlyPasses: 5, // Lower than default!
		MaxPasses:              8,
		MaxWordDistance:        0.5,
		SplitHyphenatedWords:   true,
		RotationOptimization:   true,
		PermutationsPerPass:    10000,
	}
	sorter := sorters.NewOcrSorterWithConfig(blocks, canonicalText, logger.NewLogos(), config)

	// Access internal fields for debugging (this is hacky but useful for diagnosis)
	fmt.Printf("\nCanonical lines: %d\n", len(canonicalText))
	fmt.Printf("First canonical line: '%s'\n", canonicalText[0])

	_, err := sorter.Sort()
	if err != nil {
		log.Fatalf("Sort failed: %v", err)
	}

	metrics := sorter.Metrics()
	fmt.Printf("\nResults:\n")
	fmt.Printf("  Lines found: %d / %d\n", metrics.LinesFound, len(canonicalText))
	fmt.Printf("  Permutations: %d\n", metrics.TotalPermutationsExplored)
	fmt.Printf("  Passes: %d\n", metrics.PassesCompleted)
}
