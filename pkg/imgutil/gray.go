package imgutil

import "image"

func GrayImage(src image.Image) *image.Gray {
	bounds := src.Bounds()
	gray := image.NewGray(bounds)
	for x := 0; x < bounds.Max.X; x++ {
		for y := 0; y < bounds.Max.Y; y++ {
			var rgba = src.At(x, y)
			gray.Set(x, y, rgba)
		}
	}
	return gray
}
