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

func TestComplementaryRedGreen(t *testing.T) {
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
	// On the RYB artist wheel red's +180° complement is a green (RGB hue
	// ≈138°), not the cyan (≈180°) the technical HSV wheel produces.
	if h := got[1].HueDeg(); h < 136 || h > 140 {
		t.Errorf("complement hue %.1f, want ~138 (green)", h)
	}
}

func TestTriadic(t *testing.T) {
	red := mustParse(t, "red")
	got, err := Generate(Triadic, red, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != red {
		t.Errorf("element 0 should be base, got %v", got[0])
	}
	// RYB triad of red: +120° → RGB hue ~60 (yellow), +240° → ~204 (blue).
	if h := got[1].HueDeg(); h < 58 || h > 62 {
		t.Errorf("got[1] hue %.1f, want ~60", h)
	}
	if h := got[2].HueDeg(); h < 202 || h > 206 {
		t.Errorf("got[2] hue %.1f, want ~204", h)
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
	// RYB split of red: +199° → RGB hue ~159 (the near-complement anchor,
	// placed first), +161° → ~118.
	if h := got[1].HueDeg(); h < 157 || h > 161 {
		t.Errorf("got[1] hue %.1f, want ~159", h)
	}
	if h := got[2].HueDeg(); h < 116 || h > 120 {
		t.Errorf("got[2] hue %.1f, want ~118", h)
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
	// RYB tetradic of red: 0/90/180/270 → RGB hues 0/48/138/234.
	wants := []float64{0, 48, 138, 234}
	for i, w := range wants {
		h := tet[i].HueDeg()
		if w == 0 && h > 2 && h < 358 {
			t.Errorf("idx %d hue %.1f, want ~0", i, h)
		}
		if w != 0 && (h < w-2 || h > w+2) {
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
	// RYB double-complementary of red: 0/38/180/218 → RGB hues 0/22/138/180.
	wants := []float64{0, 22, 138, 180}
	for i, w := range wants {
		h := got[i].HueDeg()
		if w == 0 && h > 2 && h < 358 {
			t.Errorf("idx %d hue %.1f, want ~0", i, h)
		}
		if w != 0 && (h < w-2 || h > w+2) {
			t.Errorf("idx %d hue %.1f, want ~%.0f", i, h, w)
		}
	}
}

func TestCompound(t *testing.T) {
	red := mustParse(t, "red")
	got, _ := Generate(Compound, red, Options{})
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	// RYB compound of red: 0/-30/-150/180 → RGB hues 0/298/171/138 —
	// a tight cluster at red and another near its green complement.
	wants := []float64{0, 298, 171, 138}
	for i, w := range wants {
		h := got[i].HueDeg()
		if w == 0 && h > 2 && h < 358 {
			t.Errorf("idx %d hue %.1f, want ~0", i, h)
		}
		if w != 0 && (h < w-2 || h > w+2) {
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
	// Outer elements ±30° on the RYB wheel → RGB hues ~17 and ~298.
	if h := got[2].HueDeg(); h < 15 || h > 19 {
		t.Errorf("got[2] hue %.1f, want ~17", h)
	}
	if h := got[0].HueDeg(); h < 296 || h > 300 {
		t.Errorf("got[0] hue %.1f, want ~298", h)
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
	// Reference tint/shade strips. Saturation is a reflecting triangle wave
	// stepping 1/count off the gamut edges; value dips one step then ramps
	// away from base — towards white for a dark base, towards black (clamped
	// at 20%) for a bright one. The step is count-dependent: count 5 → 20%,
	// count 10 → 10%.
	cases := []struct {
		name         string
		count        int
		h, s, v      float64
		wantS, wantV []float64
	}{
		{
			name: "dark base ramps light", count: 5,
			h: 25, s: 0.89, v: 0.40,
			wantS: []float64{0.89, 0.69, 0.49, 0.29, 0.09},
			wantV: []float64{0.40, 0.20, 0.80, 1.00, 1.00},
		},
		{
			name: "low-sat bright base, wave reflects off the floor", count: 5,
			h: 23, s: 0.10, v: 0.70,
			wantS: []float64{0.10, 0.30, 0.50, 0.70, 0.90},
			wantV: []float64{0.70, 0.50, 0.30, 0.20, 0.20},
		},
		{
			// count 10 → step 0.10: the saturation strip walks down by 10%
			// per slot rather than the count-5 strip's 20%.
			name: "ten slots subdivide the ramp by 1/10", count: 10,
			h: 120, s: 1.00, v: 1.00,
			wantS: []float64{1.00, 0.90, 0.80, 0.70, 0.60, 0.50, 0.40, 0.30, 0.20, 0.10},
			wantV: []float64{1.00, 0.90, 0.80, 0.70, 0.60, 0.50, 0.40, 0.30, 0.20, 0.20},
		},
	}
	for _, c := range cases {
		base := color.FromHSV(c.h, c.s, c.v)
		// Monochromatic keeps every slot at the base's hue; compare against
		// the base's round-tripped hue, since a low-saturation base loses a
		// few degrees of hue precision through the 8-bit RGB encoding.
		baseHue, _, _ := base.ToHSV()
		got, _ := Generate(Monochromatic, base, Options{Count: c.count})
		if len(got) != c.count {
			t.Fatalf("%s: len = %d, want %d", c.name, len(got), c.count)
		}
		if got[0] != base {
			t.Errorf("%s: slot 0 should be base verbatim, got %v", c.name, got[0])
		}
		for i, col := range got {
			h, s, v := col.ToHSV()
			if d := h - baseHue; d < -4 || d > 4 {
				t.Errorf("%s: slot %d hue %.1f, want ~%.1f (base hue)", c.name, i, h, baseHue)
			}
			if d := s - c.wantS[i]; d < -0.03 || d > 0.03 {
				t.Errorf("%s: slot %d saturation %.3f, want ~%.2f", c.name, i, s, c.wantS[i])
			}
			if d := v - c.wantV[i]; d < -0.03 || d > 0.03 {
				t.Errorf("%s: slot %d value %.3f, want ~%.2f", c.name, i, v, c.wantV[i])
			}
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

	// Complementary count=5 → first two are the red/green anchors; slots 2..4
	// are muted cycle echoes of the anchors.
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
	// On the RYB wheel red's complement is green (RGB hue ~138), not cyan.
	if h := got[1].HueDeg(); h < 136 || h > 140 {
		t.Errorf("slot 1 hue %.1f, want ~138 (green)", h)
	}
	// Extra slots group into cycles of n=2: slots 2–3 are cycle 1 (anchors
	// red, green at 67% of the base S/V), slot 4 opens cycle 2 (red at 33%).
	// So slot 4 is a muted, darker red clearly distinct from the pure-red base.
	if got[4] == red {
		t.Errorf("slot 4 should differ from base after the ramp, got %s", got[4].Hex())
	}
	_, redHSVSat, _ := red.ToHSV()
	_, slot4HSVSat, slot4HSVVal := got[4].ToHSV()
	if slot4HSVSat >= redHSVSat {
		t.Errorf("slot 4 HSV saturation %.2f, want < %.2f", slot4HSVSat, redHSVSat)
	}
	if slot4HSVVal >= 1.0 {
		t.Errorf("slot 4 HSV value %.2f, want < 1 (bright base ramps darker)", slot4HSVVal)
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
		{"compound", Compound, Options{}, 0},
		{"shades-4", Shades, Options{Count: 4}, 0},
		{"mono-3", Monochromatic, Options{Count: 3}, 0},
		{"mono-5", Monochromatic, Options{Count: 5}, 0},
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
