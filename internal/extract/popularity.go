package extract

import (
	"sort"

	"github.com/leporel/huetension/internal/color"
)

// Popularity quantisation: drop the low bits of each channel so colors fall
// into a coarse grid, count pixels per bucket, return the top-K buckets
// averaged into their centroid. Trivial, deterministic, and the algorithm
// of choice for indexed/pixel-art images where exact histogram matters.

const defaultPopularityBits = 5 // 32 levels per channel → 32³ ≈ 32 768 buckets

func extractPopularity(pixels []color.Color, k, bits int) []color.Color {
	if k <= 0 {
		k = 1
	}
	if bits <= 0 || bits > 8 {
		bits = defaultPopularityBits
	}

	bins := binRGB(pixels, bits)
	if len(bins) == 0 {
		return nil
	}

	// Count descending; key ascending breaks ties so equal-count bins always
	// land in the same order across runs.
	sort.SliceStable(bins, func(i, j int) bool {
		if bins[i].count != bins[j].count {
			return bins[i].count > bins[j].count
		}
		return bins[i].key < bins[j].key
	})

	if len(bins) > k {
		bins = bins[:k]
	}

	var total uint64
	for _, b := range bins {
		total += b.count
	}

	out := make([]color.Color, len(bins))
	for i, b := range bins {
		c := b.rgb
		if total > 0 {
			c.Freq = float64(b.count) / float64(total)
		}
		out[i] = c
	}
	return out
}
