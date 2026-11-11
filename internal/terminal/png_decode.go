package terminal

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
)

// decodePNGBytes decodes raw PNG image bytes into RGBA format (32-bit: R, G, B, A).
// Returns width, height, pixels slice, and whether decoding succeeded.
func decodePNGBytes(data []byte) (uint32, uint32, []byte, bool) {
	if len(data) == 0 {
		return 0, 0, nil, false
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, false
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return 0, 0, nil, false
	}

	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Stride != w*4 {
		rgba = image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	}

	return uint32(w), uint32(h), rgba.Pix, true
}
