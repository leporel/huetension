// Package contrast computes color-contrast scores using two algorithms:
//
//   - WCAG 2.1 — the legacy luminance-ratio metric (1 .. 21:1) with the
//     familiar AA / AAA pass flags for normal and large text.
//   - APCA   — Accessible Perceptual Contrast Algorithm (W3 working draft
//     0.1.9), a perceptually-uniform Lc score in roughly -108..+106 used
//     by the WCAG 3 silver track.
//
// Callers can pick one or the other via Check(); higher-level callers
// (CLI, MCP, web UI) typically display both.
package contrast

import (
	"fmt"
	"math"

	"github.com/leporel/huetension/internal/color"
)

// Algo names a contrast algorithm.
type Algo string

const (
	AlgoWCAG21 Algo = "wcag21"
	AlgoAPCA   Algo = "apca"
)

// WCAG21Result captures the four pass/fail outcomes plus the raw ratio.
type WCAG21Result struct {
	Algo     Algo    `json:"algo"`
	Ratio    float64 `json:"ratio"`     // 1..21
	AA       bool    `json:"aa"`        // ≥ 4.5
	AALarge  bool    `json:"aa_large"`  // ≥ 3
	AAA      bool    `json:"aaa"`       // ≥ 7
	AAALarge bool    `json:"aaa_large"` // ≥ 4.5
}

// APCAResult carries the signed Lc score and the standard usage flags.
//
// Sign convention: positive Lc means dark text on lighter background
// ("normal polarity"); negative means light text on darker background.
// Magnitudes are designed to be intuitive for designers — 75 is the
// recommended floor for body copy, 60 for content text, 45 for large
// headings, 30 for icons and incidental UI.
type APCAResult struct {
	Algo         Algo    `json:"algo"`
	Lc           float64 `json:"lc"`
	AbsLc        float64 `json:"abs_lc"`
	BodyText     bool    `json:"body_text"`     // |Lc| ≥ 75
	Content      bool    `json:"content"`       // |Lc| ≥ 60
	LargeHeading bool    `json:"large_heading"` // |Lc| ≥ 45
	Icon         bool    `json:"icon"`          // |Lc| ≥ 30
}

// WCAG21 returns the contrast ratio of fg over bg per WCAG 2.1, with the
// four standard pass flags. Order of arguments is irrelevant — the ratio
// is symmetric.
func WCAG21(fg, bg color.Color) WCAG21Result {
	l1 := fg.Luminance()
	l2 := bg.Luminance()
	lighter, darker := l1, l2
	if l2 > l1 {
		lighter, darker = l2, l1
	}
	r := (lighter + 0.05) / (darker + 0.05)
	// Round to two decimals to avoid noisy JSON output.
	r = math.Round(r*100) / 100
	return WCAG21Result{
		Algo:     AlgoWCAG21,
		Ratio:    r,
		AA:       r >= 4.5,
		AALarge:  r >= 3,
		AAA:      r >= 7,
		AAALarge: r >= 4.5,
	}
}

// APCA constants come from the SAPC W3 working draft 0.1.9 ("0.0.98G-4g"
// in the older naming) — the same numbers shipped in the apca-w3 npm
// package and the W3 Color Module 4 community group reference.
const (
	apcaTRC = 2.4

	apcaRco = 0.2126729
	apcaGco = 0.7151522
	apcaBco = 0.0721750

	apcaNormBG  = 0.56
	apcaNormTXT = 0.57
	apcaRevTXT  = 0.62
	apcaRevBG   = 0.65

	apcaBlkThrs   = 0.022
	apcaBlkClmp   = 1.414
	apcaScaleBoW  = 1.14
	apcaLoBoWoff  = 0.027
	apcaScaleWoB  = 1.14
	apcaLoWoBoff  = 0.027
	apcaDeltaYmin = 0.0005
	apcaLoClip    = 0.1
)

// APCA returns the perceptual contrast score for `text` over `bg`.
// Argument order matters: text first, background second.
func APCA(text, bg color.Color) APCAResult {
	txtY := apcaY(text)
	bgY := apcaY(bg)

	// Soft black clamp: lift very-low Y values slightly so the curve
	// behaves sensibly near zero.
	if txtY < apcaBlkThrs {
		txtY += math.Pow(apcaBlkThrs-txtY, apcaBlkClmp)
	}
	if bgY < apcaBlkThrs {
		bgY += math.Pow(apcaBlkThrs-bgY, apcaBlkClmp)
	}

	// Below this difference, the math is meaningless — clamp to zero.
	if math.Abs(bgY-txtY) < apcaDeltaYmin {
		return apcaResult(0)
	}

	var sapc, output float64
	if bgY > txtY {
		// Normal polarity: dark text on light background.
		sapc = (math.Pow(bgY, apcaNormBG) - math.Pow(txtY, apcaNormTXT)) * apcaScaleBoW
		switch {
		case sapc < apcaLoClip:
			output = 0
		default:
			output = sapc - apcaLoBoWoff
		}
	} else {
		// Reverse polarity: light text on dark background.
		sapc = (math.Pow(bgY, apcaRevBG) - math.Pow(txtY, apcaRevTXT)) * apcaScaleWoB
		switch {
		case sapc > -apcaLoClip:
			output = 0
		default:
			output = sapc + apcaLoWoBoff
		}
	}

	return apcaResult(output * 100)
}

