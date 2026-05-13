package color

// Luminance returns relative luminance per WCAG 2.1 in 0..1.
//
// This is the perceptual measure used by sort-by-luminance and the
// contrast helpers; it is not the same as HSL lightness.
func (c Color) Luminance() float64 {
	rl := srgbToLinear(float64(c.R) / 255)
	gl := srgbToLinear(float64(c.G) / 255)
	bl := srgbToLinear(float64(c.B) / 255)
	return 0.2126*rl + 0.7152*gl + 0.0722*bl
}

// Lightness returns the L component of HSL in 0..1.
func (c Color) Lightness() float64 {
	_, _, l := c.toColorful().Hsl()
	return l
}

// HueDeg returns the HSL hue in degrees [0, 360).
func (c Color) HueDeg() float64 {
	h, _, _ := c.toColorful().Hsl()
	return h
}

// Saturation returns the HSL saturation in 0..1.
func (c Color) Saturation() float64 {
	_, s, _ := c.toColorful().Hsl()
	return s
}

// OkL returns the L component of OkLab in 0..1 — perceptually uniform,
// preferred over Luminance() when ordering for designer-facing output.
func (c Color) OkL() float64 {
	L, _, _ := rgbToOkLab(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
	return L
}

// ToHSL returns (H deg, S 0..1, L 0..1).
func (c Color) ToHSL() (h, s, l float64) {
	return c.toColorful().Hsl()
}

// ToHSV returns (H deg, S 0..1, V 0..1).
func (c Color) ToHSV() (h, s, v float64) {
	return c.toColorful().Hsv()
}

// ToLab returns CSS-scaled Lab: L 0..100, a/b roughly ±125.
func (c Color) ToLab() (L, a, b float64) {
	cl, ca, cb := c.toColorful().Lab()
	return cl * labScale, ca * labScale, cb * labScale
}

// ToOkLab returns OkLab (L 0..1, a/b roughly ±0.4).
func (c Color) ToOkLab() (L, a, b float64) {
	return rgbToOkLab(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
}

// ToOkLCH returns OkLCH (L 0..1, C 0..0.4, H degrees).
func (c Color) ToOkLCH() (L, C, H float64) {
	L, a, b := c.ToOkLab()
	_, C, H = okLabToLCH(L, a, b)
	return L, C, H
}

// OkChroma returns the C component of OkLCH (perceptual chroma, ~0..0.4).
// Useful as a perceptual replacement for HSL saturation in ranking and
// filtering.
func (c Color) OkChroma() float64 {
	_, C, _ := c.ToOkLCH()
	return C
}
