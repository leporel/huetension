package color

// RYB artist-wheel ⇄ RGB hue conversion.
//
// huetension's color wheel and harmony engine rotate hue on the artist's RYB
// (Red-Yellow-Blue / Itten) wheel rather than the technical HSV/RGB wheel. The
// two wheels agree on the rotation maths — complement = +180°, triad = 120°
// apart — but disagree on which hue sits at a given angle: on the RYB wheel
// the complement of red is green; on the RGB wheel it is cyan.
//
// The bridge is a 1-D piecewise-linear remap between an RYB-wheel hue and an
// HSV/HSL hue. Saturation and lightness are untouched — a color keeps its S
// and L, only its angle around the wheel changes. The remap is monotonic and
// therefore invertible: RYBHueToRGBHue and RGBHueToRYBHue are inverses.
//
// Table source: the Nodebox / Sighack RYB hue-correction table — 25 anchors
// evenly spaced every 15° on the RYB wheel. Itten's primaries land where a
// designer expects: RYB 0°→red, 120°→yellow, 240°→blue, and red's RYB-180°
// complement maps to RGB hue 138° — a green.

// rybToRGBHue is the RGB/HSV hue for each evenly-spaced RYB anchor (index i ⇒
// RYB hue i·15°). The values increase monotonically from 0 to 360 — that is
// what makes the remap invertible.
var rybToRGBHue = [...]float64{
	// RYB 0°..120°: red → orange → yellow
	0, 8, 17, 26, 34, 41, 48, 54, 60,
	// RYB 135°..240°: → green → blue
	81, 103, 123, 138, 155, 171, 187, 204,
	// RYB 255°..360°: → violet → red
	219, 234, 251, 267, 282, 298, 329, 360,
}

// rybAnchorStep is the angular gap between adjacent RYB anchors, derived from
// the table length so it stays correct if the table is ever re-sampled.
const rybAnchorStep float64 = 360.0 / float64(len(rybToRGBHue)-1)

// RYBHueToRGBHue converts a hue on the RYB artist wheel to the equivalent
// HSV/HSL hue (both in degrees). Use it to render the color at an RYB-wheel
// angle. The input is normalized to [0,360); the result lies in [0,360).
func RYBHueToRGBHue(rybHue float64) float64 {
	rybHue = modAngle(rybHue)
	seg := rybHue / rybAnchorStep
	i := int(seg)
	lo, hi := rybToRGBHue[i], rybToRGBHue[i+1]
	return lo + (hi-lo)*(seg-float64(i))
}

// RGBHueToRYBHue is the inverse of RYBHueToRGBHue: it maps an HSV/HSL hue to
// its position on the RYB artist wheel. Use it to place a color's handle on
// the wheel. The input is normalized to [0,360); the result lies in [0,360).
func RGBHueToRYBHue(rgbHue float64) float64 {
	rgbHue = modAngle(rgbHue)
	// rybToRGBHue is strictly increasing, so exactly one segment contains
	// rgbHue. A linear scan over the 24 segments is negligible.
	for i := range len(rybToRGBHue) - 1 {
		lo, hi := rybToRGBHue[i], rybToRGBHue[i+1]
		if rgbHue < hi {
			return (float64(i) + (rgbHue-lo)/(hi-lo)) * rybAnchorStep
		}
	}
	return 360 // unreachable: modAngle keeps rgbHue < 360 = rybToRGBHue[last]
}
