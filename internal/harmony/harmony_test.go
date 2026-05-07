package harmony

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

// closeHex compares two colors by hex (drops sub-channel drift introduced by
// HSL/Lab round-trips).
func closeHex(a, b color.Color) bool { return a.Hex() == b.Hex() }

func TestComplementaryRedCyan(t *testing.T) {
	red := mustParse(t, "red")
	got, err := Generate(Complementary, red, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != red {
		t.Errorf("element 0 should be base, got %v", got[0])
	}
	if !closeHex(got[1], mustParse(t, "cyan")) {
		t.Errorf("element 1 = %s, want cyan", got[1].Hex())
	}
}

func TestTriadicRGB(t *testing.T) {
	red := mustParse(t, "red")
	got, err := Generate(Triadic, red, Options{})
	if err != nil {
		t.Fatal(err)
	}
	wants := []color.Color{red, mustParse(t, "lime"), mustParse(t, "blue")}
	for i, w := range wants {
		if !closeHex(got[i], w) {
			t.Errorf("idx %d = %s, want %s", i, got[i].Hex(), w.Hex())
		}
	}
}

func TestSplitComplementary(t *testing.T) {
	red := mustParse(t, "red")
	got, err := Generate(Split, red, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Hues should be 0, 150, 210 around red.
	if h := got[1].HueDeg(); h < 149 || h > 151 {
		t.Errorf("got[1] hue %.1f, want ~150", h)
	}
	if h := got[2].HueDeg(); h < 209 || h > 211 {
		t.Errorf("got[2] hue %.1f, want ~210", h)
	}
}

func TestTetradicSquare(t *testing.T) {
	red := mustParse(t, "red")
	tet, _ := Generate(Tetradic, red, Options{})
	sq, _ := Generate(Square, red, Options{})
	if len(tet) != 4 || len(sq) != 4 {
		t.Fatalf("expected 4 colors")
	}
	for i := range tet {
		if tet[i] != sq[i] {
			t.Errorf("Tetradic and Square should be identical, differ at %d", i)
		}
	}
	// Hues should be 0/90/180/270.
	wants := []float64{0, 90, 180, 270}
	for i, w := range wants {
		h := tet[i].HueDeg()
		if w == 0 && h > 1 && h < 359 {
			t.Errorf("idx %d hue %.1f, want ~0", i, h)
		}
		if w != 0 && (h < w-1 || h > w+1) {
			t.Errorf("idx %d hue %.1f, want ~%.0f", i, h, w)
		}
	}
}

func TestDoubleComplementary(t *testing.T) {
	red := mustParse(t, "red")
	got, _ := Generate(DoubleComplementary, red, Options{})
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	wants := []float64{0, 60, 180, 240}
	for i, w := range wants {
		h := got[i].HueDeg()
		if w == 0 && h > 1 && h < 359 {
			t.Errorf("idx %d hue %.1f, want ~0", i, h)
		}
		if w != 0 && (h < w-1 || h > w+1) {
			t.Errorf("idx %d hue %.1f, want ~%.0f", i, h, w)
		}
	}
}

func TestAnalogousDefault(t *testing.T) {
	red := mustParse(t, "red")
	got, _ := Generate(Analogous, red, Options{})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[1] != red {
		t.Errorf("middle element should be base, got %v", got[1])
	}
	// Outer elements ±30°.
	if h := got[2].HueDeg(); h < 29 || h > 31 {
		t.Errorf("got[2] hue %.1f, want ~30", h)
	}
	if h := got[0].HueDeg(); h < 329 || h > 331 {
		t.Errorf("got[0] hue %.1f, want ~330", h)
	}
}

func TestAnalogousCustom(t *testing.T) {
	red := mustParse(t, "red")
	got, _ := Generate(Analogous, red, Options{Count: 5, Step: 20})
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	// Hues: -40, -20, 0, 20, 40 → 320, 340, 0, 20, 40
	wantsCenter := got[2]
	if wantsCenter != red {
		t.Errorf("centre should be base")
	}
}

func TestMonochromatic(t *testing.T) {
	red := mustParse(t, "red")
	got, _ := Generate(Monochromatic, red, Options{Count: 5})
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	// All should share the same hue. Note: pure-grey samples have undefined
	// hue, so we sanity-check by checking saturation > 0.
	for i, c := range got {
		if c.Saturation() < 0.5 {
			t.Errorf("idx %d saturation %.2f, expected ≥0.5", i, c.Saturation())
		}
	}
	// Lightness should grow monotonically.
	for i := 1; i < len(got); i++ {
		if got[i].Lightness() <= got[i-1].Lightness() {
			t.Errorf("lightness not monotonic at %d: %.2f -> %.2f",
				i, got[i-1].Lightness(), got[i].Lightness())
		}
	}
}

func TestShades(t *testing.T) {
	red := mustParse(t, "red")
	got, _ := Generate(Shades, red, Options{Count: 4})
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	if got[0] != red {
		t.Errorf("first shade should be base")
	}
	for i := 1; i < len(got); i++ {
		if got[i].Lightness() >= got[i-1].Lightness() {
			t.Errorf("shades should be monotonically darker, but %d≥%d (%.2f, %.2f)",
				i, i-1, got[i].Lightness(), got[i-1].Lightness())
		}
	}
}

func TestUnknownType(t *testing.T) {
	if _, err := Generate(Type("not-a-thing"), mustParse(t, "red"), Options{}); err == nil {
		t.Errorf("expected error for unknown harmony type")
	}
}

func TestHueHarmonyCountExpansion(t *testing.T) {
	red := mustParse(t, "red")

	// Complementary count=5 → first two are red/cyan anchors, slots 2..4 are
	// HSV variations cycling back through the anchors.
	got, err := Generate(Complementary, red, Options{Count: 5})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0] != red {
		t.Errorf("slot 0 should be base bit-exact, got %v", got[0])
	}
	if !closeHex(got[1], mustParse(t, "cyan")) {
		t.Errorf("slot 1 = %s, want cyan", got[1].Hex())
	}
	// Slot 2 cycles back to anchor 0 (red) with ring-1 delta (V +0.20).
	// Red is already at V=1, so V is clamped — slot 2 should still differ from
	// pure red because the table moves it through HSV at all only if the V
	// has headroom; for fully-saturated red the slot collapses onto red.
	// Slot 3 cycles to anchor 1 (cyan) with ring-1 delta — also clamped.
	// Slot 4 cycles to anchor 0 with ring-2 delta (S -0.25) → desaturated red.
	if got[4] == red {
		t.Errorf("slot 4 should differ from base after S desaturation, got %s", got[4].Hex())
	}
	// Slot 4 = anchor 0 (red, HSV S=1) at ring 2 (S -0.25) → HSV S=0.75.
	_, redHSVSat, _ := red.ToHSV()
	_, slot4HSVSat, _ := got[4].ToHSV()
	if slot4HSVSat >= redHSVSat {
		t.Errorf("slot 4 HSV saturation %.2f, want < %.2f", slot4HSVSat, redHSVSat)
	}

	// Triadic count=2 → too small, error.
	if _, err := Generate(Triadic, red, Options{Count: 2}); err == nil {
		t.Errorf("expected error for triadic count=2")
	}

	// Tetradic count=4 → identical to natural (count=0) output.
	natural, _ := Generate(Tetradic, red, Options{})
	expanded, err := Generate(Tetradic, red, Options{Count: 4})
	if err != nil {
		t.Fatalf("Generate tetradic count=4: %v", err)
	}
	if len(expanded) != 4 {
		t.Fatalf("len = %d, want 4", len(expanded))
	}
	for i := range natural {
		if natural[i] != expanded[i] {
			t.Errorf("count=4 should match natural at %d: %s vs %s",
				i, natural[i].Hex(), expanded[i].Hex())
		}
	}
}

func TestHueHarmonyCountDeterministic(t *testing.T) {
	// Same inputs must produce byte-identical outputs (wire-contract guarantee).
	base := mustParse(t, "#3366cc")
	a, _ := Generate(Triadic, base, Options{Count: 7})
	b, _ := Generate(Triadic, base, Options{Count: 7})
	if len(a) != 7 || len(b) != 7 {
		t.Fatalf("unexpected lengths: %d, %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("non-deterministic at %d: %s vs %s", i, a[i].Hex(), b[i].Hex())
		}
	}
	if a[0] != base {
		t.Errorf("slot 0 should be base bit-exact, got %s", a[0].Hex())
	}
}

func TestBaseAtKnownIndex(t *testing.T) {
	// Pick a desaturated base that would drift if regenerated through HSL.
	base := mustParse(t, "#7C8A99")
	cases := []struct {
		name  string
		ty    Type
		opts  Options
		index int
	}{
		{"complementary", Complementary, Options{}, 0},
		{"analogous-3", Analogous, Options{Count: 3}, 1},
		{"analogous-5", Analogous, Options{Count: 5}, 2},
		{"triadic", Triadic, Options{}, 0},
		{"split", Split, Options{}, 0},
		{"tetradic", Tetradic, Options{}, 0},
		{"square", Square, Options{}, 0},
		{"double-comp", DoubleComplementary, Options{}, 0},
		{"shades-4", Shades, Options{Count: 4}, 0},
		{"mono-3", Monochromatic, Options{Count: 3}, 1},
		{"mono-5", Monochromatic, Options{Count: 5}, 2},
	}
	for _, c := range cases {
		got, err := Generate(c.ty, base, c.opts)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got[c.index] != base {
			t.Errorf("%s: got[%d] = %v, want base %v", c.name, c.index, got[c.index], base)
		}
	}
}
