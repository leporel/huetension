// Package harmony generates color harmonies — complementary, analogous,
// triadic, split-complementary, tetradic / square, double-complementary,
// monochromatic, and shades — around a base color.
//
// The base color is always present in the output at a natural index (0 for
// hue-rotation harmonies and shades, the middle slot for analogous and
// odd-count monochromatic) and is preserved bit-exact, sidestepping
// HSL round-trip drift on desaturated inputs.
//
// Hue-rotation harmonies (complementary, triadic, split, tetradic, square,
// double-complementary) accept Options.Count to produce more slots than the
// natural anchor set: extra slots cycle through anchors round-robin and apply
// a deterministic HSV variation table — the Adobe Kuler approach.
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
	DoubleComplementary Type = "double-complementary" // 0/60/180/240 — rectangular
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
	// double-complementary 4); when 0, the natural anchor count is used.
	//
	// For analogous / monochromatic / shades, defaults: 3 / 5 / 5.
	Count int
	// Step is the angular distance between analogous neighbours, in degrees.
	// Default 30°.
	Step float64
}

// hsvDelta is the HSV (S, V) offset applied at a given Kuler-style ring.
type hsvDelta struct{ s, v float64 }

// ringDeltas is the cyclic HSV variation table used to fill slots beyond the
// natural anchor count. Ring 0 is reserved for the unmodified anchors; the
// table starts at ring 1. Five entries → after 5 rings the pattern repeats.
//
// Values are intentionally moderate so a single anchor's variants stay
// visually related. FromHSV clamps so out-of-gamut combinations degrade
// gracefully on near-white / near-black inputs.
var ringDeltas = []hsvDelta{
	{s: 0, v: 0.20},     // ring 1: lighter (tint)
	{s: -0.25, v: 0},    // ring 2: desaturated
	{s: 0, v: -0.20},    // ring 3: darker (shade)
	{s: -0.25, v: 0.15}, // ring 4: soft tint
	{s: 0.15, v: -0.15}, // ring 5: punchier
}

// Generate returns the harmony for base with the given options. The returned
// slice's element 0 is always base unchanged.
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
		return expandIfNeeded(t, rotateHues(base, []float64{0, 150, 210}), opts.Count)
	case Tetradic, Square:
		return expandIfNeeded(t, rotateHues(base, []float64{0, 90, 180, 270}), opts.Count)
	case DoubleComplementary:
		return expandIfNeeded(t, rotateHues(base, []float64{0, 60, 180, 240}), opts.Count)
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

// rotateHues returns base rotated by each offset in offsets. An offset of 0
// returns base verbatim — important so the caller's chosen color survives
// without a HSL round-trip drift.
func rotateHues(base color.Color, offsets []float64) []color.Color {
	h, s, l := base.ToHSL()
	out := make([]color.Color, len(offsets))
	for i, o := range offsets {
		if o == 0 {
			out[i] = base
			continue
		}
		out[i] = color.FromHSL(h+o, s, l)
	}
	return out
}

// expandIfNeeded returns anchors as-is when count is 0 or equals the anchor
// count, errors when count is below the anchor count, and otherwise expands
// the palette by cycling through anchors with HSV ring variations.
func expandIfNeeded(t Type, anchors []color.Color, count int) ([]color.Color, error) {
	n := len(anchors)
	if count == 0 || count == n {
		return anchors, nil
	}
	if count < n {
		return nil, fmt.Errorf("harmony: count=%d too small for %s (min %d)", count, t, n)
	}
	out := make([]color.Color, count)
	copy(out, anchors)
	for i := n; i < count; i++ {
		anchor := anchors[i%n]
		ring := (i/n - 1) % len(ringDeltas)
		d := ringDeltas[ring]
		h, s, v := anchor.ToHSV()
		out[i] = color.FromHSV(h, s+d.s, v+d.v)
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
			out[i] = color.FromHSL(h+offset, s, l)
		}
	}
	return out
}

// monochromatic varies HSL lightness from base.l-30% to base.l+30% across
// count steps, clamped to [10%, 90%]. Mirrors color-palette-api/monochromatic.
func monochromatic(base color.Color, count int) []color.Color {
	if count == 1 {
		return []color.Color{base}
	}
	h, s, l := base.ToHSL()
	out := make([]color.Color, count)
	span := 60.0
	for i := range count {
		lp := l*100 - 30 + (span/float64(count-1))*float64(i)
		switch {
		case lp < 10:
			lp = 10
		case lp > 90:
			lp = 90
		}
		out[i] = color.FromHSL(h, s, lp/100)
	}
	// For odd counts the middle element is at base lightness — substitute
	// base verbatim to avoid HSL round-trip drift on desaturated inputs.
	if count%2 == 1 {
		out[count/2] = base
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
