//go:build darwin

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/imgutil"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
)

func main() {
	var (
		imagePath  = flag.String("image", "", "Path to original image file")
		ocrFile    = flag.String("ocr", "", "Path to OCR JSON file")
		sortedFile = flag.String("sorted", "", "Path to sorted JSON file (if available, uses this instead of re-sorting)")
		textFile   = flag.String("text", "", "Path to canonical text file (optional, for sorting if no sorted file)")
		engine     = flag.String("engine", "apple", "OCR engine used")
		outputPath = flag.String("output", "", "Output path for highlighted image (default: <image>-highlighted.png)")
	)
	flag.Parse()

	if *imagePath == "" || *ocrFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -image <image-file> -ocr <ocr-json> [-text <canonical-text>] [-output <output-file>]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Determine output path
	if *outputPath == "" {
		base := strings.TrimSuffix(*imagePath, filepath.Ext(*imagePath))
		*outputPath = base + "-highlighted.png"
	}

	// Get image dimensions
	width, height, err := imgutil.Size(*imagePath)
	if err != nil {
		panic(fmt.Sprintf("failed to get image size: %v", err))
	}

	fmt.Printf("Processing: %s (%dx%d)\n", *imagePath, width, height)

	// Load OCR JSON
	ocrData, err := os.ReadFile(*ocrFile)
	if err != nil {
		panic(fmt.Sprintf("failed to read OCR file: %v", err))
	}

	// Create sort request to normalize blocks
	var lines []string
	if *textFile != "" {
		textData, err := os.ReadFile(*textFile)
		if err != nil {
			panic(fmt.Sprintf("failed to read text file: %v", err))
		}
		lines = strings.Split(string(textData), "\n")
	} else {
		lines = []string{"dummy line"} // Just to get normalized blocks
	}

	request := &api.SortRequest{
		Engine:     *engine,
		Lines:      lines,
		InputJson:  string(ocrData),
		PageWidth:  width,
		PageHeight: height,
	}

	if err := request.Parse(); err != nil {
		panic(fmt.Sprintf("failed to parse OCR: %v", err))
	}

	// Get blocks - either from sorted JSON, or by sorting, or just normalized
	var blocks []ocr.Block
	var useSortedOrder bool

	if *sortedFile != "" {
		// Read pre-sorted blocks from JSON file
		sortedData, err := os.ReadFile(*sortedFile)
		if err != nil {
			panic(fmt.Sprintf("failed to read sorted file: %v", err))
		}

		var sortResponse api.SortResponse
		if err := json.Unmarshal(sortedData, &sortResponse); err != nil {
			panic(fmt.Sprintf("failed to parse sorted JSON: %v", err))
		}

		blocks = sortResponse.SortedBlocks
		useSortedOrder = true
		fmt.Printf("Loaded %d sorted blocks from %s\n", len(blocks), *sortedFile)
	} else if *textFile != "" {
		// Run the sort to get blocks in canonical order
		// Use the same config as integration tests for consistent results
		config := sorters.SorterConfig{
			MaxPermutations:        10000000, // 10M - early exit on perfect matches makes this safe
			PrecurseLength:         20,       // Longer precurse for better initial positioning
			MinWordsForEarlyPasses: 1,        // Very low threshold for CJK bigrams
			MaxPasses:              20,       // More passes for difficult matching
			MaxWordDistance:        0.5,
			SplitHyphenatedWords:   true,
			RotationOptimization:   true,
			PermutationsPerPass:    2000000, // 2M per pass - early exit makes this efficient
		}

		sorter := sorters.NewOcrSorterWithConfig(
			request.Blocks(),
			request.Lines,
			logger.Noop{}, // Silent logger for highlighting
			config,
		)

		sortedBlocks, err := sorter.Sort()
		if err != nil {
			panic(fmt.Sprintf("failed to sort: %v", err))
		}

		blocks = sortedBlocks
		useSortedOrder = true
		fmt.Printf("Loaded %d sorted blocks\n", len(blocks))
	} else {
		blocks = request.Blocks()
		useSortedOrder = false
		fmt.Printf("Loaded %d blocks\n", len(blocks))
	}

	// Load image
	imgFile, err := os.Open(*imagePath)
	if err != nil {
		panic(fmt.Sprintf("failed to open image: %v", err))
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		panic(fmt.Sprintf("failed to decode image: %v", err))
	}

	// Create output image
	bounds := img.Bounds()
	highlighted := image.NewRGBA(bounds)
	draw.Draw(highlighted, bounds, img, bounds.Min, draw.Src)

	// Define colors for highlighting (cycle through them)
	colors := []color.RGBA{
		{255, 0, 0, 255},   // Red
		{0, 255, 0, 255},   // Green
		{0, 0, 255, 255},   // Blue
		{255, 255, 0, 255}, // Yellow
		{255, 0, 255, 255}, // Magenta
		{0, 255, 255, 255}, // Cyan
		{255, 128, 0, 255}, // Orange
		{128, 0, 255, 255}, // Purple
		{0, 255, 128, 255}, // Spring green
		{255, 0, 128, 255}, // Rose
	}

	// Build paragraph map: block index -> color index
	// If using sorted order, paragraphs are separated by empty blocks (line breaks)
	// If not sorted, fall back to LineId grouping
	blockColors := make(map[int]int)

	if useSortedOrder {
		// Color by paragraph in sorted output (separated by empty blocks)
		colorIndex := 0
		for i, block := range blocks {
			if block.Engine() == "" {
				// Empty block = line separator, next paragraph gets new color
				colorIndex++
				continue
			}
			blockColors[i] = colorIndex
		}
	} else {
		// Fall back to LineId grouping for unsorted blocks
		paragraphColors := make(map[string]int)
		colorIndex := 0
		var lastLineId string

		for i, block := range blocks {
			if block.Text == "" {
				continue
			}

			lineId := block.LineId
			if lineId == "" {
				lineId = fmt.Sprintf("_unknown_%d", block.OriginalIndex)
			}

			// If this is a new LineId, assign a new color
			if _, exists := paragraphColors[lineId]; !exists {
				if lastLineId != "" && lineId != lastLineId {
					colorIndex++
				}
				paragraphColors[lineId] = colorIndex
				lastLineId = lineId
			}
			blockColors[i] = paragraphColors[lineId]
		}
	}

	// Draw bounding boxes and labels
	for i, block := range blocks {
		if block.Text == "" {
			continue // Skip empty blocks (line separators)
		}

		// Convert normalized coordinates to pixel coordinates
		x := int(block.BoundingBox.Left * float64(width))
		y := int(block.BoundingBox.Top * float64(height))
		w := int(block.BoundingBox.Width * float64(width))
		h := int(block.BoundingBox.Height * float64(height))

		// Choose color based on paragraph
		boxColor := colors[blockColors[i]%len(colors)]

		// Draw bounding box (3 pixel thick border)
		drawRect(highlighted, x, y, w, h, boxColor, 3)

		// Draw label with both indexes to show splits clearly
		// If Index != OriginalIndex, this block was renumbered (possibly from splitting)
		if block.Index != block.OriginalIndex {
			label := fmt.Sprintf("%d→%d", block.OriginalIndex, block.Index)
			drawLabel(highlighted, x, y, w, h, label)
		} else {
			label := fmt.Sprintf("%d", block.Index)
			drawLabel(highlighted, x, y, w, h, label)
		}
	}

	// Save highlighted image
	outFile, err := os.Create(*outputPath)
	if err != nil {
		panic(fmt.Sprintf("failed to create output file: %v", err))
	}
	defer outFile.Close()

	if err := png.Encode(outFile, highlighted); err != nil {
		panic(fmt.Sprintf("failed to encode PNG: %v", err))
	}

	fmt.Printf("✓ Saved highlighted image: %s\n", *outputPath)
	fmt.Printf("  Highlighted %d blocks\n", len(blocks))
}

