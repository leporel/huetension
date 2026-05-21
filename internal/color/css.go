package color

import (
	"fmt"
	"strings"
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
//	hls(0.33 0.5 1.0)        — Python colorsys ordering
//
// Each functional notation is parsed by its dedicated colour-space file
// (rgb.go, hsl.go, hsv.go, hls.go, lab.go, oklab.go); this file only routes.
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
