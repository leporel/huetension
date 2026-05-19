package extract

import (
	"fmt"
	"strings"
)

// SoftPreset is a mood preset for the Soft/SoftK pipeline. One
// value drives both the perceptual pre-filter (OkLCH chroma + OkL bounds)
// and the ranking weights. Empty string means "no preset" — the legacy
// HSL-knob path stays in effect for back-compat.
type SoftPreset string

const (
	// SoftPresetDefault is the explicit "balanced" preset: a mild chroma
	// boost in ranking compared to the legacy no-preset behavior, with
	// broad lightness coverage. Distinct from omitting the preset.
	SoftPresetDefault SoftPreset = "default"
	// SoftPresetColorful favours high-chroma colors with a strong
	// saturation exponent in ranking; broad lightness coverage.
	SoftPresetColorful SoftPreset = "colorful"
	// SoftPresetBright keeps the OkL high (≥ 0.55) and pushes ranking
	// toward the lighter half via OkL preference +0.5.
	SoftPresetBright SoftPreset = "bright"
	// SoftPresetMuted caps OkLCH chroma low and neutralises the
	// chroma-driven ranking (bias↑, exponent↓), so faded / desaturated
	// hues surface first.
	SoftPresetMuted SoftPreset = "muted"
	// SoftPresetDeep favours high chroma with a moderate OkL ceiling —
	// "rich" colors, not pastel and not pure-dark.
	SoftPresetDeep SoftPreset = "deep"
	// SoftPresetDark constrains OkL low and biases ranking toward the
	// darker half via OkL preference −0.6.
	SoftPresetDark SoftPreset = "dark"
)

// AllSoftPresets is the canonical list. Order is stable so it can be
// used directly in CLI usage strings and test iteration.
var AllSoftPresets = []SoftPreset{
	SoftPresetDefault,
	SoftPresetColorful,
	SoftPresetBright,
	SoftPresetMuted,
	SoftPresetDeep,
	SoftPresetDark,
}

// ParseSoftPreset parses a case-insensitive preset name. Empty string is
// returned as ("", nil) so callers can treat "no preset" as a valid
// omitted-input state without special-casing.
func ParseSoftPreset(s string) (SoftPreset, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", nil
	}
	lower := SoftPreset(strings.ToLower(trimmed))
	for _, p := range AllSoftPresets {
		if p == lower {
			return p, nil
		}
	}
	return "", fmt.Errorf("unknown soft preset %q (valid: %s)", s, joinPresetNames())
}

func joinPresetNames() string {
	parts := make([]string, len(AllSoftPresets))
	for i, p := range AllSoftPresets {
		parts[i] = string(p)
	}
	return strings.Join(parts, "|")
}

// softPresetTuning carries every per-preset knob the Soft pipeline reads.
// Zero values for the filter fields mean "no bound" except where the
// mapping explicitly sets a non-zero floor/ceiling.
type softPresetTuning struct {
	MinOkL, MaxOkL       float64
	MinChroma, MaxChroma float64
	SaturationBias       float64
	SaturationExponent   float64
	OkLPreference        float64
}

// softPresetMapping returns the tuning numbers for a preset. Pure
// function — no RNG, no map iteration, deterministic by construction.
// Unknown / empty preset returns the zero tuning; callers must guard
// before reaching this.
func softPresetMapping(p SoftPreset) softPresetTuning {
	switch p {
	case SoftPresetDefault:
		return softPresetTuning{
			MinOkL: 0.10, MaxOkL: 0.92,
			MinChroma: 0.02, MaxChroma: 0,
			SaturationBias: 0.5, SaturationExponent: 1.2,
			OkLPreference: 0.0,
		}
	case SoftPresetColorful:
		return softPresetTuning{
			MinOkL: 0.15, MaxOkL: 0.90,
			MinChroma: 0.08, MaxChroma: 0,
			SaturationBias: 0.3, SaturationExponent: 2.0,
			OkLPreference: 0.0,
		}
	case SoftPresetBright:
		return softPresetTuning{
			MinOkL: 0.55, MaxOkL: 0.95,
			MinChroma: 0.06, MaxChroma: 0,
			SaturationBias: 0.4, SaturationExponent: 1.5,
			OkLPreference: +0.5,
		}
	case SoftPresetMuted:
		return softPresetTuning{
			MinOkL: 0.20, MaxOkL: 0.85,
			MinChroma: 0.01, MaxChroma: 0.10,
			SaturationBias: 1.5, SaturationExponent: 0.4,
			OkLPreference: 0.0,
		}
	case SoftPresetDeep:
		return softPresetTuning{
			MinOkL: 0.20, MaxOkL: 0.60,
			MinChroma: 0.10, MaxChroma: 0,
			SaturationBias: 0.3, SaturationExponent: 1.8,
			OkLPreference: -0.3,
		}
	case SoftPresetDark:
		return softPresetTuning{
			MinOkL: 0.05, MaxOkL: 0.45,
			MinChroma: 0.04, MaxChroma: 0,
			SaturationBias: 0.5, SaturationExponent: 1.2,
			OkLPreference: -0.6,
		}
	}
	return softPresetTuning{}
}
