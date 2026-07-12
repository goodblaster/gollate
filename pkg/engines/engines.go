// Package engines maps OCR engine names to the code that parses their
// native output into normalized blocks.
//
// Built-in engines (apple, tesseract, easyocr, blocks) register themselves;
// consumers add their own - Textract, Google Document AI, anything local or
// remote - by implementing Engine and calling Register before parsing:
//
//	type textract struct{}
//
//	func (textract) Name() string { return "textract" }
//	func (textract) Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
//	    // parse the raw Textract response JSON into ocr.Block values
//	}
//
//	func init() { engines.Register(textract{}) }
//
// After that, api.SortRequest{Engine: "textract", ...} works like any
// built-in. Implementations must honor the block contract documented on
// ocr.Block. Engines whose output is already in the normalized block format
// don't need code at all: use the built-in "blocks" engine.
package engines

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/goodblaster/gollate/pkg/ocr"
)

// Engine parses one OCR engine's native output into normalized blocks.
type Engine interface {
	// Name is the identifier used by api.SortRequest.Engine and the CLI
	// -engine flag. Case-insensitive; registered lowercase.
	Name() string

	// Read parses the engine's native output into blocks satisfying the
	// contract documented on ocr.Block: word-level granularity, 0-1 page
	// coordinates, non-empty Extractor, Index reflecting emit order.
	// pageWidth/pageHeight are the source image dimensions in pixels, for
	// engines that report pixel coordinates.
	Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Engine{}
)

// Register makes an engine available by name. Built-in engines register
// themselves; call Register (typically from init) to add custom ones.
// Registering a name twice replaces the earlier engine.
func Register(e Engine) {
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(e.Name())] = e
}

// Get returns the engine registered under name (case-insensitive).
func Get(name string) (Engine, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := registry[strings.ToLower(name)]
	return e, ok
}

// Supported reports whether an engine is registered under name.
func Supported(name string) bool {
	_, ok := Get(name)
	return ok
}

// Names returns the registered engine names, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Read parses input with the named engine.
func Read(name string, r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
	e, ok := Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown engine %q (registered: %s)", name, strings.Join(Names(), ", "))
	}
	return e.Read(r, pageWidth, pageHeight)
}