// drawRect draws a rectangle border on the image
func drawRect(img *image.RGBA, x, y, w, h int, c color.RGBA, thickness int) {
	// Top
	for t := 0; t < thickness; t++ {
		for i := x; i < x+w; i++ {
			if y+t >= 0 && y+t < img.Bounds().Dy() && i >= 0 && i < img.Bounds().Dx() {
				img.Set(i, y+t, c)
			}
		}
	}
	// Bottom
	for t := 0; t < thickness; t++ {
		for i := x; i < x+w; i++ {
			if y+h-t-1 >= 0 && y+h-t-1 < img.Bounds().Dy() && i >= 0 && i < img.Bounds().Dx() {
				img.Set(i, y+h-t-1, c)
			}
		}
	}
	// Left
	for t := 0; t < thickness; t++ {
		for j := y; j < y+h; j++ {
			if j >= 0 && j < img.Bounds().Dy() && x+t >= 0 && x+t < img.Bounds().Dx() {
				img.Set(x+t, j, c)
			}
		}
	}
	// Right
	for t := 0; t < thickness; t++ {
		for j := y; j < y+h; j++ {
			if j >= 0 && j < img.Bounds().Dy() && x+w-t-1 >= 0 && x+w-t-1 < img.Bounds().Dx() {
				img.Set(x+w-t-1, j, c)
			}
		}
	}
}

// drawLabel draws white text on black background using TrueType rendering for smooth output
func drawLabel(img *image.RGBA, x, y, boxWidth, boxHeight int, label string) {
	// Parse the Go Regular font for smooth rendering
	ttfFont, err := truetype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}

	// Calculate font size: aim for label to be about 40% of box height
	fontSize := float64(boxHeight) * 0.4
	if fontSize < 10 {
		fontSize = 10
	}
	if fontSize > 24 {
		fontSize = 24
	}

	// Create freetype context for rendering
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(ttfFont)
	c.SetFontSize(fontSize)

	// Measure the text to determine background size
	face := truetype.NewFace(ttfFont, &truetype.Options{
		Size: fontSize,
		DPI:  72,
	})
	defer face.Close()

	// Measure text width
	textWidth := font.MeasureString(face, label).Ceil()
	padding := 4
	borderOffset := 3

	// Calculate label dimensions
	labelWidth := textWidth + padding*2
	labelHeight := int(fontSize) + padding*2

	// Position label at top-left corner of box with small offset
	bgX := x + borderOffset
	bgY := y + borderOffset

	// Ensure background stays within image bounds
	if bgX+labelWidth > img.Bounds().Dx() {
		bgX = img.Bounds().Dx() - labelWidth
	}
	if bgY+labelHeight > img.Bounds().Dy() {
		bgY = img.Bounds().Dy() - labelHeight
	}
	if bgX < 0 {
		bgX = 0
	}
	if bgY < 0 {
		bgY = 0
	}

	// Draw solid black background
	bgColor := color.RGBA{0, 0, 0, 255}
	for py := bgY; py < bgY+labelHeight && py < img.Bounds().Dy(); py++ {
		for px := bgX; px < bgX+labelWidth && px < img.Bounds().Dx(); px++ {
			img.Set(px, py, bgColor)
		}
	}

	// Draw the text in white with anti-aliasing
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(image.White)

	// Position text baseline
	pt := freetype.Pt(bgX+padding, bgY+padding+int(fontSize*0.8))
	_, err = c.DrawString(label, pt)
	if err != nil {
		// If drawing fails, just skip the label
		return
	}
}
