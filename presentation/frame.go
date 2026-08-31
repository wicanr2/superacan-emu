// Package presentation converts immutable machine output for host adapters.
// It does not advance emulated time or depend on a windowing library.
package presentation

import (
	"image"
	"image/png"
	"io"
)

// ARGBToRGBA converts the UM6618 framebuffer into conventional RGBA bytes.
// destination may be reused across frames.
func ARGBToRGBA(destination []byte, source []uint32) []byte {
	required := len(source) * 4
	if cap(destination) < required {
		destination = make([]byte, required)
	} else {
		destination = destination[:required]
	}
	for index, pixel := range source {
		offset := index * 4
		destination[offset] = uint8(pixel >> 16)
		destination[offset+1] = uint8(pixel >> 8)
		destination[offset+2] = uint8(pixel)
		destination[offset+3] = uint8(pixel >> 24)
	}
	return destination
}

func EncodePNG(output io.Writer, width, height int, framebuffer []uint32) error {
	frame := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(frame.Pix, ARGBToRGBA(frame.Pix[:0], framebuffer))
	return png.Encode(output, frame)
}
