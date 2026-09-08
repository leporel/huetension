package tools

import (
	"fmt"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/exporter"
)

// PaletteResult is the `result` block every palette-bearing MCP tool
// returns. It is the exporter's own type, so MCP, CLI and Web emit one
// wire shape (`result.palette.colors` + `result.metadata`) from a single
// encoder — see exporter.EncodeResult.
type PaletteResult = exporter.ResultJSON

// ColorEntry is one row of a colors[] array; shared with the CLI / Web
// encoders for the same reason as PaletteResult.
type ColorEntry = exporter.ColorJSON

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
