package color

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseHex accepts hex notation in any of these forms (case-insensitive,
// surrounding whitespace tolerated):
//
//	#rgb       #rgba
//	#rrggbb    #rrggbbaa
//	0xrrggbb   0xrrggbbaa
//	rrggbb     rrggbbaa     (no prefix)
func ParseHex(s string) (Color, error) {
	raw := strings.ToLower(strings.TrimSpace(s))
	raw = strings.TrimPrefix(raw, "#")
	raw = strings.TrimPrefix(raw, "0x")

	var hex string
	switch len(raw) {
	case 3:
		hex = string([]byte{raw[0], raw[0], raw[1], raw[1], raw[2], raw[2]})
	case 4:
		hex = string([]byte{raw[0], raw[0], raw[1], raw[1], raw[2], raw[2], raw[3], raw[3]})
	case 6, 8:
		hex = raw
	default:
		return Color{}, hexErr(s)
	}

	bytes, err := parseHexBytes(hex)
	if err != nil {
		return Color{}, hexErr(s)
	}

	c := Color{R: bytes[0], G: bytes[1], B: bytes[2], A: 255}
	if len(bytes) == 4 {
		c.A = bytes[3]
	}
	return c, nil
}

func parseHexBytes(raw string) ([]uint8, error) {
	n := len(raw) / 2
	out := make([]uint8, n)
	for i := 0; i < n; i++ {
		v, err := strconv.ParseUint(raw[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		out[i] = uint8(v)
	}
	return out, nil
}

func hexErr(s string) error {
	return fmt.Errorf("color: invalid hex %q", s)
}

// Hex returns the lower-case hex form: "#rrggbb" or "#rrggbbaa" when alpha is
// non-opaque.
func (c Color) Hex() string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// HexUpper returns the upper-case hex form.
func (c Color) HexUpper() string {
	if c.A == 255 {
		return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, c.A)
}

// Int returns the color packed into a 24- or 32-bit integer string
// (0xRRGGBB or 0xRRGGBBAA).
func (c Color) Int() string {
	if c.A == 255 {
		return fmt.Sprintf("0x%02X%02X%02X", c.R, c.G, c.B)
	}
	return fmt.Sprintf("0x%02X%02X%02X%02X", c.R, c.G, c.B, c.A)
}
