package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goodblaster/errors"
	"github.com/goodblaster/gollate/pkg/api"
	"github.com/goodblaster/gollate/pkg/imgutil"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
	"github.com/goodblaster/gollate/pkg/sorters"
)

var (
	engine    = flag.String("engine", "", "OCR engine: apple, tesseract, easyocr, blocks (default: apple on macOS, tesseract elsewhere)")
	ocrFile   = flag.String("ocr-file", "", "Path to OCR JSON file [required]")
	textFile  = flag.String("text-file", "", "Path to canonical text file [required]")
	width     = flag.Int("width", 0, "Page width in pixels [required]")
	height    = flag.Int("height", 0, "Page height in pixels [required]")
	imagePath = flag.String("image", "", "Path to image file (for debug output)")
	output    = flag.String("output", "", "Output file path (default: stdout)")
	debugDir  = flag.String("debug-dir", "", "Directory for debug output files (enables debug mode)")
	format    = flag.String("format", "json", "Output format: json, text")

	// Provenance for response meta.source (optional; the pipeline supplies these).
	sourceFile = flag.String("source", "", "Original source file (PDF/image) for response meta")
	sliceCount = flag.Int("slices", 0, "How many strips the image was sliced into for OCR (for response meta)")

	// Language is the only allowed hint about the document. It selects the
	// base configuration (reading order, tokenization, penalties).
	langName = flag.String("language", "", "Document language: english, spanish, chinese, japanese, arabic, hindi (default: english defaults)")

	// Layout flags
	columnJumpPenalty = flag.Float64("column-jump-penalty", 0, "Override penalty for jumping between columns (default: from language config)")
)

// defaultEngine picks the OCR engine matching the platform's OCR tooling:
// Apple Vision on macOS, Tesseract everywhere else.
func defaultEngine() string {
	if runtime.GOOS == "darwin" {
		return "apple"
	}
	return "tesseract"
}

func main() {
	flag.Parse()

	// Initialize logger
	// Logs go to stderr: stdout belongs to the sorted output, so
	// `gollate -format json | jq` stays clean.
	log := logger.NewLogosTo(os.Stderr)

	if *engine == "" {
		*engine = defaultEngine()
	}

	// Validate required flags
	if *ocrFile == "" || *textFile == "" || *width == 0 || *height == 0 {
		fmt.Fprintln(os.Stderr, "Error: Missing required flags")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}

	// Validate format
	if *format != "text" && *format != "json" {
		fmt.Fprintf(os.Stderr, "Error: Invalid format %q, must be 'text' or 'json'\n", *format)
		os.Exit(1)
	}

	// Create and parse request
	request := api.SortRequest{
		Engine:     *engine,
		PageWidth:  *width,
		PageHeight: *height,
	}
	request.WithLogger(log)

	// Read OCR file
	ocrData, err := os.ReadFile(*ocrFile)
	if err != nil {
		log.WithError(err).Fatal("failed to read OCR file")
	}
	request.InputJson = string(ocrData)

	// Read text file
	textData, err := os.ReadFile(*textFile)
	if err != nil {
		log.WithError(err).Fatal("failed to read text file")
	}
	request.Lines = strings.Split(string(textData), "\n")

	// Parse the request (validates engine and converts OCR format)
	if err := request.Parse(); err != nil {
		log.WithError(err).Fatal("failed to parse request")
	}

	config := sorters.ConfigForLanguage(*langName)

	if *columnJumpPenalty > 0 {
		config.ColumnJumpPenalty = *columnJumpPenalty
	}

	// Validate config
	if err := config.Validate(); err != nil {
		log.WithError(err).Fatal("invalid configuration")
	}

	// Create sorter with custom config
	sorter := sorters.NewOcrSorterWithConfig(
		request.Blocks(),
		request.Lines,
		log,
		config,
	)

	// Run the sort
	blocks, err := sorter.Sort()
	if err != nil {
		log.WithError(err).Fatal("failed to sort")
	}

	// Generate debug output if requested
	if *debugDir != "" {
		if err := generateDebugOutput(sorter, blocks, log); err != nil {
			log.WithError(err).Error("failed to generate debug output")
		}
	}

	// Generate output. NewSortResponse fills document, unhandled, and stats;
	// the CLI adds source/provenance it alone can see.
	response := api.NewSortResponse(sorter)
	response.Meta.Source = &api.Source{
		Engine:    *engine,
		Language:  *langName,
		Width:     *width,
		Height:    *height,
		Slices:    *sliceCount,
		ImageFile: fileInfo(*sourceFile),
		OCRFile:   fileInfo(*ocrFile),
		TextFile:  fileInfo(*textFile),
	}

	if err := writeOutput(*response); err != nil {
		log.WithError(err).Fatal("failed to write output")
	}
}

