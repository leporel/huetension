package exporter

import (
	"bytes"
	"fmt"

	"github.com/leporel/huetension/internal/palette"
)

// renderGGR emits a GIMP gradient (.ggr) — the format GIMP's gradient
// editor reads. The palette's colors become the gradient steps: N colors
// produce N-1 linear RGB segments laid out evenly across [0,1]. The
// layout is:
//
//	GIMP Gradient
//	Name: <name>
//	<segment count>
//	<L> <M> <R>  <r0> <g0> <b0> <a0>  <r1> <g1> <b1> <a1>  <blend> <color>
//	...
//
// L/M/R are the segment's left, middle and right positions; the two RGBA
// quads are its endpoint colors as 0..1 floats; blend 0 = linear,
// color 0 = RGB. GIMP's blend modes have no perceptual space, so a
// palette already interpolated in OkLCH is reproduced faithfully only as
// this piecewise-linear approximation — which is why we segment the
// discrete colors rather than emit two stops.
func renderGGR(p *palette.Palette, opts Options) []byte {
	colors := p.Colors
	n := len(colors)
	// A single-color palette still needs one (flat) segment.
	segs := max(n-1, 1)

	name := paletteName(p, opts)
	if name == "" {
		name = opts.Name
	}

	var buf bytes.Buffer
	buf.WriteString("GIMP Gradient\n")
	fmt.Fprintf(&buf, "Name: %s\n", name)
	fmt.Fprintf(&buf, "%d\n", segs)
	for i := range segs {
		left := float64(i) / float64(segs)
		right := float64(i+1) / float64(segs)
		mid := (left + right) / 2

		a := colors[i]
		b := a
		if i+1 < n {
			b = colors[i+1]
		}
		fmt.Fprintf(&buf,
			"%.6f %.6f %.6f %.6f %.6f %.6f 1.000000 %.6f %.6f %.6f 1.000000 0 0\n",
			left, mid, right,
			channel01(a.R), channel01(a.G), channel01(a.B),
			channel01(b.R), channel01(b.G), channel01(b.B))
	}
	return buf.Bytes()
}

// channel01 converts a 0..255 color channel to GIMP's 0..1 float scale.
func channel01(c uint8) float64 {
	return float64(c) / 255
}
