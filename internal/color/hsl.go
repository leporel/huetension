package color

import (
	"fmt"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// ToHSL returns (H deg, S 0..1, L 0..1).
func (c Color) ToHSL() (h, s, l float64) {
	return c.toColorful().Hsl()
}

// Lightness returns the L component of HSL in 0..1.
func (c Color) Lightness() float64 {
	_, _, l := c.toColorful().Hsl()
	return l
}

// HueDeg returns the HSL hue in degrees [0, 360).
func (c Color) HueDeg() float64 {
	h, _, _ := c.toColorful().Hsl()
	return h
}

// Saturation returns the HSL saturation in 0..1.
func (c Color) Saturation() float64 {
	_, s, _ := c.toColorful().Hsl()
	return s
}

// HSL returns CSS hsl()/hsla() notation.
func (c Color) HSL() string {
	h, s, l := c.toColorful().Hsl()
	if c.A == 255 {
		return fmt.Sprintf("hsl(%d %d%% %d%%)", roundInt(h), roundInt(s*100), roundInt(l*100))
	}
	return fmt.Sprintf("hsla(%d %d%% %d%% / %s)", roundInt(h), roundInt(s*100), roundInt(l*100), formatAlpha(c.A))
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
