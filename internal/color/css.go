package color

import (
	"fmt"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// ParseCSS parses a CSS-style functional color notation:
// rgb / rgba / hsl / hsla / hsv / hwb / hls / lab / lch / oklab / oklch.
//
// Both legacy comma-separated and modern slash-alpha forms are accepted:
//
//	rgb(255, 0, 0)
//	rgb(255 0 0 / 50%)
//	hsl(120deg 50% 40%)
//	oklch(0.6 0.18 280deg / 0.8)
//	hls(0.33 0.5 1.0)        — Pylette / Python colorsys ordering
func ParseCSS(s string) (Color, error) {
	s = strings.TrimSpace(s)
	open := strings.IndexByte(s, '(')
	closeIdx := strings.LastIndexByte(s, ')')
	if open < 0 || closeIdx < 0 || closeIdx <= open {
		return Color{}, fmt.Errorf("color: not a function color: %q", s)
	}

	fn := strings.ToLower(strings.TrimSpace(s[:open]))
	args := s[open+1 : closeIdx]

	switch fn {
	case "rgb", "rgba":
		return parseRGBFn(args, s)
	case "hsl", "hsla":
		return parseHSLFn(args, s)
	case "hsv", "hsva":
		return parseHSVFn(args, s)
	case "hls":
		return parseHLSFn(args, s)
	case "lab":
		return parseLabFn(args, s)
	case "lch":
		return parseLCHFn(args, s)
	case "oklab":
		return parseOkLabFn(args, s)
	case "oklch":
		return parseOkLCHFn(args, s)
	default:
		return Color{}, fmt.Errorf("color: unsupported function %q", fn)
	}
}

func parseRGBFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: rgb needs 3-4 args, got %d in %q", len(parts), full)
	}
	r, err := parseChannel(parts[0])
	if err != nil {
		return Color{}, fmt.Errorf("color: rgb r: %w", err)
	}
	g, err := parseChannel(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: rgb g: %w", err)
	}
	b, err := parseChannel(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: rgb b: %w", err)
	}
	a := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: rgb alpha: %w", err)
		}
		a = av
	}
	return Color{R: r, G: g, B: b, A: a}, nil
}

func parseHSLFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: hsl needs 3-4 args, got %d in %q", len(parts), full)
	}
	h, err := parseHue(parts[0])
	if err != nil {
		return Color{}, fmt.Errorf("color: hsl h: %w", err)
	}
	sat, err := parsePctOrUnit(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: hsl s: %w", err)
	}
	l, err := parsePctOrUnit(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: hsl l: %w", err)
	}
	a := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: hsl alpha: %w", err)
		}
		a = av
	}
	return fromColorful(colorful.Hsl(h, clampUnit(sat), clampUnit(l)), a), nil
}

func parseHSVFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: hsv needs 3-4 args, got %d in %q", len(parts), full)
	}
	h, err := parseHue(parts[0])
	if err != nil {
		return Color{}, fmt.Errorf("color: hsv h: %w", err)
	}
	sat, err := parsePctOrUnit(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: hsv s: %w", err)
	}
	v, err := parsePctOrUnit(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: hsv v: %w", err)
	}
	a := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: hsv alpha: %w", err)
		}
		a = av
	}
	return fromColorful(colorful.Hsv(h, clampUnit(sat), clampUnit(v)), a), nil
}

// parseHLSFn matches Python's colorsys.hls_to_rgb argument order (h, l, s).
// Each argument is interpreted as a 0..1 unit value, but '%'/'deg' suffixes
// are tolerated for ergonomic parity with hsl().
func parseHLSFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: hls needs 3-4 args, got %d in %q", len(parts), full)
	}
	// Allow either "0.33" or "120deg" for hue.
	var h float64
	if endsWithUnit(parts[0]) {
		hv, err := parseHue(parts[0])
		if err != nil {
			return Color{}, fmt.Errorf("color: hls h: %w", err)
		}
		h = hv
	} else {
		v, err := parseFloat(parts[0])
		if err != nil {
			return Color{}, fmt.Errorf("color: hls h: %w", err)
		}
		// 0..1 fraction → degrees.
		if v <= 1 {
			h = v * 360
		} else {
			h = v
		}
		h = modAngle(h)
	}
	l, err := parsePctOrUnit(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: hls l: %w", err)
	}
	sat, err := parsePctOrUnit(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: hls s: %w", err)
	}
	a := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: hls alpha: %w", err)
		}
		a = av
	}
	// HSL and HLS describe the same model; only argument order differs.
	return fromColorful(colorful.Hsl(h, clampUnit(sat), clampUnit(l)), a), nil
}

func endsWithUnit(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasSuffix(s, "deg") ||
		strings.HasSuffix(s, "turn") ||
		strings.HasSuffix(s, "rad") ||
		strings.HasSuffix(s, "grad") ||
		strings.HasSuffix(s, "%")
}

func modAngle(h float64) float64 {
	h = h - 360*float64(int(h/360))
	if h < 0 {
		h += 360
	}
	return h
}

// ---- formatters ----

// RGB returns CSS rgb()/rgba() notation (legacy comma form, widely supported).
func (c Color) RGB() string {
	if c.A == 255 {
		return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", c.R, c.G, c.B, formatAlpha(c.A))
}

// HSL returns CSS hsl()/hsla() notation.
func (c Color) HSL() string {
	h, s, l := c.toColorful().Hsl()
	if c.A == 255 {
		return fmt.Sprintf("hsl(%s %s%% %s%%)", formatFloat(h, 1), formatFloat(s*100, 1), formatFloat(l*100, 1))
	}
	return fmt.Sprintf("hsla(%s %s%% %s%% / %s)", formatFloat(h, 1), formatFloat(s*100, 1), formatFloat(l*100, 1), formatAlpha(c.A))
}

// HSV returns hsv() notation. Not part of the CSS Color spec but widely used
// in design tools.
func (c Color) HSV() string {
	h, s, v := c.toColorful().Hsv()
	if c.A == 255 {
		return fmt.Sprintf("hsv(%s %s%% %s%%)", formatFloat(h, 1), formatFloat(s*100, 1), formatFloat(v*100, 1))
	}
	return fmt.Sprintf("hsva(%s %s%% %s%% / %s)", formatFloat(h, 1), formatFloat(s*100, 1), formatFloat(v*100, 1), formatAlpha(c.A))
}

// HLS returns Python colorsys-compatible notation: hls(h, l, s) with all
// values normalised to 0..1.
func (c Color) HLS() string {
	h, s, l := c.toColorful().Hsl()
	if c.A == 255 {
		return fmt.Sprintf("hls(%s %s %s)", formatFloat(h/360, 4), formatFloat(l, 4), formatFloat(s, 4))
	}
	return fmt.Sprintf("hls(%s %s %s / %s)", formatFloat(h/360, 4), formatFloat(l, 4), formatFloat(s, 4), formatAlpha(c.A))
}
