// Package exporter renders a *palette.Palette into one of several text
// formats: JSON (the huetension/v1 wire contract), CSS custom properties,
// SCSS variables, a Tailwind theme.extend.colors snippet, plain hex per
// line, and the GIMP .gpl format.
//
// The package has a single Export entrypoint dispatching by Format. Each
// format lives in its own file so adding a new one is a matter of writing
// one renderFoo function and registering the format constant.
package exporter

import (
	"errors"
	"fmt"

	"github.com/leporel/huetension/internal/palette"
)

// Format selects an output flavour. Format strings double as CLI flag
// values and file extensions (json/css/scss/tailwind/txt/gpl).
type Format string

const (
	FormatJSON     Format = "json"
	FormatCSS      Format = "css"
	FormatSCSS     Format = "scss"
	FormatLESS     Format = "less"
	FormatTailwind Format = "tailwind"
	FormatPlain    Format = "txt"
	FormatGPL      Format = "gpl"
	// FormatPNG / FormatJPEG render the palette as a horizontal strip of
	// solid swatches. Useful for thumbnails, design-review screenshots, and
	// the "save palette next to the source image" workflow. Output is
	// binary, not text — the rest of the pipeline (writeOutput, file
	// handles) treats []byte the same regardless.
	FormatPNG  Format = "png"
	FormatJPEG Format = "jpeg"
)

// AllFormats lists every supported export Format. Used by tests and CLI
// flag completion.
var AllFormats = []Format{
	FormatJSON,
	FormatCSS,
	FormatSCSS,
	FormatLESS,
	FormatTailwind,
	FormatPlain,
	FormatGPL,
	FormatPNG,
	FormatJPEG,
}

// Options configures format-specific rendering. Zero values are sensible
// defaults — only override what you need.
type Options struct {
	// Prefix used for generated identifiers in CSS/SCSS/Tailwind. Defaults
	// to "color" so variables look like --color-1, $color-2, color-3.
	Prefix string

	// Pretty toggles indented JSON output. Default (false) produces a
	// compact single-line document, matching the wire contract.
	Pretty bool

	// Name overrides the palette's display name in formats that surface it
	// (GPL header, JSON when palette.Name is empty). Falls back to "huetension".
	Name string

	// SwatchWidth and SwatchHeight control the per-color block size for
	// image renders (PNG / JPEG). Zero falls back to 96×96 — matches the
	// testdata previews under internal/extract/testdata/.
	SwatchWidth  int
	SwatchHeight int

	// Tool identifies the entry point that produced this palette — set by
	// the CLI ("extract", "harmony", …) or by the MCP server (the tool
	// name registered with `mcp.RegisterTool`). Library callers leave it
	// empty; the JSON envelope's `tool` field is then omitted.
	Tool string

	// Params is the set of arguments the tool was invoked with. Mirrors
	// the JSON envelope's `params` field. Library callers leave it nil;
	// CLI/MCP populate it so consumers can replay the call exactly.
	Params map[string]any
}

const (
	defaultPrefix = "color"
	defaultName   = "huetension"
)

// ErrUnknownFormat is returned by Export when the supplied Format is not
// in AllFormats.
var ErrUnknownFormat = errors.New("unknown export format")

// Export renders p into the requested format. opts is consulted only for
// fields the format actually uses, so e.g. Pretty has no effect on CSS.
func Export(p *palette.Palette, format Format, opts Options) ([]byte, error) {
	if p == nil {
		return nil, errors.New("exporter: nil palette")
	}
	if p.Len() == 0 {
		return nil, errors.New("exporter: empty palette")
	}

	opts = applyOptionDefaults(opts)

	switch format {
	case FormatJSON:
		return renderJSON(p, opts)
	case FormatCSS:
		return renderCSS(p, opts), nil
	case FormatSCSS:
		return renderSCSS(p, opts), nil
	case FormatLESS:
		return renderLESS(p, opts), nil
	case FormatTailwind:
		return renderTailwind(p, opts), nil
	case FormatPlain:
		return renderPlain(p), nil
	case FormatGPL:
		return renderGPL(p, opts), nil
	case FormatPNG:
		return renderSwatchPNG(p, opts)
	case FormatJPEG:
		return renderSwatchJPEG(p, opts)
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownFormat, format)
}

// FileExtension returns the conventional file extension for a Format,
// without the leading dot. Useful for CLI auto-naming output files.
func FileExtension(format Format) string {
	switch format {
	case FormatJSON:
		return "json"
	case FormatCSS:
		return "css"
	case FormatSCSS:
		return "scss"
	case FormatLESS:
		return "less"
	case FormatTailwind:
		return "js"
	case FormatPlain:
		return "txt"
	case FormatGPL:
		return "gpl"
	case FormatPNG:
		return "png"
	case FormatJPEG:
		return "jpg"
	}
	return ""
}

// FormatFromExtension maps a file extension (with or without leading dot,
// case-insensitive) to its Format. Returns the empty string + false when
// the extension doesn't match any known format. Used by the CLI to infer
// the format from the -o path when --format wasn't explicitly set.
func FormatFromExtension(ext string) (Format, bool) {
	switch ext {
	case "json", ".json":
		return FormatJSON, true
	case "css", ".css":
		return FormatCSS, true
	case "scss", ".scss":
		return FormatSCSS, true
	case "less", ".less":
		return FormatLESS, true
	case "js", ".js":
		return FormatTailwind, true
	case "txt", ".txt":
		return FormatPlain, true
	case "gpl", ".gpl":
		return FormatGPL, true
	case "png", ".png":
		return FormatPNG, true
	case "jpg", "jpeg", ".jpg", ".jpeg":
		return FormatJPEG, true
	}
	return "", false
}

func applyOptionDefaults(opts Options) Options {
	if opts.Prefix == "" {
		opts.Prefix = defaultPrefix
	}
	if opts.Name == "" {
		opts.Name = defaultName
	}
	return opts
}