// fileInfo returns path + size for a file, or nil if unset/unstattable.
func fileInfo(path string) *api.FileInfo {
	if path == "" {
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return &api.FileInfo{Path: path}
	}
	return &api.FileInfo{Path: path, Bytes: fi.Size()}
}

func writeOutput(response api.SortResponse) error {
	var writer io.Writer = os.Stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			return errors.Wrap(err, "failed to create output file")
		}
		defer f.Close()
		writer = f
	}

	switch *format {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return errors.Wrap(err, "failed to encode JSON")
		}

	case "text":
		if _, err := fmt.Fprintln(writer, response.Document.Text); err != nil {
			return errors.Wrap(err, "failed to write text output")
		}

	default:
		return fmt.Errorf("invalid format: %s", *format)
	}

	return nil
}

func generateDebugOutput(sorter *sorters.Sorter, blocks []ocr.Block, log logger.Logger) error {
	// Create debug directory
	if err := os.MkdirAll(*debugDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create debug directory")
	}

	// Write sorted text
	sortedTextPath := filepath.Join(*debugDir, "sorted-text.txt")
	sortedText, err := os.Create(sortedTextPath)
	if err != nil {
		return errors.Wrap(err, "failed to create sorted-text.txt")
	}
	defer sortedText.Close()

	for _, line := range sorter.SortedLines() {
		fmt.Fprintln(sortedText, line)
	}

	// Write sorted nodes as JSON
	sortedNodesPath := filepath.Join(*debugDir, "sorted-nodes.txt")
	sortedNodes, err := os.Create(sortedNodesPath)
	if err != nil {
		return errors.Wrap(err, "failed to create sorted-nodes.txt")
	}
	defer sortedNodes.Close()

	b, _ := json.MarshalIndent(sorter.SortedBlocks(), "", "  ")
	sortedNodes.Write(b)

	// Write missing lines
	missingLinesPath := filepath.Join(*debugDir, "missing-lines.txt")
	missingLines, err := os.Create(missingLinesPath)
	if err != nil {
		return errors.Wrap(err, "failed to create missing-lines.txt")
	}
	defer missingLines.Close()

	for i, line := range sorter.Lines() {
		if !line.Found {
			fmt.Fprintf(missingLines, "%03d - %s\n", i, line.OriginalText)
		}
	}

	// Generate debug images if image path provided
	if *imagePath != "" {
		// Raw indexed image
		rawIndexedPath := filepath.Join(*debugDir, "raw-indexed.png")
		if err := imgutil.CreateRawIndexImage(*imagePath, rawIndexedPath, sorter.InputBlocks()); err != nil {
			log.WithError(err).Error("failed to create raw-indexed.png")
		}

		// Normalized indexed image (blocks are already normalized by the sorter)
		normIndexedPath := filepath.Join(*debugDir, "norm-indexed.png")
		if err := imgutil.CreateRawIndexImage(*imagePath, normIndexedPath, sorter.InputBlocks()); err != nil {
			log.WithError(err).Error("failed to create norm-indexed.png")
		}

		// Sorted nodes image
		sortedNodesImgPath := filepath.Join(*debugDir, "sorted-nodes.png")
		if err := imgutil.DebugOut(*imagePath, sortedNodesImgPath, sorter.SortedBlocks()); err != nil {
			log.WithError(err).Error("failed to create sorted-nodes.png")
		}

		// Leftover nodes image
		mapped := sorter.MappedBlocks()
		var leftoverBlocks []ocr.Block
		for _, blocks := range mapped {
			leftoverBlocks = append(leftoverBlocks, blocks...)
		}
		leftoverNodesPath := filepath.Join(*debugDir, "leftover-nodes.png")
		if err := imgutil.DebugOut(*imagePath, leftoverNodesPath, leftoverBlocks); err != nil {
			log.WithError(err).Error("failed to create leftover-nodes.png")
		}
	}

	return nil
}
