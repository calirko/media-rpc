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

	for y := range size {
		for x := range size {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d > outer {
				continue
			}
			if d > inner {
				img.SetNRGBA(x, y, color.NRGBA{R: 180, G: 180, B: 180, A: 255})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{R: 80, G: 80, B: 80, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
