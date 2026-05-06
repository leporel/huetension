package palette

import (
	"math/rand/v2"
	"time"

	"github.com/leporel/huetension/internal/color"
)

// RandomOptions configures Random.
type RandomOptions struct {
	// Count is the palette size (default 5, capped at 32).
	Count int
	// Seed makes generation deterministic when non-zero. Zero seeds use
	// the current monotonic clock.
	Seed uint64
}

const (
	defaultRandomCount = 5
	maxRandomCount     = 32
)

// Random generates a designer-friendly palette by spreading hues evenly
// around the wheel with a small angular jitter, then randomising saturation
// and lightness within ranges that avoid muddy or eye-burning colors.
//
// Algorithm (mirrors color-palette-api/randomPalette but tuned for variety):
//
//	hue        = baseHue + (360/n)*i + jitter ∈ ±10°
//	saturation ∈ [0.50, 0.80]
//	lightness  ∈ [0.40, 0.70]
//
// Harmony-aware random generation lives in the harmony package and reuses
// these defaults via its own helpers.
func Random(opts RandomOptions) *Palette {
	n := opts.Count
	if n <= 0 {
		n = defaultRandomCount
	}
	if n > maxRandomCount {
		n = maxRandomCount
	}

	rng := newRNG(opts.Seed)

	baseHue := rng.Float64() * 360
	step := 360.0 / float64(n)

	cols := make([]color.Color, n)
	for i := range n {
		h := baseHue + step*float64(i) + rng.Float64()*20 - 10
		s := 0.50 + rng.Float64()*0.30
		l := 0.40 + rng.Float64()*0.30
		cols[i] = color.FromHSL(h, s, l)
	}

	return &Palette{
		Colors: cols,
		Metadata: Metadata{
			Method: "random",
			Params: map[string]any{
				"count": n,
				"seed":  opts.Seed,
			},
			GeneratedAt: time.Now(),
		},
	}
}

// newRNG returns a deterministic PCG when seed is non-zero, otherwise one
// seeded from the wall clock.
func newRNG(seed uint64) *rand.Rand {
	if seed == 0 {
		seed = uint64(time.Now().UnixNano())
	}
	// Mix the seed for the second PCG stream so runs with neighbouring
	// integer seeds (1, 2, 3 …) don't produce visually similar palettes.
	return rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
}
