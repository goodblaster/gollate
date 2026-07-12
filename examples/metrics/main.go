package main

import (
	"fmt"
	"log"
	"time"

	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// This example demonstrates how to collect and analyze performance metrics
// from the sorting operation. Metrics are useful for:
//   - Debugging difficult documents
//   - Performance tuning
//   - Understanding algorithm behavior
//   - Monitoring production systems
func main() {
	canonicalText := []string{
		"The quick brown fox jumps over the lazy dog",
		"Pack my box with five dozen liquor jugs",
		"How vexingly quick daft zebras jump",
	}

	ocrJSON := `[
		{"text": "The quick brown fox jumps over the lazy dog", "confidence": 0.96, "rect": {"top": 0.1, "left": 0.1, "width": 0.6, "height": 0.04}, "words": [
			{"text": "The", "top": 0.1, "left": 0.1, "width": 0.05, "height": 0.04},
			{"text": "quick", "top": 0.1, "left": 0.16, "width": 0.07, "height": 0.04},
			{"text": "brown", "top": 0.1, "left": 0.24, "width": 0.07, "height": 0.04},
			{"text": "fox", "top": 0.1, "left": 0.32, "width": 0.05, "height": 0.04},
			{"text": "jumps", "top": 0.1, "left": 0.38, "width": 0.07, "height": 0.04},
			{"text": "over", "top": 0.1, "left": 0.46, "width": 0.06, "height": 0.04},
			{"text": "the", "top": 0.1, "left": 0.53, "width": 0.05, "height": 0.04},
			{"text": "lazy", "top": 0.1, "left": 0.59, "width": 0.06, "height": 0.04},
			{"text": "dog", "top": 0.1, "left": 0.66, "width": 0.05, "height": 0.04}
		]},
		{"text": "Pack my box with five dozen liquor jugs", "confidence": 0.94, "rect": {"top": 0.2, "left": 0.1, "width": 0.55, "height": 0.04}, "words": [
			{"text": "Pack", "top": 0.2, "left": 0.1, "width": 0.06, "height": 0.04},
			{"text": "my", "top": 0.2, "left": 0.17, "width": 0.04, "height": 0.04},
			{"text": "box", "top": 0.2, "left": 0.22, "width": 0.05, "height": 0.04},
			{"text": "with", "top": 0.2, "left": 0.28, "width": 0.06, "height": 0.04},
			{"text": "five", "top": 0.2, "left": 0.35, "width": 0.06, "height": 0.04},
			{"text": "dozen", "top": 0.2, "left": 0.42, "width": 0.07, "height": 0.04},
			{"text": "liquor", "top": 0.2, "left": 0.5, "width": 0.08, "height": 0.04},
			{"text": "jugs", "top": 0.2, "left": 0.59, "width": 0.06, "height": 0.04}
		]},
		{"text": "How vexingly quick daft zebras jump", "confidence": 0.92, "rect": {"top": 0.3, "left": 0.1, "width": 0.5, "height": 0.04}, "words": [
			{"text": "How", "top": 0.3, "left": 0.1, "width": 0.05, "height": 0.04},
			{"text": "vexingly", "top": 0.3, "left": 0.16, "width": 0.11, "height": 0.04},
			{"text": "quick", "top": 0.3, "left": 0.28, "width": 0.07, "height": 0.04},
			{"text": "daft", "top": 0.3, "left": 0.36, "width": 0.06, "height": 0.04},
			{"text": "zebras", "top": 0.3, "left": 0.43, "width": 0.08, "height": 0.04},
			{"text": "jump", "top": 0.3, "left": 0.52, "width": 0.06, "height": 0.04}
		]}
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

	// Create custom config to see metrics variation
	config := sorters.SorterConfig{
		MaxPermutations:        100000,
		PrecurseLength:         8,
		MinWordsForEarlyPasses: 5,
		MaxPasses:              8,
		MaxWordDistance:        0.5,
		SplitHyphenatedWords:   true,
		RotationOptimization:   true,
		PermutationsPerPass:    10000,
	}

	sorter := sorters.NewOcrSorterWithConfig(
		request.Blocks(),
		request.Lines,
		logger.NewLogos(),
		config,
	)

	if _, err := sorter.Sort(); err != nil {
		log.Fatalf("Sort failed: %v", err)
	}

	// Get and analyze metrics
	metrics := sorter.Metrics()

	fmt.Println("Performance Metrics Report")
	fmt.Println("==========================")
	fmt.Println()

	// Efficiency metrics
	fmt.Println("Efficiency:")
	fmt.Printf("  Passes completed:       %d / %d (%.1f%%)\n",
		metrics.PassesCompleted, config.MaxPasses,
		float64(metrics.PassesCompleted)/float64(config.MaxPasses)*100)
	fmt.Printf("  Permutations explored:  %d / %d (%.1f%%)\n",
		metrics.TotalPermutationsExplored, config.MaxPermutations,
		float64(metrics.TotalPermutationsExplored)/float64(config.MaxPermutations)*100)
	fmt.Printf("  Time per pass:          %v\n",
		metrics.ElapsedTime/time.Duration(metrics.PassesCompleted))
	fmt.Println()

	// Success metrics
	fmt.Println("Success Rate:")
	totalLines := len(canonicalText)
	fmt.Printf("  Lines found:            %d / %d (%.1f%%)\n",
		metrics.LinesFound, totalLines,
		float64(metrics.LinesFound)/float64(totalLines)*100)
	fmt.Printf("  Lines split:            %d\n", metrics.LinesSplit)
	fmt.Printf("  Leftover blocks:        %d\n", metrics.LeftoverBlocks)
	fmt.Println()

	// Performance summary
	fmt.Println("Summary:")
	fmt.Printf("  Total time:             %v\n", metrics.ElapsedTime)
	fmt.Printf("  Time per line:          %v\n",
		metrics.ElapsedTime/time.Duration(totalLines))
	fmt.Printf("  Perms per millisecond:  %.0f\n",
		float64(metrics.TotalPermutationsExplored)/float64(metrics.ElapsedTime.Milliseconds()))
	fmt.Println()

	// Recommendations based on metrics
	fmt.Println("Analysis:")
	if float64(metrics.TotalPermutationsExplored)/float64(config.MaxPermutations) > 0.8 {
		fmt.Println("  ⚠ High permutation usage - consider increasing MaxPermutations")
	} else {
		fmt.Println("  ✓ Permutation limit is adequate")
	}

	if metrics.PassesCompleted >= config.MaxPasses {
		fmt.Println("  ⚠ Reached max passes - some lines may not be found")
	} else {
		fmt.Println("  ✓ Completed before max passes")
	}

	if metrics.LinesSplit > 0 {
		fmt.Printf("  ℹ %d lines required splitting\n", metrics.LinesSplit)
	}

	if metrics.LeftoverBlocks > 0 {
		fmt.Printf("  ℹ %d blocks not matched to canonical text\n", metrics.LeftoverBlocks)
	}
}
