package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

func makePNG() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	cx, cy := float64(size)/2, float64(size)/2
	outer := float64(size)/2 - 1
	inner := outer * 0.55

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d > outer {
				continue
			}
			if d > inner {
				img.SetNRGBA(x, y, color.NRGBA{R: 30, G: 180, B: 200, A: 255})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{R: 15, G: 100, B: 120, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
