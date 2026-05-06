package extract

import (
	"sort"

	"github.com/leporel/huetension/internal/color"
)

// rgbBinEntry is one occupied cell in the RGB binning grid: the key (used
// for deterministic ordering), the bin's centroid Color, and pixel count.
type rgbBinEntry struct {
	key   uint32
	rgb   color.Color
	count uint64
}

// binRGB groups pixels into a bits-per-channel RGB grid and returns occupied
// bins SORTED BY KEY for deterministic downstream behaviour.
//
// Why the sort matters: Go map iteration order is randomised between
// processes. Any code path that consumes bin output and depends on ordering
// — k-means++ seeding, FPS picks, sort tie-breaks, DBSCAN visit order —
// would otherwise produce different palettes on every test run. Sorting
// once here closes that hole for every caller.
//
// bits must be in [1, 8]. Each bin's centroid is the per-channel pixel-count
// average of its members.
func binRGB(pixels []color.Color, bits int) []rgbBinEntry {
	bits = clampBits(bits)
	shift := uint(8 - bits)

	type acc struct {
		count            uint64
		rSum, gSum, bSum uint64
	}
	bins := make(map[uint32]*acc)
	for _, c := range pixels {
		key := uint32(c.R>>shift)<<(2*bits) |
			uint32(c.G>>shift)<<bits |
			uint32(c.B>>shift)
		b, ok := bins[key]
		if !ok {
			b = &acc{}
			bins[key] = b
		}
		b.count++
		b.rSum += uint64(c.R)
		b.gSum += uint64(c.G)
		b.bSum += uint64(c.B)
	}

	out := make([]rgbBinEntry, 0, len(bins))
	for k, b := range bins {
		out = append(out, rgbBinEntry{
			key: k,
			rgb: color.New(
				uint8(b.rSum/b.count),
				uint8(b.gSum/b.count),
				uint8(b.bSum/b.count),
			),
			count: b.count,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

func clampBits(bits int) int {
	if bits < 1 {
		return 1
	}
	if bits > 8 {
		return 8
	}
	return bits
}
