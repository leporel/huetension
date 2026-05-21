package lut

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

// EncodeHaldPNG encodes a LUT as a 2D "LUT texture" PNG — a square grid
// of the cube laid out as √Size × √Size macroblocks, each Size × Size
// pixels and representing one fixed-B slice (R left-to-right, G top-to-
// bottom). Macroblocks are arranged row-major in B order
// (B = my·√Size + mx); final image side = Size · √Size pixels.
//
// Size must be a perfect square (4, 9, 16, 25, 36, 49, 64, ...).
//
// NOTE: this layout looks like the canonical HALD CLUT format but does
// not actually round-trip through ffmpeg's `haldclut` filter (empirically
// produces saturated/clipped output). The function is kept as a visual
// representation of the cube; for ffmpeg-side application use the .cube
// text format via `lut3d=...` instead.
func EncodeHaldPNG(l *LUT) ([]byte, error) {
	sqrtSize := int(math.Sqrt(float64(l.Size)))
	if sqrtSize*sqrtSize != l.Size {
		return nil, fmt.Errorf("lut: LUT texture requires Size to be a perfect square (4, 9, 16, 25, ...), got %d", l.Size)
	}

	side := l.Size * sqrtSize
	img := image.NewRGBA(image.Rect(0, 0, side, side))

	for b := range l.Size {
		mx := b % sqrtSize
		my := b / sqrtSize
		baseX := mx * l.Size
		baseY := my * l.Size
		for g := range l.Size {
			for r := range l.Size {
				node := l.Nodes[r+g*l.Size+b*l.Size*l.Size]
				img.SetRGBA(baseX+r, baseY+g,
					color.RGBA{R: node.R, G: node.G, B: node.B, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("lut: encode PNG: %w", err)
	}
	return buf.Bytes(), nil
}
