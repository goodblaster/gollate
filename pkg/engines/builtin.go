package engines

import (
	"io"
	"strconv"

	"github.com/goodblaster/errors"
	"github.com/goodblaster/gollate/pkg/engines/apple"
	"github.com/goodblaster/gollate/pkg/engines/easyocr"
	"github.com/goodblaster/gollate/pkg/engines/tesseract"
	"github.com/goodblaster/gollate/pkg/ocr"
)

func init() {
	Register(appleEngine{})
	Register(tesseractEngine{})
	Register(easyOCREngine{})
	Register(blocksEngine{})
}

type appleEngine struct{}

func (appleEngine) Name() string { return "apple" }

func (appleEngine) Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
	appleBlocks, err := apple.Read(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse Apple Vision OCR JSON - ensure input matches VNRecognizeTextRequest output format")
	}

	var blocks []ocr.Block
	for i, appleBlock := range appleBlocks {
		if appleBlock.Text == "" {
			continue
		}
		block := ocr.FromApple(appleBlock, pageWidth, pageHeight)
		block.Index = i         // Set initial index from OCR data
		block.OriginalIndex = i // Also set OriginalIndex (preserved during normalization)
		blocks = append(blocks, block)
	}
	return blocks, nil
}

type tesseractEngine struct{}

func (tesseractEngine) Name() string { return "tesseract" }

func (tesseractEngine) Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
	tessBlocks, err := tesseract.Read(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse Tesseract OCR JSON - ensure input matches Tesseract output format")
	}

	var blocks []ocr.Block
	for i, tessBlock := range tessBlocks {
		if tessBlock.Text == "" {
			continue
		}
		block := ocr.FromTesseract(tessBlock, pageWidth, pageHeight)
		block.Index = i         // Set initial index from OCR data
		block.OriginalIndex = i // Also set OriginalIndex (preserved during normalization)
		block.LineId = strconv.Itoa(tessBlock.LineNum)
		blocks = append(blocks, block)
	}
	return blocks, nil
}

type easyOCREngine struct{}

func (easyOCREngine) Name() string { return "easyocr" }

func (easyOCREngine) Read(r io.Reader, pageWidth, pageHeight int) ([]ocr.Block, error) {
	easyBlocks, err := easyocr.Read(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse EasyOCR JSON - ensure input matches EasyOCR output format")
	}

	var blocks []ocr.Block
	for i, easyBlock := range easyBlocks {
		if easyBlock.Text == "" {
			continue
		}
		block := ocr.FromEasyOCR(easyBlock, pageWidth, pageHeight)
		block.Index = i         // Set initial index from OCR data
		block.OriginalIndex = i // Also set OriginalIndex (preserved during normalization)
		block.LineId = ""       // EasyOCR does not report line grouping.
		blocks = append(blocks, block)
	}
	return blocks, nil
}
