// Package gradient produces interpolated color sequences between two or
// more endpoints. Interpolation can be performed in sRGB, CIE Lab, OkLab,
// OkLCH, or HSL — the default (OkLab) is perceptually uniform and avoids
// the muddy mid-tones that plain RGB interpolation produces.
package gradient

import (
	"fmt"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/leporel/huetension/internal/color"
)

// Space chooses the color space in which the linear interpolation is done.
type Space string

const (
	SpaceRGB   Space = "rgb"
	SpaceLab   Space = "lab"
	SpaceOkLab Space = "oklab"
	SpaceOkLCH Space = "oklch"
	SpaceHSL   Space = "hsl"
)

// Easing scales the interpolation parameter t before lerping.
type Easing string

const (
	EasingLinear    Easing = "linear"
	EasingEaseIn    Easing = "ease-in"
	EasingEaseOut   Easing = "ease-out"
	EasingEaseInOut Easing = "ease-in-out"
)

// Options configures Build and MultiStop. Zero values default to OkLab and
// linear easing.
type Options struct {
	Steps  int
	Space  Space
	Easing Easing
}

// Build returns Steps colors interpolating from `from` to `to`. Both
// endpoints are included and bit-exact.
func Build(from, to color.Color, opts Options) ([]color.Color, error) {
	if opts.Steps < 2 {
		return nil, fmt.Errorf("gradient: steps must be >= 2 (got %d)", opts.Steps)
	}
	sp := opts.Space
	if sp == "" {
		sp = SpaceOkLab
	}
	es := opts.Easing
	if es == "" {
		es = EasingLinear
	}

	out := make([]color.Color, opts.Steps)
	out[0] = from
	out[opts.Steps-1] = to

	for i := 1; i < opts.Steps-1; i++ {
		raw := float64(i) / float64(opts.Steps-1)
		t := ease(raw, es)
		c, err := lerp(from, to, t, sp)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// MultiStop blends through stops with even spacing — stop[0] at t=0,
// stop[N-1] at t=1, intermediate stops evenly distributed.
func MultiStop(stops []color.Color, opts Options) ([]color.Color, error) {
	if len(stops) < 2 {
		return nil, fmt.Errorf("gradient: need at least 2 stops (got %d)", len(stops))
	}
	if opts.Steps < len(stops) {
		return nil, fmt.Errorf("gradient: steps (%d) must be >= number of stops (%d)", opts.Steps, len(stops))
	}
	sp := opts.Space
	if sp == "" {
		sp = SpaceOkLab
	}
	es := opts.Easing
	if es == "" {
		es = EasingLinear
	}

	segments := len(stops) - 1
	out := make([]color.Color, opts.Steps)
	out[0] = stops[0]
	out[opts.Steps-1] = stops[segments]

	for i := 1; i < opts.Steps-1; i++ {
		raw := float64(i) / float64(opts.Steps-1)
		t := ease(raw, es)

		// Map t into (segment index, local t within segment).
		scaled := t * float64(segments)
		seg := int(scaled)
		if seg >= segments {
			seg = segments - 1
		}
		local := scaled - float64(seg)

		c, err := lerp(stops[seg], stops[seg+1], local, sp)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// ease maps t in 0..1 through the chosen easing curve.
func ease(t float64, e Easing) float64 {
	switch e {
	case EasingEaseIn:
		return t * t
	case EasingEaseOut:
		u := 1 - t
		return 1 - u*u
	case EasingEaseInOut:
		// Smoothstep (Hermite).
		return t * t * (3 - 2*t)
	}
	return t
}

func lerp(a, b color.Color, t float64, sp Space) (color.Color, error) {
	switch sp {
	case SpaceRGB:
		return lerpRGB(a, b, t), nil
	case SpaceLab:
		return lerpLab(a, b, t), nil
	case SpaceHSL:
		return lerpHSL(a, b, t), nil
	case SpaceOkLab:
		return lerpOkLab(a, b, t), nil
	case SpaceOkLCH:
		return lerpOkLCH(a, b, t), nil
	}
	return color.Color{}, fmt.Errorf("gradient: unknown space %q", string(sp))
}

func lerpRGB(a, b color.Color, t float64) color.Color {
	return color.New(
		lerpByte(a.R, b.R, t),
		lerpByte(a.G, b.G, t),
		lerpByte(a.B, b.B, t),
	)
}

func lerpByte(a, b uint8, t float64) uint8 {
	v := float64(a) + (float64(b)-float64(a))*t
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	}
	return uint8(v + 0.5)
}

// lerpLab uses go-colorful's BlendLab so we share its Lab implementation
// rather than duplicating the math.
func lerpLab(a, b color.Color, t float64) color.Color {
	ca := colorful.Color{R: float64(a.R) / 255, G: float64(a.G) / 255, B: float64(a.B) / 255}
	cb := colorful.Color{R: float64(b.R) / 255, G: float64(b.G) / 255, B: float64(b.B) / 255}
	out := ca.BlendLab(cb, t).Clamped()
	return color.FromRGB01(out.R, out.G, out.B)
}

func lerpHSL(a, b color.Color, t float64) color.Color {
	h1, s1, l1 := a.ToHSL()
	h2, s2, l2 := b.ToHSL()
	h := lerpHue(h1, h2, t)
	s := s1 + (s2-s1)*t
	l := l1 + (l2-l1)*t
	return color.FromHSL(h, s, l)
}

func lerpOkLab(a, b color.Color, t float64) color.Color {
	L1, a1, b1 := a.ToOkLab()
	L2, a2, b2 := b.ToOkLab()
	return color.FromOkLab(
		L1+(L2-L1)*t,
		a1+(a2-a1)*t,
		b1+(b2-b1)*t,
	)
}

func lerpOkLCH(a, b color.Color, t float64) color.Color {
	L1, C1, H1 := a.ToOkLCH()
	L2, C2, H2 := b.ToOkLCH()
	return color.FromOkLCH(
		L1+(L2-L1)*t,
		C1+(C2-C1)*t,
		lerpHue(H1, H2, t),
	)
}

// lerpHue interpolates two hues along the shortest path on the wheel.
func lerpHue(h1, h2, t float64) float64 {
	diff := h2 - h1
	switch {
	case diff > 180:
		h2 -= 360
	case diff < -180:
		h2 += 360
	}
	h := h1 + (h2-h1)*t
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}
	return h
}
