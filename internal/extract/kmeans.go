package extract

import (
	"fmt"

	"github.com/leporel/huetension/internal/color"
)

// labSpace selects which perceptual space the k-means routine clusters in.
// CIE Lab (D65) and OkLab use the same numerical range — for OkLab we scale
// triplets ×100 so distances and convergence thresholds carry the same
// meaning across spaces.
type labSpace int

const (
	spaceLab labSpace = iota
	spaceOkLab
)

// extractKMeans runs k-means in CIE Lab. Frequency-faithful.
func extractKMeans(pixels []color.Color, k int) ([]color.Color, error) {
	return extractKMeansSpace(pixels, k, spaceLab)
}

// extractOkKMeans runs k-means in OkLab — perceptually uniform, often
// gives better separation on muted / pastel palettes than CIE Lab. Same
// math as extractKMeans, only the feature transform differs.
func extractOkKMeans(pixels []color.Color, k int) ([]color.Color, error) {
	return extractKMeansSpace(pixels, k, spaceOkLab)
}

// extractKMeansSpace is the shared backbone. We use our own deterministic
// Lloyd implementation (lloyd.go) instead of an external library: muesli/
// kmeans seeds its centroids from the global math/rand which is randomised
// per process, so two test runs would produce different palettes. The
// shared runLloyd is seeded from the input pixels and gives identical
// output across runs.
func extractKMeansSpace(pixels []color.Color, k int, sp labSpace) ([]color.Color, error) {
	if len(pixels) == 0 {
		return nil, fmt.Errorf("kmeans: no pixels")
	}
	if k <= 0 {
		return nil, fmt.Errorf("kmeans: k must be positive, got %d", k)
	}

	sampled := subsamplePixels(pixels, maxKMeansSamplePixels)
	if k > len(sampled) {
		k = len(sampled)
	}

	points := make([]lloydPoint, len(sampled))
	for i, c := range sampled {
		points[i] = lloydPoint{
			feat:   featurise(c, sp),
			weight: 1.0, // uniform — every pixel counts equally
		}
	}

	rng := seedRNGFromPixels(sampled)
	centers, assignments := runLloyd(points, k, rng)

	counts := make([]int, k)
	for _, j := range assignments {
		counts[j]++
	}
	total := float64(len(sampled))

	out := make([]color.Color, 0, k)
	for j, n := range counts {
		if n == 0 {
			continue
		}
		c := unfeaturise(centers[j], sp)
		c.Freq = float64(n) / total
		out = append(out, c)
	}
	return out, nil
}

// featurise projects a Color into the chosen feature space, scaling so
// both spaces use a comparable numerical range.
func featurise(c color.Color, sp labSpace) [3]float64 {
	switch sp {
	case spaceOkLab:
		L, a, b := c.ToOkLab()
		return [3]float64{L * 100, a * 100, b * 100}
	}
	L, a, b := c.ToLab()
	return [3]float64{L, a, b}
}

// unfeaturise inverts featurise.
func unfeaturise(coords [3]float64, sp labSpace) color.Color {
	switch sp {
	case spaceOkLab:
		return color.FromOkLab(coords[0]/100, coords[1]/100, coords[2]/100)
	}
	return color.FromLab(coords[0], coords[1], coords[2])
}

// subsamplePixels returns at most max pixels from in, sampling at a fixed
// stride. Deterministic — the stride pattern is purely positional.
func subsamplePixels(in []color.Color, max int) []color.Color {
	if len(in) <= max {
		return in
	}
	stride := len(in) / max
	if stride < 1 {
		stride = 1
	}
	out := make([]color.Color, 0, max)
	for i := 0; i < len(in); i += stride {
		out = append(out, in[i])
		if len(out) == max {
			break
		}
	}
	return out
}
