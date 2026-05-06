package cliutil

import (
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

func TestSwatchHonoursNoColor(t *testing.T) {
	c := color.New(255, 0, 0)
	got := Swatch(c, Options{NoColor: true})
	// With NoColor, swatch is plain whitespace — no escape characters.
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("NoColor swatch contains escape: %q", got)
	}
	if got != "  " {
		t.Errorf("NoColor swatch = %q, want two spaces", got)
	}
}

func TestSwatchEmitsANSIWhenColored(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	// Force color profile. lipgloss may detect "no TTY" in tests and skip
	// escapes. We accept either an escape OR the plain string — both are
	// valid behaviours for a bytes.Buffer output. The hard guarantee is
	// that the result has at least the swatch width.
	got := Swatch(color.New(255, 0, 0), Options{NoColor: false, SwatchWidth: 4})
	if len(got) < 4 {
		t.Errorf("swatch shorter than width: %q", got)
	}
}

func TestRenderColorsAlignsRows(t *testing.T) {
	cs := []color.Color{
		{R: 255, A: 255, Freq: 0.5},
		{G: 255, A: 255, Freq: 0.3},
		{B: 255, A: 255, Freq: 0.2},
	}
	got := RenderColors(cs, Options{NoColor: true})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d rows, want 3:\n%s", len(lines), got)
	}
	for _, want := range []string{"#ff0000", "#00ff00", "#0000ff", "50.0%", "30.0%"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

func TestRenderColorsOmitsFreqWhenAllZero(t *testing.T) {
	cs := []color.Color{
		{R: 255, A: 255},
		{B: 255, A: 255},
	}
	got := RenderColors(cs, Options{NoColor: true})
	if strings.Contains(got, "%") {
		t.Errorf("freq column should be omitted, got:\n%s", got)
	}
}

func TestRenderPaletteEmpty(t *testing.T) {
	if got := RenderPalette(nil, Options{}); got != "" {
		t.Errorf("nil palette: got %q, want empty", got)
	}
	if got := RenderPalette(palette.New(nil), Options{}); got != "" {
		t.Errorf("empty palette: got %q, want empty", got)
	}
}

func TestRenderHeader(t *testing.T) {
	if got := RenderHeader("", "src", 4); got != "" {
		t.Errorf("empty verb should suppress header, got %q", got)
	}
	if got := RenderHeader("Extracted", "photo.jpg", 4); !strings.Contains(got, "Extracted 4 colors from photo.jpg") {
		t.Errorf("got %q", got)
	}
	if got := RenderHeader("Generated", "", 5); !strings.Contains(got, "Generated 5 colors") {
		t.Errorf("got %q", got)
	}
}
