package extract

import (
	"sort"

	"github.com/leporel/huetension/internal/color"
)

// extractMedianCut implements classical median-cut quantisation: recursively
// split the largest bucket along its longest RGB axis until k buckets remain,
// then average each bucket. Frequency = bucket size ÷ total pixels.
func extractMedianCut(pixels []color.Color, k int) []color.Color {
	if len(pixels) == 0 {
		return nil
	}
	if k <= 0 {
		k = 1
	}
	if k > len(pixels) {
		k = len(pixels)
	}

	// Copy so we can sort in-place without surprising the caller.
	work := append([]color.Color(nil), pixels...)
	buckets := []bucket{{pixels: work}}

	for len(buckets) < k {
		// Pick the bucket with the most pixels (and at least 2 to split).
		idx := -1
		size := 1
		for i, b := range buckets {
			if len(b.pixels) > size {
				size = len(b.pixels)
				idx = i
			}
		}
		if idx < 0 {
			break
		}
		left, right := buckets[idx].split()
		buckets = append(buckets[:idx], append([]bucket{left, right}, buckets[idx+1:]...)...)
	}

	out := make([]color.Color, len(buckets))
	total := float64(len(pixels))
	for i, b := range buckets {
		c := b.average()
		c.Freq = float64(len(b.pixels)) / total
		out[i] = c
	}
	return out
}

type bucket struct {
	pixels []color.Color
}

// longestAxis returns 0/1/2 for R/G/B based on the widest channel range.
func (b bucket) longestAxis() int {
	if len(b.pixels) == 0 {
		return 0
	}
	minR, maxR := b.pixels[0].R, b.pixels[0].R
	minG, maxG := b.pixels[0].G, b.pixels[0].G
	minB, maxB := b.pixels[0].B, b.pixels[0].B
	for _, c := range b.pixels[1:] {
		if c.R < minR {
			minR = c.R
		}
		if c.R > maxR {
			maxR = c.R
		}
		if c.G < minG {
			minG = c.G
		}
		if c.G > maxG {
			maxG = c.G
		}
		if c.B < minB {
			minB = c.B
		}
		if c.B > maxB {
			maxB = c.B
		}
	}
	rs := int(maxR) - int(minR)
	gs := int(maxG) - int(minG)
	bs := int(maxB) - int(minB)
	switch {
	case rs >= gs && rs >= bs:
		return 0
	case gs >= bs:
		return 1
	default:
		return 2
	}
}

// split partitions the bucket into two halves at the median along the widest
// channel.
func (b bucket) split() (bucket, bucket) {
	axis := b.longestAxis()
	sort.Slice(b.pixels, func(i, j int) bool {
		switch axis {
		case 0:
			return b.pixels[i].R < b.pixels[j].R
		case 1:
			return b.pixels[i].G < b.pixels[j].G
		}
		return b.pixels[i].B < b.pixels[j].B
	})
	mid := len(b.pixels) / 2
	if mid == 0 {
		mid = 1 // ensure both halves are non-empty
	}
	return bucket{pixels: b.pixels[:mid]}, bucket{pixels: b.pixels[mid:]}
}

func (b bucket) average() color.Color {
	if len(b.pixels) == 0 {
		return color.Color{}
	}
	var rs, gs, bs uint64
	for _, c := range b.pixels {
		rs += uint64(c.R)
		gs += uint64(c.G)
		bs += uint64(c.B)
	}
	n := uint64(len(b.pixels))
	return color.New(uint8(rs/n), uint8(gs/n), uint8(bs/n))
}
