package lut

import (
	"fmt"
	"math"
	"sort"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// LUT generation methods. The empty string defaults to MethodKNN for
// backward compatibility with callers that predate the RBF rewrite.
const (
	MethodKNN = "knn"
	MethodRBF = "rbf"
)

// Options controls LUT generation behavior.
type Options struct {
	Size              int    // grid edge per channel (default 33)
	Method            string // "knn" (legacy K-nearest) or "rbf" (smooth Gaussian over all palette colours); empty defaults to "knn"
	IncludeSaturation bool   // shift chroma as well as hue (default false → hue only, chroma frozen)

	// K-NN method knobs (used when Method == "knn" or empty).
	Radius         float64 // OkLab distance pull zone (0..∞, default 0.40)
	Distribution   float64 // falloff shape within radius; 0..1 (default 0.15)
	Intensity      float64 // pull strength / interpolation magnitude; 0..1 (default 0.90)
	BlendNeighbors int     // K-NN: number of nearest palette colours to blend (≥1, default 2)

	// RBF method knobs (used when Method == "rbf"). The RBF path blends
	// in the OkLab a/b plane (Cartesian), so antipodal hues no longer
	// collapse to perpendicular directions via circular mean, and
	// Voronoi-edge seams disappear because every palette colour
	// contributes via a smooth exponential weight.
	Reach     float64 // kernel σ in OkLab — how far each palette colour reaches (>0, default 0.20)
	Sharpness float64 // kernel exponent p in exp(-(d/σ)^p); p=2 Gaussian, lower = softer mix, higher = closer to nearest-only (>0, default 2.0)
	Strength  float64 // pull factor 0..1 (default 0.90)
}

// LUT is a 3D lookup table for color grading.
type LUT struct {
	Size  int
	Nodes []color.Color // len Size^3, index = r + g*Size + b*Size*Size (red fastest)
}

// Generate dispatches to the selected algorithm. Method "" defaults to
// "knn" so pre-existing callers keep their behaviour; the new "rbf"
// path is opt-in and uses the Reach/Sharpness/Strength knobs instead.
func Generate(p *palette.Palette, opts Options) (*LUT, error) {
	if opts.Size < 2 {
		opts.Size = 33
	}
	if opts.Size > 256 {
		return nil, fmt.Errorf("lut: size must be ≤ 256, got %d", opts.Size)
	}
	if len(p.Colors) == 0 {
		return nil, fmt.Errorf("lut: palette is empty")
	}

	switch opts.Method {
	case "", MethodKNN:
		return generateKNN(p, opts)
	case MethodRBF:
		return generateRBF(p, opts)
	}
	return nil, fmt.Errorf("lut: unknown method %q (want %q or %q)", opts.Method, MethodKNN, MethodRBF)
}

// identityLUT emits exact grid identity colours via FromRGB01. Used by
// both methods on the zero-pull short-circuit so the byte-identical
// HALD reference invariant holds regardless of which method is active.
func identityLUT(size int) *LUT {
	total := size * size * size
	result := &LUT{Size: size, Nodes: make([]color.Color, total)}
	for idx := range total {
		r := idx % size
		g := (idx / size) % size
		b := idx / (size * size)
		result.Nodes[idx] = color.FromRGB01(
			float64(r)/float64(size-1),
			float64(g)/float64(size-1),
			float64(b)/float64(size-1),
		)
	}
	return result
}

// generateKNN is the legacy K-nearest + OkLCH circular-mean algorithm.
// Kept verbatim for callers that want the original sharper, layered look —
// the new smooth path is generateRBF.
func generateKNN(p *palette.Palette, opts Options) (*LUT, error) {
	if opts.Radius < 0 {
		return nil, fmt.Errorf("lut: radius must be ≥ 0, got %v", opts.Radius)
	}
	if opts.Distribution < 0 || opts.Distribution > 1 {
		return nil, fmt.Errorf("lut: distribution must be in [0, 1], got %v", opts.Distribution)
	}
	if opts.Intensity < 0 || opts.Intensity > 1 {
		return nil, fmt.Errorf("lut: intensity must be in [0, 1], got %v", opts.Intensity)
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
		return identityLUT(opts.Size), nil
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

// generateRBF blends every palette colour into each LUT node using a
// generalised exponential kernel in OkLab. Working in the a/b Cartesian
// plane (instead of polar C/H) avoids the two failure modes of the K-NN
// path: Voronoi-edge seams (the K-set changes discretely as you cross
// the boundary) and antipodal hue collapse (the circular mean of opposing
// hues shrinks toward zero and atan2 swings perpendicular).
//
// For each node, the weighted target (a,b) is the inverse of the input —
// nodes far from all palette colours fall back to themselves (sum of
// weights → 0). The pull is a straight lerp in (a,b); hue-only mode
// rescales the result to preserve the node's original chroma magnitude.
func generateRBF(p *palette.Palette, opts Options) (*LUT, error) {
	if opts.Reach <= 0 {
		return nil, fmt.Errorf("lut: reach must be > 0, got %v", opts.Reach)
	}
	if opts.Sharpness <= 0 {
		return nil, fmt.Errorf("lut: sharpness must be > 0, got %v", opts.Sharpness)
	}
	if opts.Strength < 0 || opts.Strength > 1 {
		return nil, fmt.Errorf("lut: strength must be in [0, 1], got %v", opts.Strength)
	}

	// Identity short-circuit — mirror the K-NN path so an all-zero pull
	// produces a byte-identical HALD regardless of which method the
	// caller picked.
	if opts.Strength == 0 {
		return identityLUT(opts.Size), nil
	}

	paletteLab := make([][3]float64, len(p.Colors))
	for i, c := range p.Colors {
		L, a, b := c.ToOkLab()
		paletteLab[i] = [3]float64{L, a, b}
	}

	sigma := opts.Reach
	sharpness := opts.Sharpness
	strength := opts.Strength

	total := opts.Size * opts.Size * opts.Size
	result := &LUT{Size: opts.Size, Nodes: make([]color.Color, total)}

	for idx := range total {
		r := idx % opts.Size
		g := (idx / opts.Size) % opts.Size
		b := idx / (opts.Size * opts.Size)

		nodeColor := color.FromRGB01(
			float64(r)/float64(opts.Size-1),
			float64(g)/float64(opts.Size-1),
			float64(b)/float64(opts.Size-1),
		)
		nodeL, nodeA, nodeB := nodeColor.ToOkLab()

		// Generalised exponential kernel: w_i = exp(-(d/σ)^p).
		// p = 2 gives a true Gaussian; p < 2 broadens the kernel (softer
		// blend), p > 2 sharpens it (closer to nearest-only).
		sumW, sumA, sumB := 0.0, 0.0, 0.0
		for i := range paletteLab {
			dL := nodeL - paletteLab[i][0]
			dA := nodeA - paletteLab[i][1]
			dB := nodeB - paletteLab[i][2]
			d := math.Sqrt(dL*dL + dA*dA + dB*dB)
			w := math.Exp(-math.Pow(d/sigma, sharpness))
			sumW += w
			sumA += w * paletteLab[i][1]
			sumB += w * paletteLab[i][2]
		}

		var targetA, targetB float64
		if sumW > 1e-12 {
			targetA = sumA / sumW
			targetB = sumB / sumW
		} else {
			// Node is far from every palette colour relative to σ — leave it alone.
			targetA = nodeA
			targetB = nodeB
		}

		newA := nodeA + (targetA-nodeA)*strength
		newB := nodeB + (targetB-nodeB)*strength

		// Hue-only mode: rescale (newA, newB) so its magnitude matches
		// the node's original chroma. This is the a/b-plane equivalent
		// of "rotate the hue toward the target without restretching the
		// chroma" — purely directional.
		if !opts.IncludeSaturation {
			newMag := math.Hypot(newA, newB)
			nodeMag := math.Hypot(nodeA, nodeB)
			if newMag > 1e-12 {
				scale := nodeMag / newMag
				newA *= scale
				newB *= scale
			}
		}

		result.Nodes[idx] = color.FromOkLab(nodeL, newA, newB)
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
