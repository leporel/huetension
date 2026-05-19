// Package harmony generates color harmonies — complementary, analogous,
// triadic, split-complementary, tetradic / square, double-complementary,
// compound, monochromatic, and shades — around a base color.
//
// The base color is always present in the output at a natural index (0 for
// hue-rotation harmonies, shades, and monochromatic; the middle slot for
// analogous) and is preserved bit-exact, sidestepping HSL round-trip drift
// on desaturated inputs.
//
// Hue-rotation harmonies (complementary, triadic, split, tetradic, square,
// double-complementary, compound) accept Options.Count to produce more slots
// than the natural anchor set: the extra slots are grouped into cycles that
// reuse the anchor hues round-robin, each cycle a progressively more muted
// echo — its saturation and value evenly subdivided down towards zero.
//
// Hue rotation happens on the artist's RYB (Red-Yellow-Blue / Itten) wheel,
// not the technical HSV wheel — so the complement of red is green, not cyan.
// The angle offsets below are RYB-wheel degrees; the RYB↔RGB hue mapping
// lives in internal/color/ryb.go.
package harmony

import (
	"fmt"

	"github.com/leporel/huetension/internal/color"
)

// Type identifies a harmony algorithm.
type Type string

const (
	Complementary       Type = "complementary"
	Analogous           Type = "analogous"
	Triadic             Type = "triadic"
	Split               Type = "split-complementary"
	Tetradic            Type = "tetradic"             // 0/90/180/270 — matches color-palette-api
	Square              Type = "square"               // alias for Tetradic
	DoubleComplementary Type = "double-complementary" // 0/38/180/218 — rectangular
	Compound            Type = "compound"             // 0/-30/-150/180 — base + complement clusters
	Monochromatic       Type = "monochromatic"
	Shades              Type = "shades"
)

// Options tunes count- and angle-bearing harmonies. Zero values fall back to
// type-specific defaults.
type Options struct {
	// Count is the palette size.
	//
	// For hue-rotation harmonies it must be ≥ the natural anchor count
	// (complementary 2, triadic 3, split 3, tetradic/square 4,
	// double-complementary 4, compound 4); when 0, the natural anchor count
	// is used.
	//
	// For analogous / monochromatic / shades, defaults: 3 / 5 / 5.
	Count int
	// Step is the angular distance between analogous neighbours, in degrees.
	// Default 30°.
	Step float64
}

// monoMinV is the value floor — a ramped slot never drops fully to black,
// keeping deep shades legible.
const monoMinV = 0.20

// monoRamp returns the HSV saturation and value for Monochromatic ramp
// position e (e ≥ 1) around a base HSV, given the per-step fraction step.
//
// Saturation is a reflecting triangle wave: it walks away from baseS by step
// and, when a step would cross the [0,1] gamut edge, flips direction and
// bounces back inward — so it never clips (step 0.20, base 0.50 → 0.30, 0.10,
// 0.30, 0.50). Value instead clamps: it dips one step below base on position
// 1, then ramps away from base — towards white for a dark base, towards black
// for a bright one (anti-correlated, so saturated slots read dark and pale
// slots read light).
//
// step is supplied by monochromatic as 1/count, so a longer palette
// subdivides the ramp more finely: a 5-slot palette steps 20%, a 10-slot
// palette 10%.
func monoRamp(baseS, baseV, step float64, e int) (s, v float64) {
	s = baseS
	dir := -1.0
	for range e {
		next := s + dir*step
		if next < 0 || next > 1 {
			dir = -dir // bounce off the gamut edge
			next = s + dir*step
		}
		s = next
	}
	vDir := 1.0 // dark base ramps lighter
	if baseV >= 0.5 {
		vDir = -1.0 // bright base ramps darker
	}
	if e == 1 {
		v = baseV - step
	} else {
		v = baseV + vDir*step*float64(e)
	}
	switch {
	case v < monoMinV:
		v = monoMinV
	case v > 1:
		v = 1
	}
	return s, v
}

// Generate returns the harmony for base with the given options. base sits at
// element 0 for the hue-rotation harmonies, Shades, and Monochromatic, and at
// the centre for Analogous (even counts spread it between slots).
func Generate(t Type, base color.Color, opts Options) ([]color.Color, error) {
	switch t {
	case Complementary:
		return expandIfNeeded(t, rotateHues(base, []float64{0, 180}), opts.Count)
	case Analogous:
		count := opts.Count
		if count <= 0 {
			count = 3
		}
		step := opts.Step
		if step == 0 {
			step = 30
		}
		return analogous(base, count, step), nil
	case Triadic:
		return expandIfNeeded(t, rotateHues(base, []float64{0, 120, 240}), opts.Count)
	case Split:
		// +199 before +161 — the anchor nearer the complement comes first,
		// matching the reference tool's slot order.
		return expandIfNeeded(t, rotateHues(base, []float64{0, 199, 161}), opts.Count)
	case Tetradic, Square:
		return expandIfNeeded(t, rotateHues(base, []float64{0, 90, 180, 270}), opts.Count)
	case DoubleComplementary:
		// Two 38°-wide clusters mirrored across the complement: {base, base+38}
		// and {base+180, base+218}.
		return expandIfNeeded(t, rotateHues(base, []float64{0, 38, 180, 218}), opts.Count)
	case Compound:
		// Offsets mirror the base+30 / complement−30 clusters so the output
		// is base, base−30, base−150, base+180 — the reference Compound shape.
		return expandIfNeeded(t, rotateHues(base, []float64{0, -30, -150, 180}), opts.Count)
	case Monochromatic:
		count := opts.Count
		if count <= 0 {
			count = 5
		}
		return monochromatic(base, count), nil
	case Shades:
		count := opts.Count
		if count <= 0 {
			count = 5
		}
		return shades(base, count), nil
	}
	return nil, fmt.Errorf("harmony: unknown type %q", string(t))
}

