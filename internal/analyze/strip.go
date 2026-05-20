package analyze

import (
	"bytes"
	"fmt"
	"image"
	stdcolor "image/color"
	"image/draw"
	"image/png"
	"sort"

	"github.com/leporel/huetension/internal/color"
)

// Combined stacks the four strips vertically into one PNG, in AllMetrics
// order (hue → luminance → saturation → distance). All strips must share
// the same width; the combined image is Width × sum(heights).
//
// Used by the CLI's `--strip all` mode. The Web frontend renders the four
// strips individually so it has no need for this; exporting it here keeps
// the image-stacking logic next to its only callers.
func Combined(strips []Strip) ([]byte, error) {
	if len(strips) == 0 {
		return nil, fmt.Errorf("analyze: no strips to combine")
	}
	images := make([]image.Image, len(strips))
	var totalH int
	var width int
	for i, s := range strips {
		img, err := png.Decode(bytes.NewReader(s.PNG))
		if err != nil {
			return nil, fmt.Errorf("analyze: decode %s strip: %w", s.Metric, err)
		}
		b := img.Bounds()
		if i == 0 {
			width = b.Dx()
		} else if b.Dx() != width {
			return nil, fmt.Errorf("analyze: width mismatch on %s strip (%d vs %d)", s.Metric, b.Dx(), width)
		}
		images[i] = img
		totalH += b.Dy()
	}

	out := image.NewRGBA(image.Rect(0, 0, width, totalH))
	var y int
	for _, img := range images {
		b := img.Bounds()
		rect := image.Rect(0, y, width, y+b.Dy())
		draw.Draw(out, rect, img, b.Min, draw.Src)
		y += b.Dy()
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("analyze: png encode combined: %w", err)
	}
	return buf.Bytes(), nil
}

// metricCache is per-pixel data shared across all four strips. Building it
// once amortises the colour-space conversions over the strips that need
// them.
type metricCache struct {
	// L, A, B are OkLab components (parallel to the pixel slice). Always
	// populated regardless of Space — the bucket averaging is always done
	// in OkLab so the rendered bands stay perceptually smooth.
	L, A, B []float64
	// Lightness / Chroma / Hue are the per-pixel sort keys for the
	// luminance / saturation / hue strips. Populated from OkLCH when
	// Space == SpaceOkLCH; from HSL otherwise. Distance is unaffected by
	// Space.
	Lightness []float64
	Chroma    []float64
	Hue       []float64
	// Dist is the squared Euclidean RGB distance to the active primary
	// (no sqrt — order-preserving and avoids overflow).
	Dist []uint32
}

func buildMetricCache(pixels []color.Color, target DistanceTarget, space Space) *metricCache {
	n := len(pixels)
	c := &metricCache{
		L:         make([]float64, n),
		A:         make([]float64, n),
		B:         make([]float64, n),
		Lightness: make([]float64, n),
		Chroma:    make([]float64, n),
		Hue:       make([]float64, n),
		Dist:      make([]uint32, n),
	}
	var tr, tg, tb int32
	switch target {
	case DistanceRed:
		tr = 255
	case DistanceGreen:
		tg = 255
	case DistanceBlue:
		tb = 255
	}
	for i, p := range pixels {
		l, a, b := p.ToOkLab()
		c.L[i] = l
		c.A[i] = a
		c.B[i] = b
		switch space {
		case SpaceHSL:
			hh, ss, ll := p.ToHSL()
			c.Hue[i] = hh
			c.Chroma[i] = ss
			c.Lightness[i] = ll
		default: // SpaceOkLCH
			L, C, H := p.ToOkLCH()
			c.Lightness[i] = L
			c.Chroma[i] = C
			c.Hue[i] = H
		}
		dr := int32(p.R) - tr
		dg := int32(p.G) - tg
		db := int32(p.B) - tb
		c.Dist[i] = uint32(dr*dr + dg*dg + db*db)
	}
	return c
}

// renderStrip sorts the pixels by the requested metric, buckets them into
// `width` columns, averages each bucket in OkLab, paints a Width×Height
// PNG.
func renderStrip(pixels []color.Color, cache *metricCache, metric Metric, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid dimensions %dx%d", width, height)
	}
	n := len(pixels)
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	switch metric {
	case MetricHue:
		// Stable-sort on Hue (ascending). Ties tie-break on original
		// index (sort.SliceStable preserves order on equal keys).
		sort.SliceStable(indices, func(i, j int) bool {
			return cache.Hue[indices[i]] < cache.Hue[indices[j]]
		})
	case MetricLuminance:
		sort.SliceStable(indices, func(i, j int) bool {
			return cache.Lightness[indices[i]] < cache.Lightness[indices[j]]
		})
	case MetricSaturation:
		sort.SliceStable(indices, func(i, j int) bool {
			return cache.Chroma[indices[i]] < cache.Chroma[indices[j]]
		})
	case MetricDistance:
		// Ascending = closest first.
		sort.SliceStable(indices, func(i, j int) bool {
			return cache.Dist[indices[i]] < cache.Dist[indices[j]]
		})
	default:
		return nil, fmt.Errorf("unknown metric %q", metric)
	}

	columns := bucketColumns(indices, cache, width)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		rect := image.Rect(x, 0, x+1, height)
		draw.Draw(img, rect, &image.Uniform{C: columns[x]}, image.Point{}, draw.Src)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}
	return buf.Bytes(), nil
}

// bucketColumns groups the sorted-index slice into `width` evenly-sized
// ranges and averages each range's OkLab components, returning the bucket
// colour for each column. Empty buckets (possible when width > N) inherit
// the previous column's colour.
func bucketColumns(sorted []int, cache *metricCache, width int) []stdcolor.RGBA {
	n := len(sorted)
	out := make([]stdcolor.RGBA, width)
	var last stdcolor.RGBA = stdcolor.RGBA{A: 255}
	for x := 0; x < width; x++ {
		start := x * n / width
		end := (x + 1) * n / width
		if end > n {
			end = n
		}
		if end <= start {
			// Empty bucket — reuse the previous column. For x=0 this
			// keeps a sane default (black opaque) until the first
			// non-empty bucket; in practice n >> width so this is rare.
			out[x] = last
			continue
		}
		var sumL, sumA, sumB float64
		for _, idx := range sorted[start:end] {
			sumL += cache.L[idx]
			sumA += cache.A[idx]
			sumB += cache.B[idx]
		}
		count := float64(end - start)
		avg := color.FromOkLab(sumL/count, sumA/count, sumB/count)
		last = stdcolor.RGBA{R: avg.R, G: avg.G, B: avg.B, A: 255}
		out[x] = last
	}
	return out
}
