package color

import (
	"fmt"
	"strings"
)

// Parse autodetects the format of a color string and returns the parsed Color.
// It dispatches in this order:
//
//  1. Function notation (anything containing '(') → ParseCSS
//  2. Hex with explicit prefix ('#' or '0x') → ParseHex
//  3. CSS named color (case-insensitive)
//  4. Bare hex digits (3 / 4 / 6 / 8 chars) → ParseHex
//
// Empty strings and unrecognised inputs return an error.
func Parse(s string) (Color, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Color{}, fmt.Errorf("color: empty input")
	}

	if strings.Contains(s, "(") {
		return ParseCSS(s)
	}

	low := strings.ToLower(s)
	if strings.HasPrefix(low, "#") || strings.HasPrefix(low, "0x") {
		return ParseHex(s)
	}

	if c, ok := LookupNamed(s); ok {
		return c, nil
	}

	if isAllHex(s) {
		switch len(s) {
		case 3, 4, 6, 8:
			return ParseHex(s)
		}
	}

	return Color{}, fmt.Errorf("color: cannot parse %q", s)
}
