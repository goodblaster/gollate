package sorters

import (
	"fmt"
	"strings"
	"time"

	"github.com/goodblaster/gollate/pkg/language"
	"github.com/goodblaster/gollate/pkg/logger"
	"github.com/goodblaster/gollate/pkg/ocr"
)

type Block = ocr.Block

// Sorter performs pathfinding-based sorting of OCR blocks to match canonical text.
type Sorter struct {
	lines   *LineServer        // canonical lines of text
	input   []Block            // input OCR blocks (normalized)
	output  []Block            // output OCR blocks (normalized)
	mapped  map[string][]Block // map of words to all blocks that contain that word
	config  SorterConfig       // algorithm configuration parameters
	metrics SortMetrics        // performance metrics
	handler language.Handler   // language-specific rules and formatting

	candidatePaths []Path        // viable paths found through the chain during pathfinding
	holeSlots      []bool        // per-slot wildcard-hole markers for the current chain
	shortest       float64       // shortest path found so far
	elapsed        time.Duration // time it took to sort
	perm           int           // current permutation count
	logger         logger.Logger // logger instance

	// debug
	debugPrintPathNodes   bool
	debugBuildDebugImages func(lineIndex int, line Line, path Path)
}

// NewOcrSorter creates a new sorter with default configuration.
func NewOcrSorter(ocrBlocks []Block, text []string, log logger.Logger) *Sorter {
	return NewOcrSorterWithConfig(ocrBlocks, text, log, DefaultConfig())
}

// NewOcrSorterWithConfig creates a new sorter with custom configuration.
func NewOcrSorterWithConfig(ocrBlocks []Block, text []string, log logger.Logger, config SorterConfig) *Sorter {
	// Use default logger if none provided
	if log == nil {
		log = logger.Default()
	}

	// Normalize text and parse into lines first (needed for smart hyphen splitting)
	lines := ParseLines(text)

	// Normalize all the text we received from OCR.
	ocrBlocks = normalizeOcrBlocks(ocrBlocks, config.SplitHyphenatedWords, lines)

	// Detect language from canonical text
	handler := language.Detect(text...)

	// Preparation: infer text orientation from block geometry (see
	// vertical.go). Only ever switches horizontal -> vertical; needs line
	// data for the flow signal.
	if !config.DisableVerticalDetection && config.ReadingOrder.IsHorizontal() && detectVerticalText(ocrBlocks) {
		log.Debug("vertical text detected; switching reading order to VerticalTTB_RTL")
		config.ReadingOrder = VerticalTTB_RTL
		if config.EnableVerticalWrapBridging && !config.EnableWrapBridging {
			log.Debug("vertical text: enabling wrap bridging (columns wrap like lines)")
			config.EnableWrapBridging = true
		}
	}

	// Preparation: repair unrecognizable tokens using the engine's own line
	// grouping (see linerepair.go). Only the matching key changes; block
	// text stays what OCR read. No-op when blocks carry no line data.
	repairs := 0
	if !config.DisableLineRepair {
		repairs = repairLines(ocrBlocks, lines, handler)
	}

	// Start building the sorter.
	sorter := &Sorter{
		input:   ocrBlocks,
		lines:   NewLineServer(lines),
		mapped:  mapBlocks(ocrBlocks),
		config:  config,
		handler: handler,
		logger:  log,
	}
	sorter.metrics.LineRepairs = repairs

	return sorter
}

