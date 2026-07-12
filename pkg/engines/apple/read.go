package apple

import (
	"encoding/json"
	"io"
	"os"

	"github.com/goodblaster/errors"
)

// Read parses Apple Vision OCR JSON from a reader and returns blocks.
// The JSON is expected to be an array of Line objects.
func Read(r io.Reader) ([]Block, error) {
	var lines []Line
	if err := json.NewDecoder(r).Decode(&lines); err != nil {
		return nil, errors.Wrap(err, "failed to decode apple vision json")
	}

	var blocks []Block
	for lineNum, line := range lines {
		for _, word := range line.Words {
			// Copy word data and add line metadata
			block := word
			block.Confidence = line.Confidence
			block.LineNum = lineNum
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}

// ReadFile reads Apple Vision OCR JSON from a file.
func ReadFile(path string) ([]Block, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open apple vision json file")
	}
	defer f.Close()

	return Read(f)
}
