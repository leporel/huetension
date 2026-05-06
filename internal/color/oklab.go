package color

import (
	"fmt"
	"math"
)

// OkLab / OkLCH (Björn Ottosson, https://bottosson.github.io/posts/oklab/)
// are implemented in-package because go-colorful does not provide them.
//
// Conventions:
//   - sRGB inputs/outputs use 0..1 floats internally.
//   - OkLab L is 0..1 (CSS allows 0..1 or 0..100% — we accept both).
//   - OkLab a/b are roughly ±0.4.
//   - OkLCH C is roughly 0..0.4; H is degrees in [0, 360).

// srgbToLinear undoes the sRGB transfer function.
func srgbToLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// linearToSRGB applies the sRGB transfer function.
func linearToSRGB(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1.0/2.4) - 0.055
}

// rgbToOkLab takes 0..1 sRGB and returns OkLab.
func rgbToOkLab(r, g, b float64) (L, a, bb float64) {
	rl := srgbToLinear(r)
	gl := srgbToLinear(g)
	bl := srgbToLinear(b)

	lp := 0.4122214708*rl + 0.5363325363*gl + 0.0514459929*bl
	mp := 0.2119034982*rl + 0.6806995451*gl + 0.1073969566*bl
	sp := 0.0883024619*rl + 0.2817188376*gl + 0.6299787005*bl

	lp = math.Cbrt(lp)
	mp = math.Cbrt(mp)
	sp = math.Cbrt(sp)

	L = 0.2104542553*lp + 0.7936177850*mp - 0.0040720468*sp
	a = 1.9779984951*lp - 2.4285922050*mp + 0.4505937099*sp
	bb = 0.0259040371*lp + 0.7827717662*mp - 0.8086757660*sp
	return
}

// okLabToRGB takes OkLab and returns 0..1 sRGB (clamped).
func okLabToRGB(L, a, b float64) (r, g, bb float64) {
	lp := L + 0.3963377774*a + 0.2158037573*b
	mp := L - 0.1055613458*a - 0.0638541728*b
	sp := L - 0.0894841775*a - 1.2914855480*b

	l := lp * lp * lp
	m := mp * mp * mp
	s := sp * sp * sp

	rl := 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	gl := -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	bl := -0.0041960863*l - 0.7034186147*m + 1.7076147010*s

	return clampUnit(linearToSRGB(rl)), clampUnit(linearToSRGB(gl)), clampUnit(linearToSRGB(bl))
}

func okLabToLCH(L, a, b float64) (Lo, C, H float64) {
	C = math.Hypot(a, b)
	H = math.Mod(math.Atan2(b, a)*180/math.Pi+360, 360)
	return L, C, H
}

func okLCHToLab(L, C, H float64) (Lo, a, b float64) {
	rad := H * math.Pi / 180
	return L, C * math.Cos(rad), C * math.Sin(rad)
}

func parseOkLabFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: oklab needs 3-4 args, got %d in %q", len(parts), full)
	}
	L, err := parsePctOrUnit(parts[0]) // 0..1
	if err != nil {
		return Color{}, fmt.Errorf("color: oklab L: %w", err)
	}
	a, err := parseFloat(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: oklab a: %w", err)
	}
	b, err := parseFloat(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: oklab b: %w", err)
	}
	alpha := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: oklab alpha: %w", err)
		}
		alpha = av
	}
	r, g, bb := okLabToRGB(L, a, b)
	return Color{R: toUint8(r * 255), G: toUint8(g * 255), B: toUint8(bb * 255), A: alpha}, nil
}

func parseOkLCHFn(args, full string) (Color, error) {
	parts := splitArgs(args)
	if len(parts) < 3 || len(parts) > 4 {
		return Color{}, fmt.Errorf("color: oklch needs 3-4 args, got %d in %q", len(parts), full)
	}
	L, err := parsePctOrUnit(parts[0])
	if err != nil {
		return Color{}, fmt.Errorf("color: oklch L: %w", err)
	}
	C, err := parseFloat(parts[1])
	if err != nil {
		return Color{}, fmt.Errorf("color: oklch C: %w", err)
	}
	H, err := parseHue(parts[2])
	if err != nil {
		return Color{}, fmt.Errorf("color: oklch H: %w", err)
	}
	alpha := uint8(255)
	if len(parts) == 4 {
		av, err := parseAlphaToken(parts[3])
		if err != nil {
			return Color{}, fmt.Errorf("color: oklch alpha: %w", err)
		}
		alpha = av
	}
	_, a, b := okLCHToLab(L, C, H)
	r, g, bb := okLabToRGB(L, a, b)
	return Color{R: toUint8(r * 255), G: toUint8(g * 255), B: toUint8(bb * 255), A: alpha}, nil
}

// OkLab returns CSS oklab() notation.
func (c Color) OkLab() string {
	L, a, b := rgbToOkLab(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
	if c.A == 255 {
		return fmt.Sprintf("oklab(%s %s %s)", formatFloat(L, 4), formatFloat(a, 4), formatFloat(b, 4))
	}
	return fmt.Sprintf("oklab(%s %s %s / %s)", formatFloat(L, 4), formatFloat(a, 4), formatFloat(b, 4), formatAlpha(c.A))
}

// OkLCH returns CSS oklch() notation.
func (c Color) OkLCH() string {
	L, a, b := rgbToOkLab(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
	_, C, H := okLabToLCH(L, a, b)
	if c.A == 255 {
		return fmt.Sprintf("oklch(%s %s %s)", formatFloat(L, 4), formatFloat(C, 4), formatFloat(H, 1))
	}
	return fmt.Sprintf("oklch(%s %s %s / %s)", formatFloat(L, 4), formatFloat(C, 4), formatFloat(H, 1), formatAlpha(c.A))
}
