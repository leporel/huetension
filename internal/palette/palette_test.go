package palette

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

func TestNewClonesInput(t *testing.T) {
	in := []color.Color{mustParse(t, "red"), mustParse(t, "blue")}
	p := New(in)
	in[0] = mustParse(t, "lime")
	if p.Colors[0] == in[0] {
		t.Fatalf("New should copy input slice")
	}
}

func TestSortLuminance(t *testing.T) {
	p := New([]color.Color{
		mustParse(t, "white"),
		mustParse(t, "black"),
		mustParse(t, "#888888"),
	})
	if err := p.Sort(SortByLuminance, false); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(p.Colors); i++ {
		if p.Colors[i-1].Luminance() > p.Colors[i].Luminance() {
			t.Errorf("not sorted ascending: %v", p.Colors)
			break
		}
	}
}

func TestSortReverse(t *testing.T) {
	p := New([]color.Color{
		mustParse(t, "black"),
		mustParse(t, "white"),
	})
	if err := p.Sort(SortByLuminance, true); err != nil {
		t.Fatal(err)
	}
	if p.Colors[0].Luminance() < p.Colors[1].Luminance() {
		t.Errorf("reverse not honoured: %v", p.Colors)
	}
}

func TestSortFrequency(t *testing.T) {
	a := mustParse(t, "red")
	a.Freq = 0.1
	b := mustParse(t, "blue")
	b.Freq = 0.7
	c := mustParse(t, "green")
	c.Freq = 0.2

	p := New([]color.Color{a, b, c})
	if err := p.Sort(SortByFrequency, true); err != nil {
		t.Fatal(err)
	}
	if p.Colors[0].Freq != 0.7 {
		t.Errorf("expected highest freq first, got %v", p.Colors[0])
	}
}

func TestSortUnknownKey(t *testing.T) {
	p := New([]color.Color{mustParse(t, "red")})
	if err := p.Sort(SortBy("not-a-real-key"), false); err == nil {
		t.Errorf("expected error on unknown sort key")
	}
}

func TestSortStability(t *testing.T) {
	// Two grays of identical luminance should retain input order.
	a := mustParse(t, "#808080")
	a.Freq = 0.1
	b := mustParse(t, "#808080")
	b.Freq = 0.9

	p := New([]color.Color{a, b})
	if err := p.Sort(SortByLuminance, false); err != nil {
		t.Fatal(err)
	}
	if p.Colors[0].Freq != 0.1 || p.Colors[1].Freq != 0.9 {
		t.Errorf("stable sort violated: %v", p.Colors)
	}
}

func TestRandomDeterministicWithSeed(t *testing.T) {
	a := Random(RandomOptions{Count: 6, Seed: 42})
	b := Random(RandomOptions{Count: 6, Seed: 42})
	if len(a.Colors) != len(b.Colors) {
		t.Fatalf("len mismatch")
	}
	for i := range a.Colors {
		if a.Colors[i] != b.Colors[i] {
			t.Errorf("idx %d: %v vs %v", i, a.Colors[i], b.Colors[i])
		}
	}
}

func TestRandomCountRespected(t *testing.T) {
	for _, n := range []int{1, 5, 12, 50} {
		got := Random(RandomOptions{Count: n, Seed: 1}).Len()
		want := min(n, maxRandomCount)
		if got != want {
			t.Errorf("Count=%d → got %d, want %d", n, got, want)
		}
	}
}

func TestRandomDefaultCount(t *testing.T) {
	if got := Random(RandomOptions{Seed: 1}).Len(); got != defaultRandomCount {
		t.Errorf("default count = %d, want %d", got, defaultRandomCount)
	}
}

func TestRandomColorsInRange(t *testing.T) {
	p := Random(RandomOptions{Count: 8, Seed: 7})
	for _, c := range p.Colors {
		s := c.Saturation()
		if s < 0.40 || s > 0.85 {
			t.Errorf("saturation %.2f out of expected band", s)
		}
		l := c.Lightness()
		if l < 0.35 || l > 0.75 {
			t.Errorf("lightness %.2f out of expected band", l)
		}
	}
}

func TestSortColorsHelper(t *testing.T) {
	in := []color.Color{
		mustParse(t, "white"),
		mustParse(t, "black"),
	}
	out, err := SortColors(in, SortByLuminance, false)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Luminance() > out[1].Luminance() {
		t.Errorf("SortColors did not sort: %v", out)
	}
	// Input slice must not be mutated.
	if in[0].Luminance() < in[1].Luminance() {
		t.Errorf("SortColors mutated input slice")
	}
}

func TestCloneIsDeep(t *testing.T) {
	p := New([]color.Color{mustParse(t, "red"), mustParse(t, "blue")})
	p.Metadata.Params = map[string]any{"k": 1}
	cp := p.Clone()
	cp.Colors[0] = mustParse(t, "green")
	cp.Metadata.Params["k"] = 2

	if p.Colors[0] == cp.Colors[0] {
		t.Errorf("Clone should not share Colors slice")
	}
	if p.Metadata.Params["k"] != 1 {
		t.Errorf("Clone should not share Params map")
	}
}

func TestNilReceiver(t *testing.T) {
	var p *Palette
	if p.Len() != 0 {
		t.Errorf("nil Palette Len = %d, want 0", p.Len())
	}
	if err := p.Sort(SortByLuminance, false); err != nil {
		t.Errorf("Sort on nil should be a no-op, got %v", err)
	}
	if p.Clone() != nil {
		t.Errorf("Clone on nil should return nil")
	}
}
