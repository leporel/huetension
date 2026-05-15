package tools

import (
	"fmt"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// PaletteResult is the wire shape used by every MCP tool that returns a
// palette of colors. Field set mirrors internal/exporter's private
// jsonPalette type so MCP and CLI emit identical JSON for palette-bearing
// responses; if exporter's encoding evolves, this should follow.
//
// TODO: lift exporter's encoding helpers up so MCP and CLI share one
// implementation. Tracked in slice C/D follow-up.
type PaletteResult struct {
	Size     int                `json:"size" jsonschema:"number of colors in the palette"`
	Name     string             `json:"name,omitempty" jsonschema:"optional palette name"`
	Colors   []ColorEntry       `json:"colors" jsonschema:"colors in the palette in canonical order"`
	Metadata *palette.Metadata  `json:"metadata,omitempty" jsonschema:"provenance metadata recorded by the producer"`
}

// ColorEntry is one row of a palette in JSON. Matches exporter.ColorJSON:
// hex, integer rgb triple, percentage HSL, percentage OkLCH, optional freq,
// and (when produced by image.extract) the normalised pin coordinate the
// Web UI's Kuler-style overlay uses to place draggable color picks.
type ColorEntry struct {
	Hex    string        `json:"hex" jsonschema:"canonical hex (#rrggbb)"`
	RGB    [3]uint8      `json:"rgb" jsonschema:"sRGB 8-bit channels"`
	HSL    [3]int        `json:"hsl" jsonschema:"HSL [hue °, saturation %, lightness %]"`
	OkLCH  [3]int        `json:"oklch" jsonschema:"OkLCH [lightness %, chroma %, hue °]"`
	Freq   float64       `json:"freq,omitempty" jsonschema:"frequency 0..1, populated for extracted palettes"`
	Source *color.Source `json:"source,omitempty" jsonschema:"normalised (0..1) representative-pixel coordinate; populated only for image.extract output"`
}

// encodePalette converts a *palette.Palette into the wire-friendly
// PaletteResult shape. Logic is intentionally identical to exporter.encodeColor.
func encodePalette(p *palette.Palette) PaletteResult {
	if p == nil {
		return PaletteResult{Colors: []ColorEntry{}}
	}
	out := PaletteResult{
		Size:   p.Len(),
		Name:   p.Name,
		Colors: encodeColors(p.Colors),
	}
	if !metadataEmpty(p.Metadata) {
		md := p.Metadata
		out.Metadata = &md
	}
	return out
}

func encodeColors(in []color.Color) []ColorEntry {
	out := make([]ColorEntry, len(in))
	for i, c := range in {
		out[i] = encodeColor(c)
	}
	return out
}

func encodeColor(c color.Color) ColorEntry {
	h, s, l := c.ToHSL()
	okL, okC, okH := c.ToOkLCH()
	return ColorEntry{
		Hex:    c.Hex(),
		RGB:    [3]uint8{c.R, c.G, c.B},
		HSL:    [3]int{roundDeg(h), pct(s), pct(l)},
		OkLCH:  [3]int{pct(okL), pct(okC), roundDeg(okH)},
		Freq:   c.Freq,
		Source: c.Source,
	}
}

func metadataEmpty(m palette.Metadata) bool {
	return m.Source == "" && m.Method == "" && m.Params == nil &&
		m.ImageInfo == nil && m.Stats == nil && m.GeneratedAt.IsZero()
}

func roundDeg(v float64) int {
	switch {
	case v < 0:
		return 0
	case v > 360:
		return 360
	}
	return int(v + 0.5)
}

func pct(v float64) int {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 100
	}
	return int(v*100 + 0.5)
}

// parseColors parses a slice of color strings, returning the first parse
// error encountered (with the offending string and index in the message).
// Used by every tool whose input is a list of colors (sort, blindness,
// gradient multi-stop, …).
func parseColors(in []string) ([]color.Color, error) {
	out := make([]color.Color, len(in))
	for i, raw := range in {
		c, err := color.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("colors[%d] %q: %w", i, raw, err)
		}
		out[i] = c
	}
	return out, nil
}
