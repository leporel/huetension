package extract

import (
	"math"
	"sort"

	"github.com/leporel/huetension/internal/color"
)

const (
	// Default target ratio for SoftK merge.
	defaultMergeTargetRatio = 1.5
)

// extractSoft is the classic designer-friendly mode: fixed ΔE threshold
// for cluster merging, with saturation/lightness pre-filter and
// population × saturation ranking.
func extractSoft(pixels []color.Color, k int, opts Options) ([]color.Color, error) {
	return extractSoftInternal(pixels, k, opts,
		func(aggs []softAgg) []softAgg {
			return mergeByDeltaE(aggs, opts.MergeEpsilon)
		})
}

// extractSoftK is the “aggressive” variant: instead of a fixed merge
// threshold, it merges clusters until only k * targetRatio remain,
// guaranteeing that the final palette has exactly k colors and that
// large areas are not fragmented by k-means.
func extractSoftK(pixels []color.Color, k int, opts Options) ([]color.Color, error) {
	// Default target ratio – can be made configurable via opts in the future.
	const defaultMergeTargetRatio = 1.5

	target := int(math.Round(float64(k) * defaultMergeTargetRatio))
	target = max(target, 1)

	return extractSoftInternal(pixels, k, opts,
		func(aggs []softAgg) []softAgg {
			// Cap target to the number of clusters we actually got.
			t := target
			t = min(t, len(aggs))
			return mergeToK(aggs, t)
		})
}

// extractSoftInternal contains the shared pipeline:
// prefilter → over-cluster → convert to aggregates → merge → rank.
func extractSoftInternal(pixels []color.Color, k int, opts Options,
	mergeFn func([]softAgg) []softAgg) ([]color.Color, error) {

	if len(pixels) == 0 {
		return nil, nil
	}

	filtered := softPrefilter(pixels, opts)
	if len(filtered) < k {
		filtered = pixels
	}

	overK := pickOverK(k, len(filtered), softMaxOverK)

	initial, err := extractKMeans(filtered, overK)
	if err != nil || len(initial) == 0 {
		return extractKMeans(filtered, k)
	}

	totalAfterFilter := len(filtered)
	aggs := make([]softAgg, len(initial))
	for i, c := range initial {
		L, a, b := c.ToLab()
		aggs[i] = softAgg{
			L:    L,
			a:    a,
			b:    b,
			pop:  int(math.Round(c.Freq * float64(totalAfterFilter))),
			seed: c,
		}
	}

	aggs = mergeFn(aggs) // delegating merge strategy

	// Rank by population × saturation.
	type ranked struct {
		col    color.Color
		pop    int
		weight float64
	}
	items := make([]ranked, len(aggs))
	for i, a := range aggs {
		c := color.FromLab(a.L, a.a, a.b)
		s := c.Saturation()
		w := float64(a.pop) * (softSaturationBias + math.Pow(s, softSaturationExponent))
		items[i] = ranked{col: c, pop: a.pop, weight: w}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].weight > items[j].weight })

	if len(items) > k {
		items = items[:k]
	}

	totalKept := 0
	for _, it := range items {
		totalKept += it.pop
	}
	if totalKept == 0 {
		totalKept = 1
	}

	out := make([]color.Color, len(items))
	for i, it := range items {
		c := it.col
		c.Freq = float64(it.pop) / float64(totalKept)
		out[i] = c
	}
	return out, nil
}

type softAgg struct {
	L, a, b float64
	pop     int
	seed    color.Color
}

// softPrefilter drops pixels whose lightness or saturation makes them
// designer-irrelevant: pure-black noise from JPEG compression, blown-out
// highlights, and grayscale anti-alias halos.
func softPrefilter(pixels []color.Color, opts Options) []color.Color {
	out := make([]color.Color, 0, len(pixels))
	for _, c := range pixels {
		l := c.Lightness()
		if l < opts.MinLightness || l > opts.MaxLightness {
			continue
		}
		if c.Saturation() < opts.MinSaturation {
			continue
		}
		out = append(out, c)
	}
	return out
}

// mergeByDeltaE collapses clusters whose CIE76 ΔE is below eps. Each merge
// is a population-weighted average, mirroring the way k-means itself would
// have behaved had it placed the centroids one step closer together.
//
// Complexity: O(n^3) worst case but n is N×3 (15-30 typical), not pixel count.
func mergeByDeltaE(aggs []softAgg, eps float64) []softAgg {
	for {
		bestI, bestJ := -1, -1
		bestD := eps
		for i := 0; i < len(aggs); i++ {
			for j := i + 1; j < len(aggs); j++ {
				d := deltaE76(aggs[i], aggs[j])
				if d < bestD {
					bestD = d
					bestI, bestJ = i, j
				}
			}
		}
		if bestI < 0 {
			break
		}
		aggs[bestI] = mergeAgg(aggs[bestI], aggs[bestJ])
		aggs = append(aggs[:bestJ], aggs[bestJ+1:]...)
	}
	return aggs
}

func deltaE76(a, b softAgg) float64 {
	dL := a.L - b.L
	da := a.a - b.a
	db := a.b - b.b
	return math.Sqrt(dL*dL + da*da + db*db)
}

func mergeAgg(a, b softAgg) softAgg {
	pa, pb := float64(a.pop), float64(b.pop)
	sum := pa + pb
	if sum == 0 {
		return a
	}
	return softAgg{
		L:   (a.L*pa + b.L*pb) / sum,
		a:   (a.a*pa + b.a*pb) / sum,
		b:   (a.b*pa + b.b*pb) / sum,
		pop: a.pop + b.pop,
	}
}

// mergeToK greedily merges the two closest clusters (Euclidean ΔE76)
// until the total number of clusters is ≤ k. When the image contains a
// large, gently varying area that was over‑fragmented by k‑means, this
// re‑assembles it into a single bloc so that its true population weight
// is preserved for the final ranking.
//
// Complexity: O(n³) but n is small (≤ overK, typically 15–30).
func mergeToK(aggs []softAgg, k int) []softAgg {
	for len(aggs) > k {
		bestI, bestJ := -1, -1
		bestD := math.MaxFloat64
		for i := 0; i < len(aggs); i++ {
			for j := i + 1; j < len(aggs); j++ {
				d := deltaE76(aggs[i], aggs[j])
				if d < bestD {
					bestD = d
					bestI, bestJ = i, j
				}
			}
		}
		if bestI < 0 {
			break // should never happen unless aggs is empty
		}
		aggs[bestI] = mergeAgg(aggs[bestI], aggs[bestJ])
		aggs = append(aggs[:bestJ], aggs[bestJ+1:]...)
	}
	return aggs
}
