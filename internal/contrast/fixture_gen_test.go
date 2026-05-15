package contrast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leporel/huetension/internal/color"
)

// TestGenerateParityFixture writes the JSON fixture consumed by the
// TS-side parity check (web/scripts/check-contrast-parity.ts). It is
// gated by HUETENSION_GEN_PARITY=1 so `go test ./... -short` does not
// regenerate it on every run; the committed fixture is canonical.
//
// To regenerate after a Go-side contrast change:
//
//	HUETENSION_GEN_PARITY=1 go test ./internal/contrast -run TestGenerateParityFixture
//
// Mirrors the harmony parity pattern in
// internal/harmony/fixture_gen_test.go.
func TestGenerateParityFixture(t *testing.T) {
	if os.Getenv("HUETENSION_GEN_PARITY") != "1" {
		t.Skip("set HUETENSION_GEN_PARITY=1 to regenerate the parity fixture")
	}

	type contrastCase struct {
		FG     string       `json:"fg"`
		BG     string       `json:"bg"`
		WCAG21 WCAG21Result `json:"wcag21"`
		APCA   APCAResult   `json:"apca"`
	}

	// Pairs span both polarities, near-equal-luminance pairs (where the
	// APCA delta-Y clamp and the WCAG rounding bite), the workspace seed
	// colors, and pure black/white extremes.
	pairs := [][2]string{
		{"#1A1A1A", "#FFFFFF"},
		{"#FFFFFF", "#1A1A1A"},
		{"#000000", "#FFFFFF"},
		{"#FFFFFF", "#000000"},
		{"#6D5AFE", "#FFFFFF"},
		{"#6D5AFE", "#0F172A"},
		{"#FFA94D", "#0F172A"},
		{"#FFD43B", "#FFFFFF"},
		{"#51CF66", "#0F172A"},
		{"#22D3EE", "#1A1A1A"},
		{"#777777", "#888888"},
		{"#333333", "#3A3A3A"},
		{"#E5E5E5", "#FAFAFA"},
		{"#0F172A", "#1E293B"},
		{"#A855F7", "#FDE68A"},
		{"#38BDF8", "#0F172A"},
	}

	out := make([]contrastCase, 0, len(pairs))
	for _, p := range pairs {
		fg, err := color.Parse(p[0])
		if err != nil {
			t.Fatalf("parse fg %q: %v", p[0], err)
		}
		bg, err := color.Parse(p[1])
		if err != nil {
			t.Fatalf("parse bg %q: %v", p[1], err)
		}
		out = append(out, contrastCase{
			FG:     p[0],
			BG:     p[1],
			WCAG21: WCAG21(fg, bg),
			APCA:   APCA(fg, bg),
		})
	}

	path := filepath.Join("..", "..", "web", "src", "composables", "__fixtures__", "contrast.json")
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
