package main

import (
	"fmt"
	"log"

	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/sorters"
)

// This example demonstrates basic usage of gollate with Apple Vision OCR.
// It shows how to sort OCR blocks to match canonical text order.
func main() {
	// Canonical text - the expected reading order
	canonicalText := []string{
		"What is Lorem Ipsum?",
		"Lorem Ipsum is simply dummy text of the printing and typesetting industry.",
	}

	// Simulated Apple Vision OCR output
	// In reality, this would come from running OCR on an image
	ocrJSON := `[
		{
			"text": "What is",
			"confidence": 0.95,
			"rect": {"top": 0.1, "left": 0.1, "width": 0.15, "height": 0.05},
			"words": [
				{"text": "What", "top": 0.1, "left": 0.1, "width": 0.07, "height": 0.05},
				{"text": "is", "top": 0.1, "left": 0.18, "width": 0.04, "height": 0.05}
			]
		},
		{
			"text": "Lorem Ipsum?",
			"confidence": 0.93,
			"rect": {"top": 0.1, "left": 0.26, "width": 0.25, "height": 0.05},
			"words": [
				{"text": "Lorem", "top": 0.1, "left": 0.26, "width": 0.12, "height": 0.05},
				{"text": "Ipsum?", "top": 0.1, "left": 0.39, "width": 0.12, "height": 0.05}
			]
		},
		{
			"text": "Lorem Ipsum is simply dummy text",
			"confidence": 0.96,
			"rect": {"top": 0.2, "left": 0.1, "width": 0.5, "height": 0.05},
			"words": [
				{"text": "Lorem", "top": 0.2, "left": 0.1, "width": 0.1, "height": 0.05},
				{"text": "Ipsum", "top": 0.2, "left": 0.21, "width": 0.1, "height": 0.05},
				{"text": "is", "top": 0.2, "left": 0.32, "width": 0.04, "height": 0.05},
				{"text": "simply", "top": 0.2, "left": 0.37, "width": 0.1, "height": 0.05},
				{"text": "dummy", "top": 0.2, "left": 0.48, "width": 0.1, "height": 0.05},
				{"text": "text", "top": 0.2, "left": 0.59, "width": 0.08, "height": 0.05}
			]
		},
		{
			"text": "of the printing and",
			"confidence": 0.94,
			"rect": {"top": 0.25, "left": 0.1, "width": 0.35, "height": 0.05},
			"words": [
				{"text": "of", "top": 0.25, "left": 0.1, "width": 0.04, "height": 0.05},
				{"text": "the", "top": 0.25, "left": 0.15, "width": 0.06, "height": 0.05},
				{"text": "printing", "top": 0.25, "left": 0.22, "width": 0.14, "height": 0.05},
				{"text": "and", "top": 0.25, "left": 0.37, "width": 0.06, "height": 0.05}
			]
		},
		{
			"text": "typesetting industry.",
			"confidence": 0.92,
			"rect": {"top": 0.3, "left": 0.1, "width": 0.35, "height": 0.05},
			"words": [
				{"text": "typesetting", "top": 0.3, "left": 0.1, "width": 0.2, "height": 0.05},
				{"text": "industry.", "top": 0.3, "left": 0.31, "width": 0.14, "height": 0.05}
			]
		}
	]`

	// Create the sort request
	request := api.SortRequest{
		Engine:     "apple",
		Lines:      canonicalText,
		InputJson:  ocrJSON,
		PageWidth:  1920,
		PageHeight: 1080,
	}

	// Parse the OCR data
	if err := request.Parse(); err != nil {
		log.Fatalf("Failed to parse OCR data: %v", err)
	}

	// Create sorter with logos logger
	sorter := sorters.NewOcrSorter(
		request.Blocks(),
		request.Lines,
		logger.NewLogos(),
	)

	// Run the sort
	blocks, err := sorter.Sort()
	if err != nil {
		log.Fatalf("Sort failed: %v", err)
	}

	// Print the sorted text
	fmt.Println("Sorted text:")
	fmt.Println("============")
	for _, line := range sorter.SortedLines() {
		fmt.Println(line)
	}

	// Print metrics
	fmt.Println("\nMetrics:")
	fmt.Println("========")
	metrics := sorter.Metrics()
	fmt.Printf("Total blocks: %d\n", len(blocks))
	fmt.Printf("Passes completed: %d\n", metrics.PassesCompleted)
	fmt.Printf("Lines found: %d\n", metrics.LinesFound)
	fmt.Printf("Leftover blocks: %d\n", metrics.LeftoverBlocks)
	fmt.Printf("Permutations explored: %d\n", metrics.TotalPermutationsExplored)
	fmt.Printf("Elapsed time: %v\n", metrics.ElapsedTime)
}
