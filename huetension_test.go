package huetension_test

import (
	"strings"
	"testing"

	"github.com/leporel/huetension"
)

// TestFacadeSmokeUseEverydayPath exercises the most likely external-user
// flow: parse a hex, build a palette, generate a harmony, export to CSS.
// Regressions in any of the re-exported symbols would surface here.
func TestFacadeSmokeUseEverydayPath(t *testing.T) {
	base, err := huetension.ParseHex("#3366cc")
	if err != nil {
		t.Fatalf("ParseHex: %v", err)
	}
	if base.Hex() != "#3366cc" {
		t.Errorf("Hex round-trip = %q", base.Hex())
	}

	colors, err := huetension.GenerateHarmony(huetension.HarmonyTriadic, base, huetension.HarmonyOptions{})
	if err != nil {
		t.Fatalf("GenerateHarmony: %v", err)
	}
	if len(colors) != 3 {
		t.Fatalf("triadic returned %d colors, want 3", len(colors))
	}

	pal := huetension.NewPalette(colors)
	out, err := huetension.Export(pal, huetension.FormatCSS, huetension.ExportOptions{Prefix: "brand"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.Contains(string(out), "--brand-1:") {
		t.Errorf("CSS missing --brand-1, got:\n%s", out)
	}
}

func TestAllExtractMethodsExposed(t *testing.T) {
	if len(huetension.AllExtractMethods) < 9 {
		t.Errorf("AllExtractMethods has %d entries, want at least 9", len(huetension.AllExtractMethods))
	}
}

func TestAllExportFormatsHaveExtensions(t *testing.T) {
	for _, f := range huetension.AllExportFormats {
		if huetension.FileExtension(f) == "" {
			t.Errorf("%s: empty file extension", f)
		}
	}
}
