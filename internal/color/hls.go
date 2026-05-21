package color

import (
	"fmt"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// HLS returns Python colorsys-compatible notation: hls(h, l, s) with all
// values normalised to 0..1.
func (c Color) HLS() string {
	h, s, l := c.toColorful().Hsl()
	if c.A == 255 {
		return fmt.Sprintf("hls(%s %s %s)", formatFloat(h/360, 4), formatFloat(l, 4), formatFloat(s, 4))
	}
	return fmt.Sprintf("hls(%s %s %s / %s)", formatFloat(h/360, 4), formatFloat(l, 4), formatFloat(s, 4), formatAlpha(c.A))
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
