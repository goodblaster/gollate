package imgutil

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fogleman/gg"
	"github.com/goodblaster/errors"
	"github.com/goodblaster/gollate/pkg/ocr"
)

func Clone(filename string) (image.Image, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	defer f.Close()

	im, _, err := image.Decode(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode image")
	}

	return im, nil
}

func DebugOut(inName, outName string, blocks []ocr.Block) error {
	w, h, err := Size(inName)
	if err != nil {
		return errors.Wrap(err, "failed to get size of image")
	}

	outImage, err := Clone(inName)
	if err != nil {
		return errors.Wrap(err, "failed to clone image")
	}

	dc := gg.NewContextForImage(GrayImage(outImage))
	for _, block := range blocks {
		bgColor := color.RGBA{R: 0xe7, G: 0x4c, B: 0x3c, A: 0xff}
		dc.SetColor(bgColor)
		w64 := float64(w)
		h64 := float64(h)
		dc.DrawRectangle(w64*block.Left(), h64*block.Top(), w64*block.Width(), h64*block.Height())
		dc.Fill()

		dc.SetColor(color.White)
		cx, cy := block.Center()
		dc.DrawStringAnchored(strconv.Itoa(block.Index), w64*cx, h64*cy, 0.5, 0.5)
	}

	return dc.SavePNG(outName)
}

func CreateRawIndexImage(inName, outName string, blocks []ocr.Block) error {
	w, h, err := Size(os.Getenv("OCR_WORK_DIR") + "/" + inName)
	if err != nil {
		return errors.Wrap(err, "failed to get size of image")
	}

	outImage, err := Clone(os.Getenv("OCR_WORK_DIR") + "/" + inName)
	if err != nil {
		return errors.Wrap(err, "failed to clone image")
	}

	dc := gg.NewContextForImage(GrayImage(outImage))
	for i, block := range blocks {
		bgColor := color.RGBA{R: 0xe7, G: 0x4c, B: 0x3c, A: 0xff}
		dc.SetColor(bgColor)
		w64 := float64(w)
		h64 := float64(h)
		dc.DrawRectangle(w64*block.Left(), h64*block.Top(), w64*block.Width(), h64*block.Height())
		dc.Fill()

		dc.SetColor(color.White)
		cx, cy := block.Center()
		dc.DrawStringAnchored(strconv.Itoa(i), w64*cx, h64*cy, 0.5, 0.5)

	}

	return dc.SavePNG(filepath.Join(os.Getenv("OCR_WORK_DIR"), "output", outName))
}
