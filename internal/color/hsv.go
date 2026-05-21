package color

import (
	"fmt"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// ToHSV returns (H deg, S 0..1, V 0..1).
func (c Color) ToHSV() (h, s, v float64) {
	return c.toColorful().Hsv()
}

// HSV returns hsv() notation. Not part of the CSS Color spec but widely used
// in design tools.
func (c Color) HSV() string {
	h, s, v := c.toColorful().Hsv()
	if c.A == 255 {
		return fmt.Sprintf("hsv(%d %d%% %d%%)", roundInt(h), roundInt(s*100), roundInt(v*100))
	}
	return fmt.Sprintf("hsva(%d %d%% %d%% / %s)", roundInt(h), roundInt(s*100), roundInt(v*100), formatAlpha(c.A))
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
