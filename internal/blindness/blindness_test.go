package blindness

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

// near checks per-channel proximity for sanity assertions on lossy simulations.
func near(got, want color.Color, tol uint8) bool {
	d := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return d(got.R, want.R) <= int(tol) &&
		d(got.G, want.G) <= int(tol) &&
		d(got.B, want.B) <= int(tol)
}

func TestSimulateAchromaIsGray(t *testing.T) {
	for _, c := range []color.Color{
		mustParse(t, "red"),
		mustParse(t, "lime"),
		mustParse(t, "blue"),
		mustParse(t, "#3366ff"),
	} {
		got, err := Simulate(c, Achroma)
		if err != nil {
			t.Fatal(err)
		}
		if got.R != got.G || got.G != got.B {
			t.Errorf("Achroma(%s) = %s, expected R=G=B", c.Hex(), got.Hex())
		}
	}
}

func TestSimulateAchromaLuma(t *testing.T) {
	// Rec. 601 luma weights → red(255,0,0) → 0.299*255 ≈ 76.
	red := mustParse(t, "red")
	got, _ := Simulate(red, Achroma)
	if got.R < 75 || got.R > 77 {
		t.Errorf("Achroma(red) gray value = %d, want ~76", got.R)
	}
}

func TestSimulateProtanRed(t *testing.T) {
	// Red(255,0,0) under protanopia → first column scaled by 255.
	// Expect ≈ (144, 142, 0).
	got, _ := Simulate(mustParse(t, "red"), Protan)
	want := color.New(144, 142, 0)
	if !near(got, want, 1) {
		t.Errorf("Protan(red) = %s, want %s", got.Hex(), want.Hex())
	}
}

func TestSimulateDeutanGreen(t *testing.T) {
	// Lime(0, 255, 0) → second column scaled by 255 → (0.375*255, 0.300*255, 0.300*255).
	got, _ := Simulate(mustParse(t, "lime"), Deutan)
	want := color.New(96, 77, 77)
	if !near(got, want, 1) {
		t.Errorf("Deutan(lime) = %s, want %s", got.Hex(), want.Hex())
	}
}

func TestSimulateTritanBlue(t *testing.T) {
	// Blue(0, 0, 255) → third column → (0, 0.567*255, 0.525*255) ≈ (0, 145, 134).
	got, _ := Simulate(mustParse(t, "blue"), Tritan)
	want := color.New(0, 145, 134)
	if !near(got, want, 1) {
		t.Errorf("Tritan(blue) = %s, want %s", got.Hex(), want.Hex())
	}
}

func TestSimulatePreservesGrayscale(t *testing.T) {
	// Pure gray stays pure gray (R=G=B) under every kind, since rows sum to 1.
	gray := color.New(128, 128, 128)
	for _, k := range AllKinds {
		got, err := Simulate(gray, k)
		if err != nil {
			t.Fatal(err)
		}
		if got.R != got.G || got.G != got.B {
			t.Errorf("%s(gray) = %s should remain gray", k, got.Hex())
		}
		if got.R < 127 || got.R > 129 {
			t.Errorf("%s(gray) magnitude drifted to %d", k, got.R)
		}
	}
}

func TestSimulatePreservesAlpha(t *testing.T) {
	c := color.NewWithAlpha(255, 0, 0, 128)
	got, _ := Simulate(c, Protan)
	if got.A != 128 {
		t.Errorf("alpha lost: got A=%d, want 128", got.A)
	}
}

func TestSimulateUnknownKind(t *testing.T) {
	if _, err := Simulate(mustParse(t, "red"), Kind("notakind")); err == nil {
		t.Errorf("unknown kind should error")
	}
}

func TestSimulatePalette(t *testing.T) {
	in := []color.Color{
		mustParse(t, "red"),
		mustParse(t, "lime"),
		mustParse(t, "blue"),
	}
	out, err := SimulatePalette(in, Achroma)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) {
		t.Fatalf("len mismatch: got %d, want %d", len(out), len(in))
	}
	// Input must not be mutated.
	if in[0].R != 255 {
		t.Errorf("SimulatePalette mutated input")
	}
	for _, c := range out {
		if c.R != c.G || c.G != c.B {
			t.Errorf("Achroma palette entry not gray: %s", c.Hex())
		}
	}
}

func TestSimulateAll(t *testing.T) {
	in := []color.Color{mustParse(t, "red"), mustParse(t, "blue")}
	all, err := SimulateAll(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(AllKinds) {
		t.Fatalf("got %d kinds, want %d", len(all), len(AllKinds))
	}
	for _, k := range AllKinds {
		if len(all[k]) != len(in) {
			t.Errorf("%s: got %d colors, want %d", k, len(all[k]), len(in))
		}
	}
}
