// Package analyze renders "colour distribution" strips for an image: four
// horizontal bars showing the image's pixels sorted by hue, luminance,
// saturation, and distance to a primary colour. Used by both the CLI
// `analyze` command and the Web image card.
//
// Each strip averages the sorted pixels into Width buckets in OkLab so the
// resulting band reads as a perceptually smooth gradient rather than a
// noisy run of raw samples.
package analyze

import (
	"errors"
	"fmt"
	"image"

	"github.com/leporel/huetension/internal/imageio"
)

// Metric identifies one of the four colour-distribution sort keys.
type Metric string

const (
	MetricHue        Metric = "hue"
	MetricLuminance  Metric = "luminance"
	MetricSaturation Metric = "saturation"
	MetricDistance   Metric = "distance"
)

// AllMetrics is the canonical strip order used by Strips. Mirroring the
// order on the wire and in the CLI keeps the Web UI and CLI consistent.
var AllMetrics = []Metric{
	MetricHue,
	MetricLuminance,
	MetricSaturation,
	MetricDistance,
}

// DistanceTarget selects which sRGB primary the Distance strip ranks
// against.
type DistanceTarget string

const (
	DistanceRed   DistanceTarget = "red"
	DistanceGreen DistanceTarget = "green"
	DistanceBlue  DistanceTarget = "blue"
)

// Space selects the colour space the hue / luminance / saturation strips
// sort against. OkLCH (default) is perceptually uniform — hue groups by
// what looks similar, lightness ranks darkest-to-brightest the way the
// eye does. HSL is the geometric sRGB cylinder used by classic
// `hsl(h s% l%)` tooling — hues are evenly spaced around the colour
// wheel and lightness is the min/max average, useful when matching
// behaviour with HSL-based design tools. The Distance strip is
// space-independent (Euclidean RGB), so it's unaffected by this knob.
type Space string

const (
	SpaceOkLCH Space = "oklch"
	SpaceHSL   Space = "hsl"
)

// Defaults applied when Options has zero values for the corresponding fields.
const (
	DefaultWidth          = 800
	DefaultHeight         = 96
	DefaultResize         = 256
	DefaultDistanceTarget = DistanceBlue
	DefaultSpace          = SpaceOkLCH
)

// Options tunes Strips. Zero values fall back to the Default* constants.
type Options struct {
	// Width and Height are the per-strip pixel dimensions of the rendered
	// PNG. Width also controls bucket count: each column averages a slice
	// of the sorted pixels.
	Width, Height int
	// Resize bounds the input image's longest side before pixel iteration
	// — keeps large photos from inflating the sort and bucketing cost. 0
	// disables the resize.
	Resize int
	// DistanceTarget picks the sRGB primary the Distance strip ranks
	// against ("red"|"green"|"blue").
	DistanceTarget DistanceTarget
	// Space picks the colour space the hue / luminance / saturation
	// strips sort against ("oklch"|"hsl"). The Distance strip is
	// unaffected. Bucket averaging is always done in OkLab so the
	// rendered bands stay perceptually smooth.
	Space Space
}

// Strip is a single rendered metric band: the PNG bytes plus the metric
// it represents.
type Strip struct {
	Metric Metric
	PNG    []byte
}

// Result is the four strips returned by Strips, always in AllMetrics order.
type Result struct {
	Strips []Strip
}

// Strips renders the four colour-distribution strips for img. The returned
// strips are always ordered by AllMetrics (hue, luminance, saturation,
// distance).
//
// Strips is deterministic: ties in the sort key resolve to the pixel's
// original raster index, and bucketing averages in OkLab.
func Strips(img image.Image, opts Options) (Result, error) {
	if img == nil {
		return Result{}, errors.New("analyze: nil image")
	}

	opts = applyDefaults(opts)
	target, err := normaliseTarget(opts.DistanceTarget)
	if err != nil {
		return Result{}, err
	}

	space, err := normaliseSpace(opts.Space)
	if err != nil {
		return Result{}, err
	}

	work := imageio.Resize(img, opts.Resize)
	pixels := imageio.Pixels(work, 1) // drop fully transparent pixels
	if len(pixels) == 0 {
		return Result{}, errors.New("analyze: image has no opaque pixels")
	}

	// Precompute the per-pixel sort keys + OkLab averaging fields once;
	// renderStrip picks the column it needs per metric.
	cache := buildMetricCache(pixels, target, space)

	out := Result{Strips: make([]Strip, 0, len(AllMetrics))}
	for _, m := range AllMetrics {
		png, err := renderStrip(pixels, cache, m, opts.Width, opts.Height)
		if err != nil {
			return Result{}, fmt.Errorf("analyze: %s strip: %w", m, err)
		}
		out.Strips = append(out.Strips, Strip{Metric: m, PNG: png})
	}
	return out, nil
}

// ParseMetric maps a CLI/JSON string to a Metric. Empty input returns the
// zero Metric and ok=false; callers should treat that as "no selection".
func ParseMetric(s string) (Metric, bool) {
	switch Metric(s) {
	case MetricHue, MetricLuminance, MetricSaturation, MetricDistance:
		return Metric(s), true
	}
	return "", false
}

// ParseDistanceTarget maps a CLI/JSON string to a DistanceTarget. Empty
// input returns the default ("blue") and ok=true so callers can pass the
// result straight into Options.
func ParseDistanceTarget(s string) (DistanceTarget, bool) {
	switch s {
	case "":
		return DefaultDistanceTarget, true
	case string(DistanceRed), string(DistanceGreen), string(DistanceBlue):
		return DistanceTarget(s), true
	}
	return "", false
}

// ParseSpace maps a CLI/JSON string to a Space. Empty input returns the
// default (OkLCH) and ok=true so callers can pass the result straight into
// Options.
func ParseSpace(s string) (Space, bool) {
	switch s {
	case "":
		return DefaultSpace, true
	case string(SpaceOkLCH), string(SpaceHSL):
		return Space(s), true
	}
	return "", false
}

func applyDefaults(o Options) Options {
	if o.Width <= 0 {
		o.Width = DefaultWidth
	}
	if o.Height <= 0 {
		o.Height = DefaultHeight
	}
	if o.Resize == 0 {
		o.Resize = DefaultResize
	}
	// Negative Resize disables the cap (passed straight to imageio.Resize,
	// which treats any side <= 0 as a no-op).
	if o.DistanceTarget == "" {
		o.DistanceTarget = DefaultDistanceTarget
	}
	if o.Space == "" {
		o.Space = DefaultSpace
	}
	return o
}

func normaliseTarget(t DistanceTarget) (DistanceTarget, error) {
	switch t {
	case DistanceRed, DistanceGreen, DistanceBlue:
		return t, nil
	}
	return "", fmt.Errorf("analyze: unknown distance target %q (want red|green|blue)", t)
}

func normaliseSpace(s Space) (Space, error) {
	switch s {
	case SpaceOkLCH, SpaceHSL:
		return s, nil
	}
	return "", fmt.Errorf("analyze: unknown space %q (want oklch|hsl)", s)
}
