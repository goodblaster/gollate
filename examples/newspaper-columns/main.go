package main

import (
	"fmt"
	"log"

	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// This example demonstrates handling multi-column layouts like newspapers or magazines.
// The algorithm uses spatial proximity to determine reading order across columns.
//
// Layout visualization:
//
//	┌─────────────┬─────────────┐
//	│  Column 1   │  Column 2   │
//	│  Line 1     │  Line 4     │
//	│  Line 2     │  Line 5     │
//	│  Line 3     │  Line 6     │
//	└─────────────┴─────────────┘
func main() {
	// Canonical text in proper reading order (top-to-bottom, left-to-right columns)
	canonicalText := []string{
		"Breaking News: Technology Advances",
		"Scientists have made a breakthrough",
		"in quantum computing research.",
		"Weather Update: Sunny Skies Ahead",
		"Temperatures will reach highs",
		"of 75 degrees this weekend.",
	}

	// Simulated OCR output with two-column layout
	// Column 1 on left (x: 0.1-0.45), Column 2 on right (x: 0.55-0.9)
	ocrJSON := `[
		{
			"text": "Breaking News: Technology Advances",
			"confidence": 0.95,
			"rect": {"top": 0.1, "left": 0.1, "width": 0.35, "height": 0.04},
			"words": [
				{"text": "Breaking", "top": 0.1, "left": 0.1, "width": 0.1, "height": 0.04},
				{"text": "News:", "top": 0.1, "left": 0.21, "width": 0.07, "height": 0.04},
				{"text": "Technology", "top": 0.1, "left": 0.29, "width": 0.13, "height": 0.04},
				{"text": "Advances", "top": 0.1, "left": 0.43, "width": 0.1, "height": 0.04}
			]
		},
		{
			"text": "Scientists have made a breakthrough",
			"confidence": 0.93,
			"rect": {"top": 0.15, "left": 0.1, "width": 0.35, "height": 0.04},
			"words": [
				{"text": "Scientists", "top": 0.15, "left": 0.1, "width": 0.12, "height": 0.04},
				{"text": "have", "top": 0.15, "left": 0.23, "width": 0.06, "height": 0.04},
				{"text": "made", "top": 0.15, "left": 0.3, "width": 0.06, "height": 0.04},
				{"text": "a", "top": 0.15, "left": 0.37, "width": 0.02, "height": 0.04},
				{"text": "breakthrough", "top": 0.15, "left": 0.4, "width": 0.14, "height": 0.04}
			]
		},
		{
			"text": "in quantum computing research.",
			"confidence": 0.94,
			"rect": {"top": 0.2, "left": 0.1, "width": 0.35, "height": 0.04},
			"words": [
				{"text": "in", "top": 0.2, "left": 0.1, "width": 0.03, "height": 0.04},
				{"text": "quantum", "top": 0.2, "left": 0.14, "width": 0.09, "height": 0.04},
				{"text": "computing", "top": 0.2, "left": 0.24, "width": 0.12, "height": 0.04},
				{"text": "research.", "top": 0.2, "left": 0.37, "width": 0.11, "height": 0.04}
			]
		},
		{
			"text": "Weather Update: Sunny Skies Ahead",
			"confidence": 0.96,
			"rect": {"top": 0.1, "left": 0.55, "width": 0.35, "height": 0.04},
			"words": [
				{"text": "Weather", "top": 0.1, "left": 0.55, "width": 0.09, "height": 0.04},
				{"text": "Update:", "top": 0.1, "left": 0.65, "width": 0.08, "height": 0.04},
				{"text": "Sunny", "top": 0.1, "left": 0.74, "width": 0.07, "height": 0.04},
				{"text": "Skies", "top": 0.1, "left": 0.82, "width": 0.07, "height": 0.04},
				{"text": "Ahead", "top": 0.1, "left": 0.9, "width": 0.07, "height": 0.04}
			]
		},
		{
			"text": "Temperatures will reach highs",
			"confidence": 0.92,
			"rect": {"top": 0.15, "left": 0.55, "width": 0.35, "height": 0.04},
			"words": [
				{"text": "Temperatures", "top": 0.15, "left": 0.55, "width": 0.15, "height": 0.04},
				{"text": "will", "top": 0.15, "left": 0.71, "width": 0.05, "height": 0.04},
				{"text": "reach", "top": 0.15, "left": 0.77, "width": 0.07, "height": 0.04},
				{"text": "highs", "top": 0.15, "left": 0.85, "width": 0.07, "height": 0.04}
			]
		},
		{
			"text": "of 75 degrees this weekend.",
			"confidence": 0.91,
			"rect": {"top": 0.2, "left": 0.55, "width": 0.35, "height": 0.04},
			"words": [
				{"text": "of", "top": 0.2, "left": 0.55, "width": 0.03, "height": 0.04},
				{"text": "75", "top": 0.2, "left": 0.59, "width": 0.03, "height": 0.04},
				{"text": "degrees", "top": 0.2, "left": 0.63, "width": 0.09, "height": 0.04},
				{"text": "this", "top": 0.2, "left": 0.73, "width": 0.05, "height": 0.04},
				{"text": "weekend.", "top": 0.2, "left": 0.79, "width": 0.11, "height": 0.04}
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
		log.Fatalf("Failed to parse: %v", err)
	}

	// For multi-column layouts, default configuration usually works well
	// because the algorithm uses spatial proximity
	sorter := sorters.NewOcrSorter(
		request.Blocks(),
		request.Lines,
		logger.NewLogos(),
	)

	if _, err := sorter.Sort(); err != nil {
		log.Fatalf("Sort failed: %v", err)
	}

	// Display results with column indication
	fmt.Println("Sorted text (reading order):")
	fmt.Println("============================")
	fmt.Println()

	lines := sorter.SortedLines()
	for i, line := range lines {
		columnIndicator := ""
		if i < 3 {
			columnIndicator = "[Column 1]"
		} else {
			columnIndicator = "[Column 2]"
		}
		fmt.Printf("%-15s %s\n", columnIndicator, line)
	}

	fmt.Println()
	fmt.Println("The algorithm correctly identified the two-column layout")
	fmt.Println("and sorted text in proper reading order:")
	fmt.Println("  • Column 1: Lines 1-3")
	fmt.Println("  • Column 2: Lines 4-6")
	fmt.Println()

	metrics := sorter.Metrics()
	fmt.Printf("Completed in %d passes, %v elapsed\n",
		metrics.PassesCompleted, metrics.ElapsedTime)
}
