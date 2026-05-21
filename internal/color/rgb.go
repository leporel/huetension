package color

import "fmt"

// RGB returns CSS rgb()/rgba() notation (legacy comma form, widely supported).
func (c Color) RGB() string {
	if c.A == 255 {
		return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", c.R, c.G, c.B, formatAlpha(c.A))
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
