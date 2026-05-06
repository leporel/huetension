package extract

import (
	"math"

	"github.com/leporel/huetension/internal/color"
)

// Saliency proxy used by methods that want to surface vibrant minor colors
// — small but distinctive regions that pure frequency-based clustering
// would drown in their larger neighbours. We don't have spatial pixel
// information at this layer, so we approximate visual saliency from what we
// do have: how many pixels share this color, and how saturated it is.
//
//	saliency = log(count + 1) × (saturationFloor + saturation)^pow
//
// log(count) tames pure-frequency dominance — a 10× larger area only scores
// ~2.3× higher rather than 10×. The saturation term then lets a small
// saturated bin compete with a huge muted one.
//
// `pow` is per-method because methods need different aggressiveness:
//
// This is NOT a true saliency model — Itti-Koch / frequency-tuned saliency
// requires the original 2D pixel grid, which we deliberately flatten in
// extract.Pixels for algorithmic simplicity. If a real saliency map ever
// arrives via Options it can replace this proxy at the call sites.
const saturationFloor = 0.05

func colorSaliency(c color.Color, count uint64, pow float64) float64 {
	return math.Log(float64(count)+1) * math.Pow(saturationFloor+c.Saturation(), pow)
}

// TODO: try a true saliency model instead of the colorSaliency proxy.
// Candidates: Maximum Symmetric Surround (MSS), Frequency-tuned salient
// region detection, Itti-Koch, or similar. A real spatial saliency map
// should better surface small visually-prominent regions that are currently
// underweighted by the (log(count), saturation) heuristic.
