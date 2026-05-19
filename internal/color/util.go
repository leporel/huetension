package color

import (
	"math"
	"strconv"
	"strings"
)

// toUint8 rounds a float to the nearest byte, clamped to [0, 255].
func toUint8(f float64) uint8 {
	switch {
	case math.IsNaN(f):
		return 0
	case f < 0:
		return 0
	case f > 255:
		return 255
	default:
		return uint8(math.Round(f))
	}
}

// clampUnit clamps a 0..1 float.
func clampUnit(f float64) float64 {
	switch {
	case math.IsNaN(f):
		return 0
	case f < 0:
		return 0
	case f > 1:
		return 1
	default:
		return f
	}
}

// formatFloat returns a compact decimal: trailing zeros trimmed,
// at most `prec` digits after the dot.
func formatFloat(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.ContainsRune(s, '.') {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-" {
		s = "0"
	}
	return s
}

func roundInt(v float64) int {
	return int(math.Round(v))
}

// formatAlpha emits the alpha channel using the same compact representation
// browsers use: "1" / "0" / "0.5" etc.
func formatAlpha(a uint8) string {
	switch a {
	case 0:
		return "0"
	case 255:
		return "1"
	}
	return formatFloat(float64(a)/255, 3)
}

// splitArgs splits a CSS function argument string on commas, slashes, and
// whitespace, returning the non-empty fields. A trailing slash is treated as a
// component separator (CSS Color 4 alpha syntax) — its semantics are encoded
// in the returned position, not preserved.
func splitArgs(s string) []string {
	s = strings.ReplaceAll(s, ",", " ")
	s = strings.ReplaceAll(s, "/", " ")
	return strings.Fields(s)
}

// parseChannel reads an rgb channel from a token: integer or float in 0..255,
// or a percentage 0..100%.
func parseChannel(s string) (uint8, error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutSuffix(s, "%"); ok {
		v, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return 0, err
		}
		return toUint8(v / 100 * 255), nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return toUint8(v), nil
}

// parseAlphaToken reads an alpha channel: 0..1 float, or 0..100%.
func parseAlphaToken(s string) (uint8, error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutSuffix(s, "%"); ok {
		v, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return 0, err
		}
		return toUint8(v / 100 * 255), nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return toUint8(v * 255), nil
}

// parseHue reads a hue in degrees (default), turns, radians, or grads.
// Result is normalized to [0, 360).
func parseHue(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	mul := 1.0
	switch {
	case strings.HasSuffix(s, "deg"):
		s = strings.TrimSuffix(s, "deg")
	case strings.HasSuffix(s, "turn"):
		s = strings.TrimSuffix(s, "turn")
		mul = 360
	case strings.HasSuffix(s, "rad"):
		s = strings.TrimSuffix(s, "rad")
		mul = 180.0 / math.Pi
	case strings.HasSuffix(s, "grad"):
		s = strings.TrimSuffix(s, "grad")
		mul = 360.0 / 400.0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	h := math.Mod(v*mul, 360)
	if h < 0 {
		h += 360
	}
	return h, nil
}

// parsePctOrUnit reads a 0..1 number that may be expressed as either a bare
// float (interpreted as 0..1 if ≤ 1, else as a percentage 0..100) or a
// percentage with an explicit '%' suffix. Forgiving for human-typed input.
func parsePctOrUnit(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutSuffix(s, "%"); ok {
		v, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return 0, err
		}
		return v / 100, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if v > 1 {
		v /= 100
	}
	return v, nil
}

// parseFloat is a thin wrapper that trims whitespace.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// isAllHex reports whether s consists only of hex digits.
func isAllHex(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
