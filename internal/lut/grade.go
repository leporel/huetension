package lut

import (
	"fmt"
	"math"
	"sort"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// Grade-method tuning constants.
const (
	// gradeNeutralChroma is the OkLCH chroma below which a palette colour
	// is treated as neutral (grey / black / white). Such colours have no
	// meaningful hue, so they never become hue anchors — otherwise a
	// palette's off-white would drag half the wheel toward a hue that
	// only exists as 8-bit rounding noise.
	gradeNeutralChroma = 0.02

	// gradeMergeDegrees collapses palette hues closer than this into one
	// anchor. Two near-identical hues would otherwise form a sliver
	// interval whose warp is invisible but whose watershed adds a kink.
	gradeMergeDegrees = 2.0

	// gradeMaxGain is the Schlick gain exponent reached at Compression=1.
	// Exponent 1 is the identity; 6 leaves roughly ±10% of each hue gap
	// as a transition band and snaps everything else onto the spokes.
	// Going higher only steepens that band until it reads as a hard cut,
	// which is the harsh result this method exists to avoid.
	gradeMaxGain = 6.0

	// gradeMuteReachDegrees is the hue distance from the nearest anchor at
	// which Mute reaches full effect. Hues beyond it are equally "not in
	// the palette", so muting flattens out instead of growing forever.
	gradeMuteReachDegrees = 90.0
)

// hueAnchor is one spoke of the palette on the hue wheel — where the
// vectorscope is squeezed toward. Chroma is kept so the saturation
// toggle can pull image chroma toward the palette's own.
type hueAnchor struct {
	hue    float64 // degrees in [0, 360)
	chroma float64 // OkLCH C
}

// hueWarp describes how one input hue moves under the grade map.
type hueWarp struct {
	hue        float64 // warped hue, degrees in [0, 360)
	anchorC    float64 // palette chroma interpolated at the warped position
	anchorDist float64 // degrees from the input hue to its nearest anchor
}

// gradeMapper holds the sorted anchor ring and the gain exponent so the
// per-node loop does a binary search plus a couple of pow calls.
type gradeMapper struct {
	anchors []hueAnchor
	gain    float64
}

// generateGrade squeezes the hue wheel onto the palette's hue spokes — the
// "vectorscope compression" a colourist does by hand — while leaving
// lightness untouched. Unlike the K-NN and RBF paths it never measures
// 3D OkLab distance, so a dark palette colour anchors bright pixels of
// the same hue just as strongly; the palette defines hues, not shades.
//
// Each gap between adjacent anchors is warped onto itself by a monotone
// S-curve (Schlick gain) that is flat at both anchors and steep at the
// midpoint. Monotone means hue order is preserved and gradients never
// fold back on themselves; flat at the anchors means hues near a spoke
// all land on it; the shared derivative at every anchor keeps the whole
// map C1-continuous so no seams appear where gaps meet. Colours in the
// no-man's land between spokes are optionally muted (chroma reduced)
// instead of being shown as intermediate hues.
//
// Rotating a saturated colour to a new hue routinely leaves sRGB, so
// every node is gamut-clipped by reducing chroma at fixed L and H —
// per-channel clamping would shift lightness and hue, i.e. exactly the
// harsh distortion this method is meant to avoid.
func generateGrade(p *palette.Palette, opts Options) (*LUT, error) {
	if !inUnitRange(opts.Compression) {
		return nil, fmt.Errorf("lut: compression must be in [0, 1], got %v", opts.Compression)
	}
	if !inUnitRange(opts.Mute) {
		return nil, fmt.Errorf("lut: mute must be in [0, 1], got %v", opts.Mute)
	}

	mapper := newGradeMapper(p.Colors, opts.Compression)

	// Identity short-circuit keeps the zero-work output byte-identical to
	// the other methods' identity cubes. Hue work needs compression and
	// at least one anchor; chroma work needs mute (with anchors to
	// measure distance from) or the saturation pull. An all-neutral
	// palette with the saturation toggle on still desaturates the image
	// toward the palette's zero chroma, which is a legitimate B&W look.
	hueWork := opts.Compression > 0 && len(mapper.anchors) > 0
	muteWork := opts.Mute > 0 && len(mapper.anchors) > 0
	chromaPull := opts.IncludeSaturation && opts.Compression > 0
	if !hueWork && !muteWork && !chromaPull {
		return identityLUT(opts.Size), nil
	}

	total := opts.Size * opts.Size * opts.Size
	result := &LUT{Size: opts.Size, Nodes: make([]color.Color, total)}

	for idx := range total {
		r := idx % opts.Size
		g := (idx / opts.Size) % opts.Size
		b := idx / (opts.Size * opts.Size)

		node := color.FromRGB01(
			float64(r)/float64(opts.Size-1),
			float64(g)/float64(opts.Size-1),
			float64(b)/float64(opts.Size-1),
		)
		L, C, H := node.ToOkLCH()

		w := mapper.warp(H)
		newC := C
		if chromaPull {
			newC = C + (w.anchorC-C)*opts.Compression
		}
		if muteWork {
			newC *= 1 - opts.Mute*smoothstep(w.anchorDist/gradeMuteReachDegrees)
		}
		newC = color.ClipOkLCHChroma(L, newC, w.hue)

		result.Nodes[idx] = color.FromOkLCH(L, newC, w.hue)
	}

	return result, nil
}

// newGradeMapper builds the anchor ring from the palette. Neutral colours
// are dropped, the rest are sorted by hue (index tie-break keeps the
// output deterministic) and near-duplicates are merged.
func newGradeMapper(colors []color.Color, compression float64) *gradeMapper {
	type indexed struct {
		hueAnchor
		index int
	}
	raw := make([]indexed, 0, len(colors))
	for i, c := range colors {
		_, C, H := c.ToOkLCH()
		if C < gradeNeutralChroma {
			continue
		}
		raw = append(raw, indexed{hueAnchor{hue: H, chroma: C}, i})
	}
	sort.SliceStable(raw, func(i, j int) bool {
		if raw[i].hue != raw[j].hue {
			return raw[i].hue < raw[j].hue
		}
		return raw[i].index < raw[j].index
	})

	anchors := make([]hueAnchor, 0, len(raw))
	for _, a := range raw {
		if n := len(anchors); n > 0 && a.hue-anchors[n-1].hue < gradeMergeDegrees {
			last := &anchors[n-1]
			last.hue = (last.hue + a.hue) / 2
			last.chroma = (last.chroma + a.chroma) / 2
			continue
		}
		anchors = append(anchors, a.hueAnchor)
	}

	return &gradeMapper{
		anchors: anchors,
		gain:    math.Pow(gradeMaxGain, compression),
	}
}

// warp maps one input hue through the anchor ring.
func (m *gradeMapper) warp(h float64) hueWarp {
	n := len(m.anchors)
	if n == 0 {
		return hueWarp{hue: h}
	}

	// Locate the gap [lo, hi) that contains h. Hues below the first anchor
	// belong to the wrap-around gap that starts at the last anchor.
	i := sort.Search(n, func(k int) bool { return m.anchors[k].hue > h }) - 1
	if i < 0 {
		i = n - 1
	}
	lo := m.anchors[i]
	hi := m.anchors[(i+1)%n]

	width := hi.hue - lo.hue
	if width <= 0 {
		width += 360 // wrap-around gap; a single anchor spans the full circle
	}
	t := math.Mod(h-lo.hue+360, 360) / width
	if t > 1 {
		t = 1 // float noise at the wrap seam
	}

	warped := gain(t, m.gain)
	return hueWarp{
		hue:        math.Mod(lo.hue+warped*width, 360),
		anchorC:    lo.chroma + (hi.chroma-lo.chroma)*warped,
		anchorDist: math.Min(t, 1-t) * width,
	}
}

// gain is Schlick's bias/gain S-curve on [0, 1]: identity at a=1, and for
// a>1 flat at both ends with a steep middle. Symmetric about 0.5, so the
// midpoint of every gap is a fixed point — the watershed between spokes.
func gain(t, a float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	ta := math.Pow(t, a)
	ua := math.Pow(1-t, a)
	return ta / (ta + ua)
}

// smoothstep is the Hermite ease on [0, 1], clamped outside.
func smoothstep(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	return x * x * (3 - 2*x)
}

// inUnitRange reports 0 ≤ v ≤ 1; NaN fails both comparisons, so it is
// rejected too.
func inUnitRange(v float64) bool {
	return v >= 0 && v <= 1
}