func apcaY(c color.Color) float64 {
	r := math.Pow(float64(c.R)/255, apcaTRC)
	g := math.Pow(float64(c.G)/255, apcaTRC)
	b := math.Pow(float64(c.B)/255, apcaTRC)
	return apcaRco*r + apcaGco*g + apcaBco*b
}

func apcaResult(lc float64) APCAResult {
	abs := math.Abs(lc)
	return APCAResult{
		Algo:         AlgoAPCA,
		Lc:           math.Round(lc*100) / 100,
		AbsLc:        math.Round(abs*100) / 100,
		BodyText:     abs >= 75,
		Content:      abs >= 60,
		LargeHeading: abs >= 45,
		Icon:         abs >= 30,
	}
}

// Check returns the WCAG21 result when algo is empty or "wcag21", or the
// APCA result when "apca". The boxed `any` mirrors the JSON shape that
// the CLI / MCP layers will marshal — both result types implement clean
// JSON tags so Marshal will Just Work.
func Check(fg, bg color.Color, algo Algo) (any, error) {
	switch algo {
	case "", AlgoWCAG21:
		return WCAG21(fg, bg), nil
	case AlgoAPCA:
		return APCA(fg, bg), nil
	}
	return nil, fmt.Errorf("contrast: unknown algo %q", string(algo))
}

// SuggestSample is one entry of the lightness sweep run by Suggest.
// L is OkLCH lightness (0..1), Score is the algo's positive contrast
// magnitude (WCAG ratio or |APCA Lc|), and Pass reports whether Score
// reached the target.
type SuggestSample struct {
	L     float64 `json:"l"`
	Score float64 `json:"score"`
	Pass  bool    `json:"pass"`
}

// Suggestion is the passing OkLCH lightness whose distance to the
// original foreground's L is smallest — a minimal nudge that lands on a
// gamut-clamped, displayable color.
type Suggestion struct {
	L     float64 `json:"l"`
	Hex   string  `json:"hex"`
	Score float64 `json:"score"`
}

// SuggestResult is the full output of a lightness-fix search.
// Suggested is nil when no sampled lightness reaches the target.
type SuggestResult struct {
	Algo         Algo            `json:"algo"`
	Target       float64         `json:"target"`
	CurrentL     float64         `json:"current_l"`
	CurrentScore float64         `json:"current_score"`
	Samples      []SuggestSample `json:"samples"`
	Suggested    *Suggestion     `json:"suggested,omitempty"`
}

// Lightness sweep resolution mirrors the web UI's useContrastFix —
// 64 samples between L≈0.03 and L≈0.99 keeps the search dense without
// crowding visualizations or wasting work on degenerate endpoints.
const (
	suggestSamples = 64
	suggestLMin    = 0.03
	suggestLMax    = 0.99
)

// Suggest sweeps the foreground's OkLCH lightness against the background,
// scoring each candidate under algo and returning the passing lightness
// nearest the original — a minimal nudge rather than a jump to the
// extreme. Chroma and hue are held fixed; out-of-gamut (L, C, H) triples
// are clamped to sRGB by FromOkLCH, so every suggestion is a real,
// displayable color.
//
// target is the contrast threshold (WCAG ratio for wcag21, |Lc| for
// apca) and must be > 0. Algo "" defaults to wcag21; "both" is rejected
// because a fix search is single-algo by construction.
func Suggest(fg, bg color.Color, algo Algo, target float64) (SuggestResult, error) {
	if target <= 0 {
		return SuggestResult{}, fmt.Errorf("contrast: suggest target must be > 0, got %g", target)
	}
	switch algo {
	case "", AlgoWCAG21:
		algo = AlgoWCAG21
	case AlgoAPCA:
		// ok
	default:
		return SuggestResult{}, fmt.Errorf("contrast: suggest does not support algo %q (want wcag21|apca)", string(algo))
	}

	L0, C, H := fg.ToOkLCH()
	samples := make([]SuggestSample, suggestSamples)

	var suggested *Suggestion
	bestDist := math.Inf(1)

	for i := range suggestSamples {
		L := suggestLMin + (suggestLMax-suggestLMin)*float64(i)/float64(suggestSamples-1)
		candidate := color.FromOkLCH(L, C, H)
		s := scoreFor(candidate, bg, algo)
		pass := s >= target
		samples[i] = SuggestSample{L: L, Score: s, Pass: pass}

		if pass {
			d := math.Abs(L - L0)
			if d < bestDist {
				bestDist = d
				suggested = &Suggestion{L: L, Hex: candidate.Hex(), Score: s}
			}
		}
	}

	return SuggestResult{
		Algo:         algo,
		Target:       target,
		CurrentL:     L0,
		CurrentScore: scoreFor(fg, bg, algo),
		Samples:      samples,
		Suggested:    suggested,
	}, nil
}

// scoreFor returns the algo's positive contrast magnitude — WCAG ratio
// for wcag21, |Lc| for apca. Used by Suggest so polarity (light text on
// dark vs. dark on light) is treated symmetrically during the sweep.
func scoreFor(fg, bg color.Color, algo Algo) float64 {
	if algo == AlgoAPCA {
		return APCA(fg, bg).AbsLc
	}
	return WCAG21(fg, bg).Ratio
}
