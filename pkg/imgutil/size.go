package imgutil

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/goodblaster/errors"
)

// Size - Determine the size of an image, so we can convert integer pixels to float percentages.
func Size(filename string) (int, int, error) {
	f, err := os.Open(filename)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to open file")
	}
	defer f.Close()

	im, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to decode image")
	}

	return im.Width, im.Height, nil
}
