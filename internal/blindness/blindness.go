// Package blindness simulates the four common forms of color-vision
// deficiency: protanopia, deuteranopia, tritanopia, and achromatopsia.
//
// Implementation: the Brettel–Viénot–Mollon (1997) channel-mixing matrices
// applied to gamma-encoded sRGB. This is the same approximation used by
// Coblis, Stark, and most browser extensions — it is not a strict LMS-space
// reproduction, but the output matches what designers expect to see when
// auditing a palette and is fast enough for live previews.
package blindness

import (
	"fmt"
	"math"

	"github.com/leporel/huetension/internal/color"
)

// Kind identifies a color-vision deficiency.
type Kind string

const (
	// Protan = protanopia (long-wavelength / "red-blind"). ~1% of men.
	Protan Kind = "protan"
	// Deutan = deuteranopia (medium-wavelength / "green-blind"). ~1% of men.
	Deutan Kind = "deutan"
	// Tritan = tritanopia (short-wavelength / "blue-blind"). Rare, ~0.01%.
	Tritan Kind = "tritan"
	// Achroma = achromatopsia (total color blindness). Rare, ~0.003%.
	Achroma Kind = "achroma"
)

// AllKinds is the canonical iteration order used when "all" is requested.
var AllKinds = []Kind{Protan, Deutan, Tritan, Achroma}

// Channel-mixing matrices applied directly to gamma-encoded sRGB (0..1).
// Sourced from the Brettel/Viénot literature (and the colorjack matrix
// reference). Strict LMS-based simulation gives slightly different output;
// these match the Coblis / Stark family of online simulators.
var matrices = map[Kind][3][3]float64{
	Protan: {
		{0.567, 0.433, 0.000},
		{0.558, 0.442, 0.000},
		{0.000, 0.242, 0.758},
	},
	Deutan: {
		{0.625, 0.375, 0.000},
		{0.700, 0.300, 0.000},
		{0.000, 0.300, 0.700},
	},
	Tritan: {
		{0.950, 0.050, 0.000},
		{0.000, 0.433, 0.567},
		{0.000, 0.475, 0.525},
	},
	Achroma: {
		{0.299, 0.587, 0.114},
		{0.299, 0.587, 0.114},
		{0.299, 0.587, 0.114},
	},
}

// Simulate returns the color as it would be perceived under the given
// deficiency. Alpha is preserved untouched.
func Simulate(c color.Color, kind Kind) (color.Color, error) {
	m, ok := matrices[kind]
	if !ok {
		return color.Color{}, fmt.Errorf("blindness: unknown kind %q", string(kind))
	}
	r := float64(c.R)
	g := float64(c.G)
	b := float64(c.B)
	nr := m[0][0]*r + m[0][1]*g + m[0][2]*b
	ng := m[1][0]*r + m[1][1]*g + m[1][2]*b
	nb := m[2][0]*r + m[2][1]*g + m[2][2]*b
	return color.NewWithAlpha(clampByte(nr), clampByte(ng), clampByte(nb), c.A), nil
}

// SimulatePalette runs Simulate over each color in the input slice.
// The result is a fresh slice the caller owns; the input is not mutated.
func SimulatePalette(in []color.Color, kind Kind) ([]color.Color, error) {
	if _, ok := matrices[kind]; !ok {
		return nil, fmt.Errorf("blindness: unknown kind %q", string(kind))
	}
	out := make([]color.Color, len(in))
	for i, c := range in {
		sim, err := Simulate(c, kind)
		if err != nil {
			return nil, err
		}
		out[i] = sim
	}
	return out, nil
}

// SimulateAll returns one slice of simulated colors per kind, keyed by Kind.
// Useful for the "all" output mode in CLI / MCP / web.
func SimulateAll(in []color.Color) (map[Kind][]color.Color, error) {
	out := make(map[Kind][]color.Color, len(AllKinds))
	for _, k := range AllKinds {
		sim, err := SimulatePalette(in, k)
		if err != nil {
			return nil, err
		}
		out[k] = sim
	}
	return out, nil
}

func clampByte(v float64) uint8 {
	switch {
	case math.IsNaN(v), v <= 0:
		return 0
	case v >= 255:
		return 255
	}
	return uint8(v + 0.5)
}
