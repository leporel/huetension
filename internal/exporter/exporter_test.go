package exporter

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// fixturePalette returns a small deterministic palette covering the
// interesting cases: opaque + translucent colors, frequencies that sum to
// 1, and a non-empty Metadata block.
func fixturePalette() *palette.Palette {
	cs := []color.Color{
		{R: 255, G: 0, B: 0, A: 255, Freq: 0.5},
		{R: 0, G: 255, B: 0, A: 255, Freq: 0.3},
		{R: 0, G: 0, B: 255, A: 128, Freq: 0.2},
	}
	p := palette.New(cs)
	p.Name = "fixture"
	p.Metadata = palette.Metadata{
		Source:      "test://fixture",
		Method:      "manual",
		Params:      map[string]any{"palette_size": 3},
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	return p
}

func TestExportRejectsNilOrEmpty(t *testing.T) {
	if _, err := Export(nil, FormatJSON, Options{}); err == nil {
		t.Error("expected error for nil palette")
	}
	if _, err := Export(palette.New(nil), FormatJSON, Options{}); err == nil {
		t.Error("expected error for empty palette")
	}
}

func TestExportRejectsUnknownFormat(t *testing.T) {
	_, err := Export(fixturePalette(), Format("does-not-exist"), Options{})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("got %v, want unknown-format error", err)
	}
}

func TestAllFormatsProduceNonEmptyOutput(t *testing.T) {
	p := fixturePalette()
	for _, f := range AllFormats {
		out, err := Export(p, f, Options{})
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if len(out) == 0 {
			t.Errorf("%s: empty output", f)
		}
	}
}

