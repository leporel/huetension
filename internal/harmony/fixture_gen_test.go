package harmony

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leporel/huetension/internal/color"
)

// TestGenerateParityFixture writes the JSON fixture consumed by the
// TS-side parity check (web/scripts/check-parity.ts). It is gated by
// HUETENSION_GEN_PARITY=1 so `go test ./... -short` does not regenerate
// it on every run; the committed fixture is the canonical source.
//
// To regenerate after a Go-side harmony change:
//
//	HUETENSION_GEN_PARITY=1 go test ./internal/harmony -run TestGenerateParityFixture
func TestGenerateParityFixture(t *testing.T) {
	if os.Getenv("HUETENSION_GEN_PARITY") != "1" {
		t.Skip("set HUETENSION_GEN_PARITY=1 to regenerate the parity fixture")
	}

	type harmonyCase struct {
		Type   string   `json:"type"`
		Base   string   `json:"base"`
		Count  int      `json:"count"`
		Step   float64  `json:"step,omitempty"`
		Result []string `json:"result"`
	}

	// Cases cover every harmony type, every natural anchor count, plus
	// one count-beyond-anchors case per hue-rotation type so the
	// HSV ring-delta expansion path is also pinned.
	bases := []string{"#6D5AFE", "#FFA94D", "#0F172A", "#FFFFFF", "#000000"}
	hueRot := []Type{Complementary, Triadic, Split, Tetradic, DoubleComplementary}

	out := make([]harmonyCase, 0, 64)

	for _, b := range bases {
		base, err := color.Parse(b)
		if err != nil {
			t.Fatalf("parse base %q: %v", b, err)
		}
		// Natural-count hue rotations.
		for _, ht := range hueRot {
			res, err := Generate(ht, base, Options{})
			if err != nil {
				t.Fatalf("%s/%s: %v", ht, b, err)
			}
			out = append(out, harmonyCase{
				Type:   string(ht),
				Base:   b,
				Count:  len(res),
				Result: hexes(res),
			})
		}
		// Expanded counts (anchors + 2 ring slots) to exercise ringDeltas.
		for _, exp := range []struct {
			t Type
			n int
		}{
			{Complementary, 4},
			{Triadic, 6},
			{Tetradic, 6},
		} {
			res, err := Generate(exp.t, base, Options{Count: exp.n})
			if err != nil {
				t.Fatalf("%s/%s/%d: %v", exp.t, b, exp.n, err)
			}
			out = append(out, harmonyCase{
				Type:   string(exp.t),
				Base:   b,
				Count:  exp.n,
				Result: hexes(res),
			})
		}
		// Analogous (default count + step).
		res, err := Generate(Analogous, base, Options{})
		if err != nil {
			t.Fatalf("analogous/%s: %v", b, err)
		}
		out = append(out, harmonyCase{
			Type:   string(Analogous),
			Base:   b,
			Count:  len(res),
			Step:   30,
			Result: hexes(res),
		})
		// Monochromatic + Shades at default count (5).
		for _, ht := range []Type{Monochromatic, Shades} {
			res, err := Generate(ht, base, Options{})
			if err != nil {
				t.Fatalf("%s/%s: %v", ht, b, err)
			}
			out = append(out, harmonyCase{
				Type:   string(ht),
				Base:   b,
				Count:  len(res),
				Result: hexes(res),
			})
		}
	}

	path := filepath.Join("..", "..", "web", "src", "composables", "__fixtures__", "harmony.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(buf, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("wrote %d cases to %s", len(out), path)
}

func hexes(cs []color.Color) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Hex()
	}
	return out
}
