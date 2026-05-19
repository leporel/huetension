package color

import (
	"fmt"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// CIE L*a*b* and LCH are wrapped over go-colorful. go-colorful keeps L on a
// 0..1 scale (and a/b proportionally smaller) — we expose CSS-style values
// (L 0..100, a/b ≈ ±125, C 0..150) by multiplying / dividing by 100 in the
// boundary functions below. Round-tripping through these helpers reproduces
// the source RGB within ±1 unit per channel, which is the contract the rest
// of the package relies on.

const labScale = 100.0

func parseLabFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: lab needs 3-4 args, got %d in %q", len(parts), full)
	}
	L, err := parseLabL(parts[0])
	if err != nil {
		return Color{}, fmt.Errorf("color: lab L: %w", err)
	}
	a, err := parseFloat(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: lab a: %w", err)
	}
	b, err := parseFloat(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: lab b: %w", err)
	}
	alpha := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: lab alpha: %w", err)
		}
		alpha = av
	}
	cf := colorful.Lab(L/labScale, a/labScale, b/labScale)
	return fromColorful(cf, alpha), nil
}

func parseLCHFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: lch needs 3-4 args, got %d in %q", len(parts), full)
	}
	L, err := parseLabL(parts[0])
	if err != nil {
		return Color{}, fmt.Errorf("color: lch L: %w", err)
	}
	C, err := parseFloat(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: lch C: %w", err)
	}
	h, err := parseHue(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: lch h: %w", err)
	}
	alpha := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: lch alpha: %w", err)
		}
		alpha = av
	}
	cf := colorful.Hcl(h, C/labScale, L/labScale)
	return fromColorful(cf, alpha), nil
}

// parseLabL accepts L either as a percentage ("50%") or as a 0..100 number.
// Bare floats ≤ 1 are treated as 0..1 fractions for tolerance.
func parseLabL(s string) (float64, error) {
	v, err := parsePctOrUnit(s)
	if err != nil {
		return 0, err
	}
	// parsePctOrUnit returns a 0..1 value. Scale to 0..100 for CSS Lab.
	return v * 100, nil
}

// Lab returns CSS lab() notation.
func (c Color) Lab() string {
	l, a, b := c.toColorful().Lab()
	if c.A == 255 {
		return fmt.Sprintf("lab(%d %d %d)", roundInt(l*labScale), roundInt(a*labScale), roundInt(b*labScale))
	}
	return fmt.Sprintf("lab(%d %d %d / %s)", roundInt(l*labScale), roundInt(a*labScale), roundInt(b*labScale), formatAlpha(c.A))
}

// LCH returns CSS lch() notation.
func (c Color) LCH() string {
	h, c2, l := c.toColorful().Hcl()
	if c.A == 255 {
		return fmt.Sprintf("lch(%d %d %d)", roundInt(l*labScale), roundInt(c2*labScale), roundInt(h))
	}
	return fmt.Sprintf("lch(%d %d %d / %s)", roundInt(l*labScale), roundInt(c2*labScale), roundInt(h), formatAlpha(c.A))
}
