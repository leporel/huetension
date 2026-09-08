package exporter

import (
	"encoding/json"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// huetensionSchema is the wire-contract identifier emitted at the top of
// every JSON envelope. Bumping it is a coordination event with the MCP
// (Phase 2) and Web (Phase 3) slices — every JSON consumer keys off this
// string.
const huetensionSchema = "huetension/v1"

// jsonEnvelope is the on-the-wire shape of a tool result. It mirrors the
// shape MCP will use for `structured_content` so the CLI / library / MCP
// layers all emit one canonical JSON document.
//
// Library callers that don't have a "tool" identity (e.g. a Go program
// calling exporter.Export directly) leave Tool / Params unset — the
// fields are omitempty, so the envelope still validates against the
// schema with just `schema` + `result`.
type jsonEnvelope struct {
	Schema string         `json:"schema"`
	Tool   string         `json:"tool,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Result ResultJSON     `json:"result"`
}

// ResultJSON is the `result` block of a palette-bearing envelope: the
// palette plus its provenance metadata. The split keeps the per-color
// array (palette.colors) separate from the run-level metadata (image
// source, timing, params recorded by the algorithm).
//
// Exported so the MCP tools embed the very same type in their typed
// outputs — one struct, one wire shape, no per-frontend copy to drift.
// The jsonschema tags feed the MCP SDK's generated output schemas.
type ResultJSON struct {
	Palette  PaletteJSON       `json:"palette" jsonschema:"the palette itself"`
	Metadata *palette.Metadata `json:"metadata,omitempty" jsonschema:"provenance metadata recorded by the producer"`
}

// PaletteJSON is the shape of the `result.palette` block. `size` is
// redundant with `len(colors)` but it's cheap and saves consumers a step.
type PaletteJSON struct {
	Size   int         `json:"size" jsonschema:"number of colors in the palette"`
	Name   string      `json:"name,omitempty" jsonschema:"optional palette name"`
	Colors []ColorJSON `json:"colors" jsonschema:"colors in the palette in canonical order"`
}

// ColorJSON is the on-the-wire shape of a single palette entry. We expand
// to multiple representations so every consumer (web, design tools, code
// generators) gets the form it wants without re-parsing.
//
// Exposed (capitalised) so callers that emit non-palette JSON envelopes
// (e.g. the blindness.simulate variant arrays in MCP and the Web API)
// can share one canonical encoding instead of duplicating per consumer.
//
// RGBA / Source are pointers because Go's encoding/json `omitempty` does
// not recognise zero-valued fixed-size arrays / nested struct values —
// without the indirection every opaque color would carry a misleading
// `"rgba":[0,0,0,0]` and `"source":{...zero...}` field.
type ColorJSON struct {
	Hex    string        `json:"hex" jsonschema:"canonical hex (#rrggbb)"`
	RGB    [3]uint8      `json:"rgb" jsonschema:"sRGB 8-bit channels"`
	RGBA   *[4]uint8     `json:"rgba,omitempty" jsonschema:"sRGB 8-bit channels plus alpha; present only when the color is translucent"`
	HSL    [3]int        `json:"hsl" jsonschema:"HSL [hue °, saturation %, lightness %]"`
	HSV    [3]int        `json:"hsv" jsonschema:"HSV [hue °, saturation %, value %]"`
	OkLCH  [3]int        `json:"oklch" jsonschema:"OkLCH [lightness %, chroma %, hue °]"`
	Freq   float64       `json:"freq,omitempty" jsonschema:"frequency 0..1, populated for extracted palettes"`
	Source *color.Source `json:"source,omitempty" jsonschema:"normalised (0..1) representative-pixel coordinate; populated only for image.extract output"`
}

// renderJSON encodes p as a huetension/v1 envelope. opts.Pretty toggles
// indented output; opts.Tool / opts.Params populate the tool-context
// fields when the caller is the CLI or an MCP tool. Library callers can
// leave them empty to get a clean palette-only envelope.
func renderJSON(p *palette.Palette, opts Options) ([]byte, error) {
	result := EncodeResult(p)
	result.Palette.Name = paletteName(p, opts)

	env := jsonEnvelope{
		Schema: huetensionSchema,
		Tool:   opts.Tool,
		Params: opts.Params,
		Result: result,
	}

	if opts.Pretty {
		return json.MarshalIndent(env, "", "  ")
	}
	return json.Marshal(env)
}

func paletteName(p *palette.Palette, opts Options) string {
	if p.Name != "" {
		return p.Name
	}
	if opts.Name != "" && opts.Name != defaultName {
		return opts.Name
	}
	return ""
}

// EncodeResult builds the canonical `result` block for a palette: colors
// through EncodeColors, metadata only when the producer recorded any. A
// nil palette yields an empty (never null) colors array so consumers can
// index without a nil check.
func EncodeResult(p *palette.Palette) ResultJSON {
	if p == nil {
		return ResultJSON{Palette: PaletteJSON{Colors: []ColorJSON{}}}
	}
	return ResultJSON{
		Palette: PaletteJSON{
			Size:   p.Len(),
			Name:   p.Name,
			Colors: EncodeColors(p.Colors),
		},
		Metadata: jsonMetadata(p),
	}
}

func jsonMetadata(p *palette.Palette) *palette.Metadata {
	m := p.Metadata
	if m.Source == "" && m.Method == "" && m.Params == nil &&
		m.ImageInfo == nil && m.Stats == nil && m.GeneratedAt.IsZero() {
		return nil
	}
	return &m
}

// EncodeColor produces the canonical JSON shape for a single color. It
// shares logic with the palette envelope renderer so the
// blindness.simulate variants (and any future non-palette JSON consumer)
// emit colors identical to result.palette.colors.
func EncodeColor(c color.Color) ColorJSON {
	return encodeColor(c)
}

// EncodeColors batches EncodeColor over a slice.
func EncodeColors(in []color.Color) []ColorJSON {
	out := make([]ColorJSON, len(in))
	for i, c := range in {
		out[i] = encodeColor(c)
	}
	return out
}

func encodeColor(c color.Color) ColorJSON {
	h, s, l := c.ToHSL()
	hh, ss, v := c.ToHSV()
	okL, okC, okH := c.ToOkLCH()
	out := ColorJSON{
		Hex:    c.Hex(),
		RGB:    [3]uint8{c.R, c.G, c.B},
		HSL:    [3]int{roundDeg(h), pct(s), pct(l)},
		HSV:    [3]int{roundDeg(hh), pct(ss), pct(v)},
		OkLCH:  [3]int{pct(okL), pct(okC), roundDeg(okH)},
		Freq:   c.Freq,
		Source: c.Source,
	}
	if c.A != 255 {
		out.RGBA = &[4]uint8{c.R, c.G, c.B, c.A}
	}
	return out
}

// roundDeg snaps a hue value (0..360) to its nearest integer. Sub-degree
// precision is below the perceptual threshold and would only add noise to
// the wire format.
func roundDeg(v float64) int {
	if v < 0 {
		v = 0
	}
	if v > 360 {
		v = 360
	}
	return int(v + 0.5)
}

// pct converts a 0..1 fraction (saturation, lightness, chroma) to a 0..100
// integer percentage. Same rationale as roundDeg — a single-percent
// resolution matches what designers actually read.
func pct(v float64) int {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return int(v*100 + 0.5)
}
