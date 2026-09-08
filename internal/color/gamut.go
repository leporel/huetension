package color

// gamutTolerance is how far outside [0, 1] a linear channel may sit and
// still count as displayable. Hides float noise from the OkLab round
// trip so pure sRGB primaries are not reported as out of gamut.
const gamutTolerance = 1e-4

// OkLabInGamut reports whether the OkLab triple maps inside the sRGB
// cube without clamping.
func OkLabInGamut(L, a, b float64) bool {
	rl, gl, bl := okLabToLinearRGB(L, a, b)
	return inUnitRange(rl) && inUnitRange(gl) && inUnitRange(bl)
}

// ClipOkLCHChroma returns the largest chroma ≤ C at which (L, C, H) is
// still inside sRGB. Lightness and hue are held fixed, so reducing chroma
// is the perceptually safest way to bring a rotated or boosted colour
// back into gamut — per-channel RGB clamping would shift both.
//
// The search is a plain bisection: gamut boundaries along a constant-L,
// constant-H ray are single crossings for sRGB, so the halving converges
// to the boundary within the tolerance in a fixed number of steps.
func ClipOkLCHChroma(L, C, H float64) float64 {
	if C <= 0 {
		return 0
	}
	_, a, b := okLCHToLab(L, C, H)
	if OkLabInGamut(L, a, b) {
		return C
	}
	lo, hi := 0.0, C
	// 16 halvings on a chroma range ≤ 0.4 land within 1e-5 — below the
	// 8-bit quantisation step, so more iterations buy nothing visible.
	for range 16 {
		mid := (lo + hi) / 2
		_, a, b = okLCHToLab(L, mid, H)
		if OkLabInGamut(L, a, b) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

func inUnitRange(v float64) bool {
	return v >= -gamutTolerance && v <= 1+gamutTolerance
}
