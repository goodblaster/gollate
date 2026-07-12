package main

import (
	"fmt"
	"log"

	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// This example demonstrates how to use custom configuration for the sorting algorithm.
// You can tune parameters for performance, accuracy, or specific document types.
func main() {
	canonicalText := []string{
		"First line of text",
		"Second line of text",
		"Third line of text",
	}

	ocrJSON := `[
		{"text": "First line of text", "confidence": 0.95, "rect": {"top": 0.1, "left": 0.1, "width": 0.3, "height": 0.05}, "words": [
			{"text": "First", "top": 0.1, "left": 0.1, "width": 0.08, "height": 0.05},
			{"text": "line", "top": 0.1, "left": 0.19, "width": 0.07, "height": 0.05},
			{"text": "of", "top": 0.1, "left": 0.27, "width": 0.04, "height": 0.05},
			{"text": "text", "top": 0.1, "left": 0.32, "width": 0.07, "height": 0.05}
		]},
		{"text": "Second line of text", "confidence": 0.93, "rect": {"top": 0.2, "left": 0.1, "width": 0.32, "height": 0.05}, "words": [
			{"text": "Second", "top": 0.2, "left": 0.1, "width": 0.1, "height": 0.05},
			{"text": "line", "top": 0.2, "left": 0.21, "width": 0.07, "height": 0.05},
			{"text": "of", "top": 0.2, "left": 0.29, "width": 0.04, "height": 0.05},
			{"text": "text", "top": 0.2, "left": 0.34, "width": 0.07, "height": 0.05}
		]},
		{"text": "Third line of text", "confidence": 0.96, "rect": {"top": 0.3, "left": 0.1, "width": 0.3, "height": 0.05}, "words": [
			{"text": "Third", "top": 0.3, "left": 0.1, "width": 0.08, "height": 0.05},
			{"text": "line", "top": 0.3, "left": 0.19, "width": 0.07, "height": 0.05},
			{"text": "of", "top": 0.3, "left": 0.27, "width": 0.04, "height": 0.05},
			{"text": "text", "top": 0.3, "left": 0.32, "width": 0.07, "height": 0.05}
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
		log.Fatalf("Failed to parse: %v", err)
	}

	// Create custom configuration
	config := sorters.SorterConfig{
		MaxPermutations:        1000000, // Allow more permutations for complex documents
		PrecurseLength:         10,      // Analyze more words for better starting point
		MinWordsForEarlyPasses: 12,      // Process shorter lines earlier
		MaxPasses:              10,      // Allow more passes
		MaxWordDistance:        0.6,     // Allow slightly larger distances
		SplitHyphenatedWords:   true,    // Split hyphenated words
		RotationOptimization:   true,    // Use rotation optimization
		PermutationsPerPass:    20000,   // Higher limit per pass
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	// Create sorter with custom configuration
	sorter := sorters.NewOcrSorterWithConfig(
		request.Blocks(),
		request.Lines,
		logger.NewLogos(),
		config,
	)

	// Run the sort
	if _, err := sorter.Sort(); err != nil {
		log.Fatalf("Sort failed: %v", err)
	}

	// Print results
	fmt.Println("Sorted text:")
	for _, line := range sorter.SortedLines() {
		fmt.Println(line)
	}

	// Print detailed metrics
	fmt.Println("\nDetailed Metrics:")
	metrics := sorter.Metrics()
	fmt.Printf("├─ Passes: %d / %d max\n", metrics.PassesCompleted, config.MaxPasses)
	fmt.Printf("├─ Lines found: %d\n", metrics.LinesFound)
	fmt.Printf("├─ Lines split: %d\n", metrics.LinesSplit)
	fmt.Printf("├─ Leftover blocks: %d\n", metrics.LeftoverBlocks)
	fmt.Printf("├─ Permutations: %d / %d max\n", metrics.TotalPermutationsExplored, config.MaxPermutations)
	fmt.Printf("└─ Time: %v\n", metrics.ElapsedTime)
}
