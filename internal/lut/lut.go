package lut

import (
	"fmt"
	"math"
	"sort"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// Options controls LUT generation behavior.
type Options struct {
	Size              int     // grid edge per channel (default 33)
	Radius            float64 // OkLab distance pull zone (0..∞, default 0.40)
	Distribution      float64 // falloff shape within radius; 0..1 (default 0.15)
	Intensity         float64 // pull strength / interpolation magnitude; 0..1 (default 0.90)
	BlendNeighbors    int     // K-NN: number of nearest palette colours to blend (≥1, default 2)
	IncludeSaturation bool    // if true, also shift chroma; if false, shift hue only (default false)
}

// LUT is a 3D lookup table for color grading.
type LUT struct {
	Size  int
	Nodes []color.Color // len Size^3, index = r + g*Size + b*Size*Size (red fastest)
}

// Generate creates a LUT from a palette using K-NN weighted blending.
//
// Algorithm: each grid node identity colour is converted to OkLab. The K
// nearest palette colours are found and weighted by inverse distance. The
// node is then pulled toward the weighted target in OkLCH (lightness frozen,
// chroma and hue interpolated by a falloff-shaped intensity).
func Generate(p *palette.Palette, opts Options) (*LUT, error) {
	if opts.Size < 2 {
		opts.Size = 33
	}
	if opts.Size > 256 {
		return nil, fmt.Errorf("lut: size must be ≤ 256, got %d", opts.Size)
	}
	if opts.Radius < 0 {
		return nil, fmt.Errorf("lut: radius must be ≥ 0, got %v", opts.Radius)
	}
	if opts.Distribution < 0 || opts.Distribution > 1 {
		return nil, fmt.Errorf("lut: distribution must be in [0, 1], got %v", opts.Distribution)
	}
	if opts.Intensity < 0 || opts.Intensity > 1 {
		return nil, fmt.Errorf("lut: intensity must be in [0, 1], got %v", opts.Intensity)
	}
	if len(p.Colors) == 0 {
		return nil, fmt.Errorf("lut: palette is empty")
	}
	if opts.BlendNeighbors < 1 {
		opts.BlendNeighbors = 1
	}
	if opts.BlendNeighbors > len(p.Colors) {
		opts.BlendNeighbors = len(p.Colors)
	}

	// Precompute palette in OkLab (for distance) and OkLCH (for target C/H).
	type paletteEntry struct {
		L, a, b float64 // OkLab
		C, H    float64 // OkLCH
	}
	paletteEntries := make([]paletteEntry, len(p.Colors))
	for i, c := range p.Colors {
		L, a, b := c.ToOkLab()
		_, C, H := c.ToOkLCH()
		paletteEntries[i] = paletteEntry{L: L, a: a, b: b, C: C, H: H}
	}

	lutSize := opts.Size * opts.Size * opts.Size
	result := &LUT{
		Size:  opts.Size,
		Nodes: make([]color.Color, lutSize),
	}

	// Identity short-circuit: Intensity=0 means no pull at all, so each node
	// is exactly its grid identity color. Bypassing the OkLab round-trip
	// keeps the output byte-identical to a true identity HALD.
	if opts.Intensity == 0 {
		for idx := range lutSize {
			r := idx % opts.Size
			g := (idx / opts.Size) % opts.Size
			b := (idx / (opts.Size * opts.Size))
			result.Nodes[idx] = color.FromRGB01(
				float64(r)/float64(opts.Size-1),
				float64(g)/float64(opts.Size-1),
				float64(b)/float64(opts.Size-1),
			)
		}
		return result, nil
	}

	type neighbor struct {
		index    int
		distance float64
	}
	distances := make([]neighbor, len(p.Colors))
	weights := make([]float64, opts.BlendNeighbors)

	for idx := range lutSize {
		r := idx % opts.Size
		g := (idx / opts.Size) % opts.Size
		b := (idx / (opts.Size * opts.Size))

		normR := float64(r) / float64(opts.Size-1)
		normG := float64(g) / float64(opts.Size-1)
		normB := float64(b) / float64(opts.Size-1)

		nodeColor := color.FromRGB01(normR, normG, normB)
		nodeL, nodeA, nodeB := nodeColor.ToOkLab()

		// Distances to every palette colour, then sort by distance (stable
		// tie-break on original index so output is deterministic).
		for i, pe := range paletteEntries {
			dL := nodeL - pe.L
			dA := nodeA - pe.a
			dB := nodeB - pe.b
			distances[i] = neighbor{index: i, distance: math.Sqrt(dL*dL + dA*dA + dB*dB)}
		}
		sort.SliceStable(distances, func(i, j int) bool {
			if distances[i].distance != distances[j].distance {
				return distances[i].distance < distances[j].distance
			}
			return distances[i].index < distances[j].index
		})

		k := opts.BlendNeighbors
		kNearest := distances[:k]

		// Falloff shape on the nearest colour's distance.
		d0 := kNearest[0].distance
		normalizedDist := math.Min(d0/opts.Radius, 1.0)
		exponent := 1.0 + (opts.Distribution-0.5)*2
		falloff := math.Pow(1.0-normalizedDist, exponent)
		pullStrength := falloff * opts.Intensity

		// K-NN inverse-distance weights (epsilon avoids /0 on a node that
		// lands exactly on a palette colour).
		const epsilon = 1e-6
		sumWeights := 0.0
		for i := range k {
			weights[i] = 1.0 / (kNearest[i].distance + epsilon)
			sumWeights += weights[i]
		}
		for i := range k {
			weights[i] /= sumWeights
		}

		// Weighted target chroma + weighted circular mean of target hues.
		targetC := 0.0
		targetHX := 0.0
		targetHY := 0.0
		for i := range k {
			pe := paletteEntries[kNearest[i].index]
			targetC += weights[i] * pe.C
			hRad := pe.H * math.Pi / 180
			targetHX += weights[i] * math.Cos(hRad)
			targetHY += weights[i] * math.Sin(hRad)
		}
		targetH := math.Mod(math.Atan2(targetHY, targetHX)*180/math.Pi+360, 360)

		nodeC := math.Hypot(nodeA, nodeB)
		nodeH := math.Mod(math.Atan2(nodeB, nodeA)*180/math.Pi+360, 360)

		newC := nodeC
		if opts.IncludeSaturation {
			newC = nodeC + (targetC-nodeC)*pullStrength
		}
		newH := lerpHue(nodeH, targetH, pullStrength)

		result.Nodes[idx] = color.FromOkLCH(nodeL, newC, newH)
	}

	return result, nil
}

// lerpHue interpolates between two hues using the shortest arc.
func lerpHue(h1, h2, t float64) float64 {
	h1 = math.Mod(h1, 360)
	if h1 < 0 {
		h1 += 360
	}
	h2 = math.Mod(h2, 360)
	if h2 < 0 {
		h2 += 360
	}
	delta := h2 - h1
	if delta > 180 {
		delta -= 360
	} else if delta < -180 {
		delta += 360
	}
	result := h1 + delta*t
	result = math.Mod(result, 360)
	if result < 0 {
		result += 360
	}
	return result
}