func TestJSONEnvelopeShape(t *testing.T) {
	out, err := Export(fixturePalette(), FormatJSON, Options{
		Pretty: true,
		Tool:   "extract",
		Params: map[string]any{"method": "manual", "size": 3},
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["schema"] != huetensionSchema {
		t.Errorf("schema = %v, want %s", got["schema"], huetensionSchema)
	}
	if got["tool"] != "extract" {
		t.Errorf("tool = %v, want extract", got["tool"])
	}
	params, _ := got["params"].(map[string]any)
	if params["method"] != "manual" {
		t.Errorf("params.method = %v", params["method"])
	}

	result, ok := got["result"].(map[string]any)
	if !ok {
		t.Fatal("result block missing")
	}
	pal, ok := result["palette"].(map[string]any)
	if !ok {
		t.Fatal("result.palette missing")
	}
	if pal["name"] != "fixture" {
		t.Errorf("palette.name = %v", pal["name"])
	}
	if size, _ := pal["size"].(float64); int(size) != 3 {
		t.Errorf("palette.size = %v, want 3", pal["size"])
	}
	colors, ok := pal["colors"].([]any)
	if !ok || len(colors) != 3 {
		t.Fatalf("palette.colors missing or wrong length: %v", pal["colors"])
	}
	first, _ := colors[0].(map[string]any)
	if first["hex"] != "#ff0000" {
		t.Errorf("first hex = %v, want #ff0000", first["hex"])
	}

	meta, ok := result["metadata"].(map[string]any)
	if !ok {
		t.Fatal("result.metadata missing")
	}
	if meta["method"] != "manual" {
		t.Errorf("metadata.method = %v", meta["method"])
	}
}

func TestJSONOmitsToolAndParamsForLibraryCallers(t *testing.T) {
	out, err := Export(fixturePalette(), FormatJSON, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Parse and check the top-level keys directly — substring match would
	// catch nested `result.metadata.params` (always present from the
	// fixture's algorithmic params) and produce a false positive.
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if _, present := got["tool"]; present {
		t.Errorf("top-level tool key should be omitted when caller leaves it unset:\n%s", out)
	}
	if _, present := got["params"]; present {
		t.Errorf("top-level params key should be omitted when caller leaves it unset:\n%s", out)
	}
}

func TestJSONOmitsRGBAForOpaqueColors(t *testing.T) {
	out, err := Export(fixturePalette(), FormatJSON, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	colors := got["result"].(map[string]any)["palette"].(map[string]any)["colors"].([]any)
	first := colors[0].(map[string]any)
	if _, present := first["rgba"]; present {
		t.Errorf("opaque color should not have rgba field, got %v", first)
	}
	third := colors[2].(map[string]any)
	if _, present := third["rgba"]; !present {
		t.Errorf("translucent color (A=128) should have rgba field, got %v", third)
	}
}

func TestJSONOmitsEmptyMetadata(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})
	out, err := Export(p, FormatJSON, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Metadata is now nested under result.metadata. With no metadata,
	// neither the inner map nor the result-level key should appear.
	if strings.Contains(string(out), `"metadata"`) {
		t.Errorf("expected no metadata key for empty metadata, got %s", out)
	}
}

func TestJSONIncludesAlphaForTranslucent(t *testing.T) {
	out, err := Export(fixturePalette(), FormatJSON, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The third fixture color has A=128 — its rgba field should appear.
	if !strings.Contains(string(out), `"rgba"`) {
		t.Errorf("expected rgba field for translucent color, got %s", out)
	}
}

func TestCSSStructure(t *testing.T) {
	out, err := Export(fixturePalette(), FormatCSS, Options{Prefix: "brand"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Third fixture has A=128 → 8-digit hex (#RRGGBBAA), which CSS accepts.
	for _, want := range []string{":root {", "--brand-1: #ff0000;", "--brand-3: #0000ff80;", "}"} {
		if !strings.Contains(s, want) {
			t.Errorf("CSS missing %q\n%s", want, s)
		}
	}
}

func TestSCSSEmitsListVariable(t *testing.T) {
	out, err := Export(fixturePalette(), FormatSCSS, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"$color-1: #ff0000;", "$color-list: ($color-1, $color-2, $color-3);"} {
		if !strings.Contains(s, want) {
			t.Errorf("SCSS missing %q\n%s", want, s)
		}
	}
}

func TestLESSEmitsListVariable(t *testing.T) {
	out, err := Export(fixturePalette(), FormatLESS, Options{Prefix: "brand"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"@brand-1: #ff0000;", "@brand-list: @brand-1, @brand-2, @brand-3;"} {
		if !strings.Contains(s, want) {
			t.Errorf("LESS missing %q\n%s", want, s)
		}
	}
}

func TestTailwindIsValidJSObject(t *testing.T) {
	out, err := Export(fixturePalette(), FormatTailwind, Options{Prefix: "brand"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`module.exports = {`, `"brand-1": "#ff0000",`, `};`} {
		if !strings.Contains(s, want) {
			t.Errorf("Tailwind missing %q\n%s", want, s)
		}
	}
}

func TestPlainHexPerLine(t *testing.T) {
	out, err := Export(fixturePalette(), FormatPlain, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := "#ff0000\n#00ff00\n#0000ff80\n"
	if string(out) != want {
		t.Errorf("plain = %q, want %q", out, want)
	}
}

func TestGPLHasHeaderAndRows(t *testing.T) {
	out, err := Export(fixturePalette(), FormatGPL, Options{Prefix: "brand"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"GIMP Palette", "Name: fixture", "255   0   0\tbrand-1", "  0   0 255\tbrand-3"} {
		if !strings.Contains(s, want) {
			t.Errorf("GPL missing %q\n%s", want, s)
		}
	}
}

func TestPNGRendersValidImage(t *testing.T) {
	out, err := Export(fixturePalette(), FormatPNG, Options{SwatchWidth: 32, SwatchHeight: 24})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	// 3 colors × 32 px wide swatches.
	if got := img.Bounds().Dx(); got != 96 {
		t.Errorf("width = %d, want 96", got)
	}
	if got := img.Bounds().Dy(); got != 24 {
		t.Errorf("height = %d, want 24", got)
	}
}

func TestJPEGRendersValidImage(t *testing.T) {
	out, err := Export(fixturePalette(), FormatJPEG, Options{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}
	// Default 96×96 swatches × 3 colors.
	if got := img.Bounds().Dx(); got != 288 {
		t.Errorf("width = %d, want 288", got)
	}
	if got := img.Bounds().Dy(); got != 96 {
		t.Errorf("height = %d, want 96", got)
	}
}

func TestFormatFromExtension(t *testing.T) {
	cases := map[string]Format{
		"json":  FormatJSON,
		".css":  FormatCSS,
		"SCSS":  "", // case-sensitive on purpose; CLI lowercases before lookup
		"png":   FormatPNG,
		".png":  FormatPNG,
		"jpg":   FormatJPEG,
		".jpeg": FormatJPEG,
		"ggr":   FormatGGR,
		".svg":  FormatSVG,
	}
	for ext, want := range cases {
		got, ok := FormatFromExtension(ext)
		if want == "" {
			if ok {
				t.Errorf("FormatFromExtension(%q) = %q, want not-found", ext, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("FormatFromExtension(%q) = (%q,%v), want (%q,true)", ext, got, ok, want)
		}
	}
}

func TestFileExtensionForEveryFormat(t *testing.T) {
	for _, f := range AllFormats {
		if FileExtension(f) == "" {
			t.Errorf("%s: empty file extension", f)
		}
	}
	if FileExtension("does-not-exist") != "" {
		t.Error("unknown format should return empty extension")
	}
}

func TestGGRStructure(t *testing.T) {
	out, err := Export(fixturePalette(), FormatGGR, Options{})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5 (header + name + count + 2 segments)\n%s", len(lines), out)
	}
	if lines[0] != "GIMP Gradient" {
		t.Errorf("line 1 = %q, want %q", lines[0], "GIMP Gradient")
	}
	if lines[1] != "Name: fixture" {
		t.Errorf("line 2 = %q, want %q", lines[1], "Name: fixture")
	}
	if lines[2] != "2" { // 3 colors → 2 segments
		t.Errorf("segment count = %q, want 2", lines[2])
	}
	for _, seg := range lines[3:] {
		if n := len(strings.Fields(seg)); n != 13 {
			t.Errorf("segment %q has %d fields, want 13", seg, n)
		}
	}
	// The gradient must span the whole [0,1] range.
	if first := strings.Fields(lines[3]); first[0] != "0.000000" {
		t.Errorf("first segment left = %q, want 0.000000", first[0])
	}
	if last := strings.Fields(lines[4]); last[2] != "1.000000" {
		t.Errorf("last segment right = %q, want 1.000000", last[2])
	}
}

// A one-color palette is still a valid (flat) gradient — one segment.
func TestGGRSingleColor(t *testing.T) {
	p := palette.New([]color.Color{{R: 10, G: 20, B: 30, A: 255}})
	out, err := Export(p, FormatGGR, Options{Name: "solid"})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 4 || lines[2] != "1" {
		t.Fatalf("single-color ggr should have 1 segment, got\n%s", out)
	}
}

func TestSVGStructure(t *testing.T) {
	out, err := Export(fixturePalette(), FormatSVG, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"<svg ", "<linearGradient ", "</linearGradient>", "</svg>",
		`fill="url(#fixture)"`, `stop-color="#ff0000"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("svg missing %q\n%s", want, s)
		}
	}
	if n := strings.Count(s, "<stop "); n != 3 { // 3 colors → 3 stops
		t.Errorf("stop count = %d, want 3", n)
	}
}

func TestSVGIdent(t *testing.T) {
	cases := map[string]string{
		"my gradient": "my-gradient",
		"  spaced  ":  "spaced",
		"3stops":      "g-3stops",
		"!!!":         "gradient",
		"":            "gradient",
		"a/b\\c":      "a-b-c",
	}
	for in, want := range cases {
		if got := svgIdent(in); got != want {
			t.Errorf("svgIdent(%q) = %q, want %q", in, got, want)
		}
	}
}