func normalizeOcrBlocks(ocrBlocks []Block, splitHyphenated bool, canonicalLines []Line) []Block {
	var processed []Block
	var next NextIndex

	// Build a map of all words in canonical text for quick lookup
	canonicalWords := make(map[string]bool)
	if splitHyphenated {
		for _, line := range canonicalLines {
			words := strings.Fields(line.Normalized)
			for _, word := range words {
				canonicalWords[word] = true
			}
		}
	}

	for _, w := range ocrBlocks {
		if splitHyphenated {
			// Skip hyphen splitting for numeric content (phone numbers, dates, etc.)
			// Only split actual hyphenated words like "company-wide"
			isNumeric := true
			for _, c := range w.Text {
				if c != ' ' && c != '-' && c != '\u2011' && (c < '0' || c > '9') {
					isNumeric = false
					break
				}
			}

			// Don't split phone numbers and other numeric hyphenated content
			if isNumeric {
				// Process as a single block without splitting
				w.NormedText = NormalizeText(w.Text)
				if strings.TrimSpace(w.NormedText) != "" {
					w.OriginalIndex = w.Index
					w.Index = next.Index()
					processed = append(processed, w)
				}
				continue
			}

			// Check if this word contains hyphens - track them before splitting
			// We need to preserve hyphen info for reconstruction
			type partInfo struct {
				text        string
				hyphenAfter string // hyphen character that came after this part
			}
			var partsWithHyphens []partInfo

			// Manual parsing to preserve hyphen characters
			currentPart := strings.Builder{}
			for _, c := range w.Text {
				if c == '-' || c == '\u2011' {
					// Found a hyphen - save current part and the hyphen
					if currentPart.Len() > 0 {
						partsWithHyphens = append(partsWithHyphens, partInfo{
							text:        currentPart.String(),
							hyphenAfter: string(c),
						})
						currentPart.Reset()
					}
				} else if c == ' ' {
					// Space separates parts but isn't preserved
					if currentPart.Len() > 0 {
						partsWithHyphens = append(partsWithHyphens, partInfo{
							text:        currentPart.String(),
							hyphenAfter: "",
						})
						currentPart.Reset()
					}
				} else {
					currentPart.WriteRune(c)
				}
			}
			// Don't forget the last part
			if currentPart.Len() > 0 {
				partsWithHyphens = append(partsWithHyphens, partInfo{
					text:        currentPart.String(),
					hyphenAfter: "",
				})
			}

			// Check if we should split: all parts must exist in canonical text
			shouldSplit := len(partsWithHyphens) > 1
			if shouldSplit {
				for _, part := range partsWithHyphens {
					normalized := NormalizeText(part.text)
					if normalized != "" && !canonicalWords[normalized] {
						// This part doesn't exist in canonical - don't split
						shouldSplit = false
						break
					}
				}
			}

			// If we shouldn't split, keep as single block
			if !shouldSplit {
				w.NormedText = NormalizeText(w.Text)
				if strings.TrimSpace(w.NormedText) != "" {
					w.OriginalIndex = w.Index
					w.Index = next.Index()
					processed = append(processed, w)
				}
				continue
			}

			// Split into parts - filter and normalize
			var validParts []partInfo
			for _, part := range partsWithHyphens {
				normalized := NormalizeText(part.text)
				if strings.TrimSpace(normalized) != "" {
					validParts = append(validParts, partInfo{
						text:        part.text, // Keep original for display
						hyphenAfter: part.hyphenAfter,
					})
				}
			}

			size := len(validParts)
			if size == 0 {
				continue
			}

			// Preserve the original index before we overwrite it
			originalIndex := w.Index

			if size == 1 {
				w.NormedText = NormalizeText(validParts[0].text)
				w.OriginalIndex = originalIndex
				w.Index = next.Index()
				w.HyphenAfter = validParts[0].hyphenAfter
				processed = append(processed, w)
				continue
			}

			// Split into multiple blocks
			for j, part := range validParts {
				newBlock := w
				newBlock.Text = part.text
				newBlock.NormedText = NormalizeText(part.text)
				newBlock.OriginalIndex = originalIndex // Preserve original index for all split blocks
				newBlock.Index = next.Index()
				newBlock.HyphenAfter = part.hyphenAfter // Preserve hyphen for reconstruction

				// Can't use the full dimensions of the original block.
				// Break up evenly according to the number of parts.
				// Future improvement: Measure actual text width for more accurate bounding boxes.
				newBlock.BoundingBox.Width = w.BoundingBox.Width / float64(size)
				newBlock.BoundingBox.Left = w.BoundingBox.Left + (float64(j) * newBlock.BoundingBox.Width)
				if j > 0 {
					newBlock.BoundingBox.Left = processed[len(processed)-1].Right()
				}
				//newBlock.BoundingBox.Height = w.BoundingBox.Height

				processed = append(processed, newBlock)
			}

			continue
		}

		// Only normalize if not already set (e.g., by Apple normalizer)
		if w.NormedText == "" {
			w.NormedText = NormalizeText(w.Text)
		}
		if strings.TrimSpace(w.NormedText) != "" {
			w.OriginalIndex = w.Index // Preserve original index
			w.Index = next.Index()
			processed = append(processed, w)
		}
	}

	return processed
}