// rotateHues returns base rotated by each offset in offsets. Offsets are
// degrees on the RYB artist wheel (see rotateHueRYB). An offset of 0 returns
// base verbatim — important so the caller's chosen color survives without HSL
// round-trip drift.
func rotateHues(base color.Color, offsets []float64) []color.Color {
	h, s, l := base.ToHSL()
	out := make([]color.Color, len(offsets))
	for i, o := range offsets {
		if o == 0 {
			out[i] = base
			continue
		}
		out[i] = color.FromHSL(rotateHueRYB(h, o), s, l)
	}
	return out
}

// rotateHueRYB rotates an HSL/HSV hue by offsetDeg degrees on the artist's RYB
// wheel: the hue is mapped to its RYB-wheel position, advanced, then mapped
// back to an HSV hue. This is what makes "complementary" land on green rather
// than cyan. Both color conversions normalize their input, so offsetDeg may
// be any sign or magnitude.
func rotateHueRYB(rgbHue, offsetDeg float64) float64 {
	return color.RYBHueToRGBHue(color.RGBHueToRYBHue(rgbHue) + offsetDeg)
}

// expandIfNeeded returns anchors as-is when count is 0 or equals the anchor
// count, errors when count is below the anchor count, and otherwise fills the
// extra slots.
//
// Extra slots are grouped into cycles of n: every slot in a cycle reuses an
// anchor hue round-robin and shares one saturation/value level. The cycles
// evenly subdivide the base's S/V down towards zero, so a longer palette just
// spaces its muted echoes more finely — 1 extra cycle lands at 50% of the
// base, 2 cycles at 67%/33%, 3 at 75%/50%/25%. Value is floored at monoMinV
// so a deep cycle never collapses to black.
func expandIfNeeded(t Type, anchors []color.Color, count int) ([]color.Color, error) {
	n := len(anchors)
	if count == 0 || count == n {
		return anchors, nil
	}
	if count < n {
		return nil, fmt.Errorf("harmony: count=%d too small for %s (min %d)", count, t, n)
	}
	// rotateHues varies only hue, so every anchor shares the base's S/V —
	// anchors[0] (the verbatim base) carries the ramp's reference HSV.
	_, baseS, baseV := anchors[0].ToHSV()
	cycles := (count - 1) / n // ceil((count-n) / n)
	out := make([]color.Color, count)
	copy(out, anchors)
	for i := n; i < count; i++ {
		cycle := (i-n)/n + 1
		f := 1 - float64(cycle)/float64(cycles+1)
		h, _, _ := anchors[i%n].ToHSV()
		v := baseV * f
		if v < monoMinV {
			v = monoMinV
		}
		out[i] = color.FromHSV(h, baseS*f, v)
	}
	return out, nil
}

func analogous(base color.Color, count int, step float64) []color.Color {
	h, s, l := base.ToHSL()
	out := make([]color.Color, count)
	half := float64(count-1) / 2
	for i := range count {
		offset := (float64(i) - half) * step
		if offset == 0 {
			out[i] = base
		} else {
			out[i] = color.FromHSL(rotateHueRYB(h, offset), s, l)
		}
	}
	return out
}

// monochromatic returns count HSV variants of base sharing its hue. Slot 0
// is base verbatim (no HSV round-trip drift on desaturated inputs); every
// later slot applies the monoRamp tint/shade ramp. The ramp step is 1/count,
// so the palette spans the same saturation/value range whatever its length.
func monochromatic(base color.Color, count int) []color.Color {
	if count == 1 {
		return []color.Color{base}
	}
	h, s, v := base.ToHSV()
	step := 1.0 / float64(count)
	out := make([]color.Color, count)
	out[0] = base
	for i := 1; i < count; i++ {
		rs, rv := monoRamp(s, v, step, i)
		out[i] = color.FromHSV(h, rs, rv)
	}
	return out
}

// shades returns count darker variants of base, walking from base.l down to
// 5% lightness.
func shades(base color.Color, count int) []color.Color {
	if count == 1 {
		return []color.Color{base}
	}
	h, s, l := base.ToHSL()
	startPct := l * 100
	out := make([]color.Color, count)
	out[0] = base
	for i := 1; i < count; i++ {
		lp := startPct - (startPct-5)*float64(i)/float64(count-1)
		out[i] = color.FromHSL(h, s, lp/100)
	}
	return out
}
