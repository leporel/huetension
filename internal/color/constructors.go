package color

import (
	colorful "github.com/lucasb-eyer/go-colorful"
)

// FromHSL builds an opaque Color from H (degrees [0, 360]), S and L (0..1).
// Out-of-gamut values are clamped to sRGB.
func FromHSL(h, s, l float64) Color {
	return fromColorful(colorful.Hsl(modAngle(h), clampUnit(s), clampUnit(l)), 255)
}

// FromHSLA is the alpha-bearing variant of FromHSL.
func FromHSLA(h, s, l, alpha float64) Color {
	return fromColorful(colorful.Hsl(modAngle(h), clampUnit(s), clampUnit(l)), toUint8(alpha*255))
}

// FromHSV builds an opaque Color from H (degrees), S and V (0..1).
func FromHSV(h, s, v float64) Color {
	return fromColorful(colorful.Hsv(modAngle(h), clampUnit(s), clampUnit(v)), 255)
}

// FromHCL builds an opaque Color from CIE H (degrees), C and L (go-colorful's
// 0..1 scale, i.e. CSS-Lab-divided-by-100).
func FromHCL(h, c, l float64) Color {
	return fromColorful(colorful.Hcl(modAngle(h), c, l), 255)
}

// FromLab builds an opaque Color from CSS-scale Lab (L 0..100, a/b ≈ ±125).
func FromLab(L, a, b float64) Color {
	return fromColorful(colorful.Lab(L/labScale, a/labScale, b/labScale), 255)
}

// FromOkLab builds an opaque Color from OkLab L, a, b (L 0..1, a/b roughly ±0.4).
func FromOkLab(L, a, b float64) Color {
	r, g, bb := okLabToRGB(L, a, b)
	return Color{R: toUint8(r * 255), G: toUint8(g * 255), B: toUint8(bb * 255), A: 255}
}

// FromOkLCH builds an opaque Color from OkLCH L (0..1), C, H (degrees).
func FromOkLCH(L, C, H float64) Color {
	_, a, b := okLCHToLab(L, C, H)
	return FromOkLab(L, a, b)
}

// FromRGB01 builds an opaque Color from 0..1 sRGB floats. Values outside
// [0, 1] are clamped.
func FromRGB01(r, g, b float64) Color {
	return Color{
		R: toUint8(clampUnit(r) * 255),
		G: toUint8(clampUnit(g) * 255),
		B: toUint8(clampUnit(b) * 255),
		A: 255,
	}
}
