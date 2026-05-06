package extract

import (
	"math/rand/v2"

	"github.com/leporel/huetension/internal/color"
)

// Shared k-means primitives used by both plain k-means (kmeans/okkmeans/soft)
// and the saliency-weighted variant (wkmeans). Centralising this code is
// what gives every k-means caller deterministic output: PRNG is seeded from
// the input slice, and Lloyd iteration order is stable as long as the
// caller passes points in a stable order (binRGB guarantees that).

const (
	lloydMaxIter     = 50
	lloydConvergeEps = 1e-3 // sum-of-squared centroid shifts in the chosen feature space
)

// lloydPoint is one observation in 3D feature space with a non-negative
// weight. Weight semantics are caller-defined: plain k-means uses 1.0 to
// stay frequency-faithful, wkmeans passes saliency to bias clustering toward
// vibrant minor colors.
type lloydPoint struct {
	feat   [3]float64
	weight float64
}

// runLloyd runs k-means++ initialisation followed by Lloyd iteration until
// convergence or lloydMaxIter, returning final centroids and per-point
// assignments. Deterministic given a deterministic rng and stable point
// order.
func runLloyd(points []lloydPoint, k int, rng *rand.Rand) ([][3]float64, []int) {
	centers := lloydInitPP(points, k, rng)
	assignments := make([]int, len(points))
	for range lloydMaxIter {
		changed := lloydAssign(points, centers, assignments)
		newCenters, shift := lloydUpdate(points, assignments, centers)
		copy(centers, newCenters)
		if !changed && shift < lloydConvergeEps {
			break
		}
	}
	return centers, assignments
}

func lloydAssign(points []lloydPoint, centers [][3]float64, assignments []int) bool {
	changed := false
	for i, p := range points {
		best := 0
		bestD := lloydDistSq(p.feat, centers[0])
		for j := 1; j < len(centers); j++ {
			d := lloydDistSq(p.feat, centers[j])
			if d < bestD {
				bestD = d
				best = j
			}
		}
		if assignments[i] != best {
			assignments[i] = best
			changed = true
		}
	}
	return changed
}

func lloydUpdate(points []lloydPoint, assignments []int, prev [][3]float64) ([][3]float64, float64) {
	k := len(prev)
	centers := make([][3]float64, k)
	weights := make([]float64, k)
	for i, p := range points {
		c := assignments[i]
		centers[c][0] += p.feat[0] * p.weight
		centers[c][1] += p.feat[1] * p.weight
		centers[c][2] += p.feat[2] * p.weight
		weights[c] += p.weight
	}
	shift := 0.0
	for j := range k {
		if weights[j] == 0 {
			centers[j] = lloydRescueEmpty(points)
			continue
		}
		centers[j][0] /= weights[j]
		centers[j][1] /= weights[j]
		centers[j][2] /= weights[j]
		shift += lloydDistSq(prev[j], centers[j])
	}
	return centers, shift
}

// lloydRescueEmpty replaces an empty cluster's centroid with the
// highest-weighted point so the cluster slot stays useful instead of
// collapsing into the origin.
func lloydRescueEmpty(points []lloydPoint) [3]float64 {
	best := 0
	for i := range points {
		if points[i].weight > points[best].weight {
			best = i
		}
	}
	return points[best].feat
}

// lloydInitPP picks k initial centers using weight-weighted k-means++. The
// first center is sampled with probability ∝ weight; subsequent ones with
// probability ∝ (weight × min-squared-distance to prior centers). This is
// what beats uniform-random init both on convergence speed and on whether
// minor distinctive colors get their own seed.
func lloydInitPP(points []lloydPoint, k int, rng *rand.Rand) [][3]float64 {
	centers := make([][3]float64, 0, k)
	first := max(lloydSampleByWeight(rng, len(points), func(i int) float64 { return points[i].weight }), 0)
	centers = append(centers, points[first].feat)

	dists := make([]float64, len(points))
	for i, p := range points {
		dists[i] = lloydDistSq(p.feat, centers[0])
	}

	for c := 1; c < k; c++ {
		picked := lloydSampleByWeight(rng, len(points), func(i int) float64 {
			return dists[i] * points[i].weight
		})
		if picked < 0 {
			picked = (first + c) % len(points)
		}
		centers = append(centers, points[picked].feat)
		newCenter := centers[len(centers)-1]
		for i, p := range points {
			d := lloydDistSq(p.feat, newCenter)
			if d < dists[i] {
				dists[i] = d
			}
		}
	}
	return centers
}

// lloydSampleByWeight does inverse-CDF sampling over n indices weighted by
// w(i). Returns -1 when the total weight is non-positive — the caller
// decides whether that's an error or a fallback case.
func lloydSampleByWeight(rng *rand.Rand, n int, w func(i int) float64) int {
	var total float64
	for i := range n {
		total += w(i)
	}
	if total <= 0 {
		return -1
	}
	target := rng.Float64() * total
	cum := 0.0
	for i := range n {
		cum += w(i)
		if cum >= target {
			return i
		}
	}
	return n - 1
}

func lloydDistSq(a, b [3]float64) float64 {
	dL := a[0] - b[0]
	da := a[1] - b[1]
	db := a[2] - b[2]
	return dL*dL + da*da + db*db
}

// seedRNGFromPixels derives a deterministic PCG seed from the pixel slice
// so two runs of the same algorithm on the same image produce identical
// palettes. Used by every k-means caller (plain + weighted) to guarantee
// reproducibility across processes.
func seedRNGFromPixels(pixels []color.Color) *rand.Rand {
	if len(pixels) == 0 {
		return rand.New(rand.NewPCG(0xDEADBEEF, 0xCAFEF00D))
	}
	first, last := pixels[0], pixels[len(pixels)-1]
	seed1 := uint64(len(pixels))*0x9E3779B97F4A7C15 +
		uint64(first.R)<<16 + uint64(first.G)<<8 + uint64(first.B)
	seed2 := uint64(last.R)<<16 + uint64(last.G)<<8 + uint64(last.B) +
		0xDEADBEEFCAFEF00D
	return rand.New(rand.NewPCG(seed1, seed2))
}
