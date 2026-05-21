package color

// Luminance returns relative luminance per WCAG 2.1 in 0..1.
//
// This is the perceptual measure used by sort-by-luminance and the
// contrast helpers; it is not the same as HSL lightness. It lives here
// rather than in any colour-space file because it is a cross-space
// perceptual metric derived directly from linear sRGB.
func (c Color) Luminance() float64 {
	rl := srgbToLinear(float64(c.R) / 255)
	gl := srgbToLinear(float64(c.G) / 255)
	bl := srgbToLinear(float64(c.B) / 255)
	return 0.2126*rl + 0.7152*gl + 0.0722*bl
}
