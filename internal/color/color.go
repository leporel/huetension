// Package color provides huetension's canonical color type and conversions
// across hex, rgb, hsl, hsv, hls (Python colorsys), lab, lch, oklab, oklch,
// and CSS named colors.
//
// Color values are stored as 8-bit-per-channel sRGB. Conversions go through
// either go-colorful (CIE Lab/HCL via D65) or hand-written matrices
// (OkLab / OkLCH from Björn Ottosson). Round-tripping back to RGB is
// guaranteed lossless within 1 unit per channel.
package color

import (
	stdcolor "image/color"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// Source records the image-space coordinate (0..1 on both axes, origin
// top-left) of a representative pixel that a palette color was sampled
// from. Populated by extract.AnnotateSources for palettes produced from
// images; nil for every other origin (hex parse, harmony generation,
// gradient interpolation, library lookup).
//
// Kept here rather than in palette/ so it can live as a field on Color
// without forcing palette to import color and vice versa.
type Source struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Color is the canonical sRGB color used throughout huetension.
//
// A is 255 for opaque colors; Freq is optional metadata (0..1) attached by
// palette extractors and is ignored by parsers, formatters, and conversions.
// Source is an optional pin coordinate attached by image extraction so the
// Web UI can show a Kuler-style pin on the source image; nil for non-image
// colors. Palette operations (sort / harmony / gradient) pass Source
// through unchanged via struct copy — the pointer travels with its color.
type Color struct {
	R, G, B uint8
	A       uint8
	Freq    float64
	Source  *Source
}

// New returns an opaque Color.
func New(r, g, b uint8) Color { return Color{R: r, G: g, B: b, A: 255} }

// NewWithAlpha returns a Color with explicit alpha.
func NewWithAlpha(r, g, b, a uint8) Color { return Color{R: r, G: g, B: b, A: a} }

// HasAlpha reports whether the color has a non-opaque alpha channel.
func (c Color) HasAlpha() bool { return c.A != 255 }

// AlphaFloat returns alpha in 0..1.
func (c Color) AlphaFloat() float64 { return float64(c.A) / 255 }

// RGBA implements image/color.Color (premultiplied 16-bit alpha).
func (c Color) RGBA() (r, g, b, a uint32) {
	af := uint32(c.A)
	r = uint32(c.R) * 0x101 * af / 0xff
	g = uint32(c.G) * 0x101 * af / 0xff
	b = uint32(c.B) * 0x101 * af / 0xff
	a = af * 0x101
	return
}

// FromImageColor converts any image/color.Color to a huetension Color.
// Premultiplied alpha is undone so component values match the user expectation
// (rgb(255, 0, 0, 0.5), not rgb(127, 0, 0, 0.5)).
func FromImageColor(ic stdcolor.Color) Color {
	r, g, b, a := ic.RGBA()
	if a == 0 {
		return Color{}
	}
	// Undo premultiplication, then drop the high byte.
	return Color{
		R: uint8((r * 0xff / a) & 0xff),
		G: uint8((g * 0xff / a) & 0xff),
		B: uint8((b * 0xff / a) & 0xff),
		A: uint8(a >> 8),
	}
}

// toColorful converts to go-colorful's float representation.
// Alpha is lost — go-colorful is alpha-agnostic.
func (c Color) toColorful() colorful.Color {
	return colorful.Color{
		R: float64(c.R) / 255,
		G: float64(c.G) / 255,
		B: float64(c.B) / 255,
	}
}

// fromColorful clamps to the sRGB gamut and converts back to 8-bit, attaching
// the requested alpha.
func fromColorful(cc colorful.Color, alpha uint8) Color {
	cc = cc.Clamped()
	return Color{
		R: toUint8(cc.R * 255),
		G: toUint8(cc.G * 255),
		B: toUint8(cc.B * 255),
		A: alpha,
	}
}

// Static interface check.
var _ stdcolor.Color = Color{}
