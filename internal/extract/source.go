package extract

import (
	"image"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// AnnotateSources attaches a representative-pixel coordinate to each
// color in p, sampling from img. The coordinate is normalised to [0,1]
// on both axes with the origin at the top-left of img — same coordinate
// system browsers use, so the Web UI's Kuler-style pin overlay can place
// pins by multiplying with the rendered image's bounding box.
//
// Matching is nearest-neighbour in sRGB space (Euclidean over R/G/B as
// integers). It's a single linear scan of img per call, regardless of
// extraction method — every algorithm in this package returns colors
// that exist on or near the source's color manifold, so a nearest match
// is always within a few perceptual units.
//
// Determinism: on ties (multiple pixels equally close to a palette
// entry), the first one encountered in scan order (top-left to
// bottom-right) wins. Same image + same palette → same coordinates.
//
// No-ops when p is nil/empty, img is nil, or the image bounds are
// degenerate. Existing Source pointers are overwritten — intentional;
// if the caller wants to suppress, they can clear Colors[i].Source
// before encoding.
//
// Pixel coordinates are encoded as (x + 0.5) / width to point at pixel
// centres, not corners — the convention WebGL / canvas drag-overlays
// expect.
func AnnotateSources(p *palette.Palette, img image.Image) {
	if p == nil || img == nil {
		return
	}
	n := p.Len()
	if n == 0 {
		return
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return
	}

	// Pre-extract palette R/G/B into ints so the inner loop avoids
	// uint8→int promotion per comparison.
	type target struct{ r, g, b int }
	targets := make([]target, n)
	for i, c := range p.Colors {
		targets[i] = target{int(c.R), int(c.G), int(c.B)}
	}

	type best struct {
		dist int
		x, y int
		seen bool
	}
	bests := make([]best, n)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.FromImageColor(img.At(x, y))
			rp, gp, bp := int(c.R), int(c.G), int(c.B)
			for i, t := range targets {
				dr := rp - t.r
				dg := gp - t.g
				db := bp - t.b
				d := dr*dr + dg*dg + db*db
				if !bests[i].seen || d < bests[i].dist {
					bests[i] = best{dist: d, x: x, y: y, seen: true}
				}
			}
		}
	}

	fw := float64(w)
	fh := float64(h)
	for i := range p.Colors {
		if !bests[i].seen {
			continue
		}
		nx := (float64(bests[i].x-bounds.Min.X) + 0.5) / fw
		ny := (float64(bests[i].y-bounds.Min.Y) + 0.5) / fh
		p.Colors[i].Source = &color.Source{X: nx, Y: ny}
	}
}
