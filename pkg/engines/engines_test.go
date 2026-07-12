package engines_test

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/goodblaster/gollate/pkg/engines"
	"github.com/goodblaster/gollate/pkg/ocr"
)

func TestBuiltinsRegistered(t *testing.T) {
	for _, name := range []string{"apple", "tesseract", "easyocr", "blocks"} {
		if !engines.Supported(name) {
			t.Errorf("built-in engine %q not registered", name)
		}
	}
}

// customEngine is what a consumer plugging in Textract, Document AI, or a
// local model writes: a struct mapping a name to parsing code.
type customEngine struct{}

func (customEngine) Name() string { return "myengine" }

func (customEngine) Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
	// Toy native format: {"items": [{"w": "hello", "x": 100, "y": 50}]}
	// with pixel coordinates and a fixed 50x20 px word box.
	var native struct {
		Items []struct {
			W string  `json:"w"`
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r).Decode(&native); err != nil {
		return nil, err
	}
	var blocks []ocr.Block
	for i, item := range native.Items {
		blocks = append(blocks, ocr.Block{
			Text:          item.W,
			Extractor:     "myengine",
			Index:         i,
			OriginalIndex: i,
			PageWidth:     pageWidth,
			PageHeight:    pageHeight,
			BoundingBox: ocr.BoundingBox{
				Left:   item.X / float64(pageWidth),
				Top:    item.Y / float64(pageHeight),
				Width:  50 / float64(pageWidth),
				Height: 20 / float64(pageHeight),
			},
		})
	}
	return blocks, nil
}

func TestRegisterCustomEngine(t *testing.T) {
	engines.Register(customEngine{})

	if !engines.Supported("myengine") {
		t.Fatal("custom engine not registered")
	}
	if !engines.Supported("MyEngine") {
		t.Fatal("engine lookup must be case-insensitive")
	}

	input := `{"items": [{"w": "hello", "x": 100, "y": 50}, {"w": "world", "x": 160, "y": 50}]}`
	blocks, err := engines.Read("myengine", strings.NewReader(input), 1000, 1000)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(blocks) != 2 || blocks[0].Text != "hello" || blocks[1].Index != 1 {
		t.Errorf("unexpected blocks: %+v", blocks)
	}
	if blocks[0].BoundingBox.Left != 0.1 {
		t.Errorf("Left = %v, want 0.1", blocks[0].BoundingBox.Left)
	}
}

func TestReadUnknownEngine(t *testing.T) {
	_, err := engines.Read("no-such-engine", strings.NewReader("{}"), 100, 100)
	if err == nil || !strings.Contains(err.Error(), "registered:") {
		t.Errorf("want error listing registered engines, got %v", err)
	}
}

func TestBlocksEngine(t *testing.T) {
	input := `[
		{"text": "hello", "bounds": {"top": 0.1, "left": 0.05, "width": 0.05, "height": 0.02}, "normalized_conf": 0.98},
		{"text": "", "bounds": {"top": 0.1, "left": 0.11, "width": 0.05, "height": 0.02}},
		{"text": "world", "bounds": {"top": 0.1, "left": 0.11, "width": 0.05, "height": 0.02}, "engine": "textract"}
	]`
	blocks, err := engines.Read("blocks", strings.NewReader(input), 1920, 1080)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (empty-text block dropped)", len(blocks))
	}
	if blocks[0].Extractor != "blocks" {
		t.Errorf("empty Extractor must default to %q, got %q (it is load-bearing)", "blocks", blocks[0].Extractor)
	}
	if blocks[1].Extractor != "textract" {
		t.Errorf("provided Extractor must be preserved, got %q", blocks[1].Extractor)
	}
	if blocks[1].Index != 2 || blocks[0].PageWidth != 1920 {
		t.Errorf("Index/PageWidth not filled: %+v", blocks[1])
	}
}

func TestBlocksEngineRejectsPixelCoordinates(t *testing.T) {
	input := `[{"text": "hello", "bounds": {"top": 200, "left": 100, "width": 50, "height": 20}}]`
	_, err := engines.Read("blocks", strings.NewReader(input), 1920, 1080)
	if err == nil || !strings.Contains(err.Error(), "0-1") {
		t.Errorf("pixel coordinates must be rejected with a helpful error, got %v", err)
	}
}
