package gradient

import (
	"testing"

	"github.com/leporel/huetension/internal/color"
)

func mustParse(t *testing.T, s string) color.Color {
	t.Helper()
	c, err := color.Parse(s)
	if err != nil {
		t.Fatalf("color.Parse(%q): %v", s, err)
	}
	return c
}

func TestBuildEndpointsExact(t *testing.T) {
	from := mustParse(t, "#10243A")
	to := mustParse(t, "#FACC15")
	for _, sp := range []Space{SpaceRGB, SpaceLab, SpaceOkLab, SpaceOkLCH, SpaceHSL} {
		got, err := Build(from, to, Options{Steps: 5, Space: sp})
		if err != nil {
			t.Fatalf("space %s: %v", sp, err)
		}
		if got[0] != from {
			t.Errorf("space %s: first != from (%v vs %v)", sp, got[0], from)
		}
		if got[len(got)-1] != to {
			t.Errorf("space %s: last != to (%v vs %v)", sp, got[len(got)-1], to)
		}
	}
}

func TestBuildLengthMatchesSteps(t *testing.T) {
	from := mustParse(t, "black")
	to := mustParse(t, "white")
	for _, n := range []int{2, 3, 5, 16} {
		got, err := Build(from, to, Options{Steps: n})
		if err != nil {
			t.Fatalf("Steps=%d: %v", n, err)
		}
		if len(got) != n {
			t.Errorf("Steps=%d: len=%d", n, len(got))
		}
	}
}

func TestBuildErrorsOnFewSteps(t *testing.T) {
	if _, err := Build(mustParse(t, "red"), mustParse(t, "blue"), Options{Steps: 1}); err == nil {
		t.Errorf("Steps=1 should error")
	}
	if _, err := Build(mustParse(t, "red"), mustParse(t, "blue"), Options{Steps: 0}); err == nil {
		t.Errorf("Steps=0 should error")
	}
}

func TestBuildRGBLinearMidpoint(t *testing.T) {
	from := color.New(0, 0, 0)
	to := color.New(200, 100, 50)
	got, err := Build(from, to, Options{Steps: 3, Space: SpaceRGB, Easing: EasingLinear})
	if err != nil {
		t.Fatal(err)
	}
	mid := got[1]
	// Allow ±1 channel due to rounding.
	if !near(mid.R, 100, 1) || !near(mid.G, 50, 1) || !near(mid.B, 25, 1) {
		t.Errorf("mid = %v, want ~(100, 50, 25)", mid)
	}
}

func TestBuildOkLabMonotonicLightness(t *testing.T) {
	from := mustParse(t, "black")
	to := mustParse(t, "white")
	got, err := Build(from, to, Options{Steps: 7, Space: SpaceOkLab})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(got); i++ {
		if got[i].OkL() <= got[i-1].OkL() {
			t.Errorf("OkL not monotonic at %d: %.3f -> %.3f", i, got[i-1].OkL(), got[i].OkL())
		}
	}
}

func TestEasingShiftsMidpoint(t *testing.T) {
	from := color.New(0, 0, 0)
	to := color.New(255, 255, 255)
	linear, _ := Build(from, to, Options{Steps: 5, Space: SpaceRGB, Easing: EasingLinear})
	easeIn, _ := Build(from, to, Options{Steps: 5, Space: SpaceRGB, Easing: EasingEaseIn})
	easeOut, _ := Build(from, to, Options{Steps: 5, Space: SpaceRGB, Easing: EasingEaseOut})

	if linear[2].R == easeIn[2].R || linear[2].R == easeOut[2].R {
		t.Errorf("eased midpoint shouldn't equal linear midpoint (linear=%d, in=%d, out=%d)",
			linear[2].R, easeIn[2].R, easeOut[2].R)
	}
	if easeIn[2].R >= easeOut[2].R {
		t.Errorf("ease-in midpoint should be darker than ease-out (in=%d, out=%d)",
			easeIn[2].R, easeOut[2].R)
	}
}

func TestMultiStop(t *testing.T) {
	stops := []color.Color{
		mustParse(t, "red"),
		mustParse(t, "yellow"),
		mustParse(t, "blue"),
	}
	got, err := MultiStop(stops, Options{Steps: 5, Space: SpaceRGB})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("len=%d, want 5", len(got))
	}
	if got[0] != stops[0] {
		t.Errorf("first = %v, want %v", got[0], stops[0])
	}
	if got[4] != stops[2] {
		t.Errorf("last = %v, want %v", got[4], stops[2])
	}
	// Middle (i=2, t=0.5) should be exactly the middle stop in RGB linear blend.
	if got[2] != stops[1] {
		t.Errorf("middle = %v, want %v (yellow)", got[2], stops[1])
	}
}

func TestMultiStopErrors(t *testing.T) {
	red := mustParse(t, "red")
	if _, err := MultiStop([]color.Color{red}, Options{Steps: 5}); err == nil {
		t.Errorf("single stop should error")
	}
	if _, err := MultiStop([]color.Color{red, red}, Options{Steps: 1}); err == nil {
		t.Errorf("steps < stops should error")
	}
}

func TestUnknownSpace(t *testing.T) {
	red := mustParse(t, "red")
	if _, err := Build(red, red, Options{Steps: 3, Space: Space("not-a-space")}); err == nil {
		t.Errorf("unknown space should error")
	}
}

// lerpHue is package-private but worth a focused test for the wraparound.
func TestLerpHueWraps(t *testing.T) {
	cases := []struct {
		h1, h2, t float64
		want      float64
	}{
		{0, 180, 0.5, 90},   // Either way is 180°; convention picks +90.
		{350, 10, 0.5, 0},   // Wrap forward through 360.
		{10, 350, 0.5, 0},   // Wrap backward through 0.
		{0, 90, 0.5, 45},    // Plain forward.
		{0, 270, 0.5, 315},  // Shortest path is backward (-90 ≡ +270).
	}
	for _, c := range cases {
		got := lerpHue(c.h1, c.h2, c.t)
		if !nearFloat(got, c.want, 1e-6) {
			t.Errorf("lerpHue(%g, %g, %g) = %g, want %g", c.h1, c.h2, c.t, got, c.want)
		}
	}
}

func near(got, want, tol uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= int(tol)
}

func nearFloat(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}
