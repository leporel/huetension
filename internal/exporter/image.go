package exporter

import (
	"bytes"
	"fmt"
	"image"
	stdcolor "image/color"
	"image/draw"
	"image/jpeg"
	"image/png"

	"github.com/leporel/huetension/internal/palette"
)

// Default swatch dimensions for image renders. 96×96 is the size used by
// the testdata sidecars; tweak via Options.SwatchWidth / SwatchHeight if
// the consumer wants tall thin strips or chunky squares.
const (
	defaultSwatchWidth  = 96
	defaultSwatchHeight = 96
	jpegQuality         = 90
)

// renderSwatchPNG paints each color as a vertical column in a single
// horizontal strip and encodes the result as PNG. Layout matches the
// testdata previews: width = swatchW × len(palette), height = swatchH.
func renderSwatchPNG(p *palette.Palette, opts Options) ([]byte, error) {
	img := buildSwatchImage(p, opts)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}
	return buf.Bytes(), nil
}

// renderSwatchJPEG is the JPEG variant. JPEG is lossy but ~5x smaller —
// fine for visual previews where the exact byte values don't matter.
func renderSwatchJPEG(p *palette.Palette, opts Options) ([]byte, error) {
	img := buildSwatchImage(p, opts)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("jpeg encode: %w", err)
	}
	return buf.Bytes(), nil
}

// buildSwatchImage produces the in-memory image used by both PNG and JPEG
// encoders. Alpha is intentionally flattened to fully opaque — JPEG can't
// carry alpha at all, and visual previews read better as solid blocks.
func buildSwatchImage(p *palette.Palette, opts Options) image.Image {
	w, h := opts.SwatchWidth, opts.SwatchHeight
	if w <= 0 {
		w = defaultSwatchWidth
	}
	if h <= 0 {
		h = defaultSwatchHeight
	}
	n := p.Len()
	if n == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	img := image.NewRGBA(image.Rect(0, 0, w*n, h))
	for i, c := range p.Colors {
		rect := image.Rect(i*w, 0, (i+1)*w, h)
		fill := stdcolor.RGBA{R: c.R, G: c.G, B: c.B, A: 255}
		draw.Draw(img, rect, &image.Uniform{C: fill}, image.Point{}, draw.Src)
	}
	return img
}