// Debug - perform a user-defined debug function for each line where we find a path.
func (s *Sorter) Debug(f func(lineIndex int, line Line, path Path)) {
	s.debugBuildDebugImages = f
}

func (s *Sorter) InputBlock(i int) Block {
	return s.input[i]
}

func (s *Sorter) InputBlocks() []Block {
	return s.input
}

func (s *Sorter) MappedBlocks() map[string][]Block {
	return s.mapped
}

func (s *Sorter) Print(path Path) {
	for _, index := range path.Nodes {
		fmt.Print(s.input[index].NormedText, " ")
	}
	fmt.Println()
}

func (s *Sorter) LineCount() int {
	return len(s.lines.List())
}

func (s *Sorter) Line(i int) Line {
	return s.lines.List()[i]
}

// Remove the words from the chain that have been used in the path.
// This prevents them from being reused which could lead to strange outcomes.
func (s *Sorter) unmapWords(path Path) {
	for _, index := range path.Nodes {
		if index < 0 {
			continue // unfilled hole - no block to unmap
		}
		s.unmapWordIndex(index)
	}
}

// unmapWordIndex - remove word index so it doesn't get reused.
func (s *Sorter) unmapWordIndex(index int) {
	// Search through ALL mapped word lists to find and remove this block by Index.
	for normedText, wordList := range s.mapped {
		for i, w := range wordList {
			if w.Index == index {
				// Delete the word from the list.
				s.mapped[normedText] = append(wordList[:i], wordList[i+1:]...)

				// If the list is now empty, remove it from the map.
				if len(s.mapped[normedText]) == 0 {
					delete(s.mapped, normedText)
				}
				return
			}
		}
	}
}

func (s *Sorter) Lines() []Line {
	return s.lines.List()
}

// UnhandledLines returns unmatched lines with a debug-oriented index prefix.
// For clean output use UnhandledText.
func (s *Sorter) UnhandledLines() []string {
	var unhandled []string
	for i, line := range s.lines.lines {
		// Exclude split lines - they were successfully split into smaller pieces
		// which may or may not have been found. We only care about lines that
		// were neither found nor split.
		if !line.Found && !line.Split {
			unhandled = append(unhandled, fmt.Sprintf("%00d - %s\n", i, s.Line(i).Normalized))
		}
	}
	return unhandled
}

// UnhandledText returns the readable canonical text of lines that were
// neither matched nor split, whitespace trimmed, blanks omitted. This is
// the clean form for API/output consumers (no index prefix, no newline).
// Always non-nil so it serializes as [] rather than null.
func (s *Sorter) UnhandledText() []string {
	unhandled := []string{}
	for i, line := range s.lines.lines {
		if line.Found || line.Split || line.IsBlank {
			continue
		}
		if text := strings.TrimSpace(s.Line(i).OriginalText); text != "" {
			unhandled = append(unhandled, text)
		}
	}
	return unhandled
}

func (s *Sorter) Elapsed() time.Duration {
	return s.elapsed
}
