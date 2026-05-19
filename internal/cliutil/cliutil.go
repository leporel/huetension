// Package cliutil renders palettes and color lists for terminal output.
//
// Output is human-friendly: each row gets a colored swatch (background
// painted via lipgloss styling), followed by hex / rgb / freq columns
// aligned by manual width-padding. Two modes:
//
//   - Colors enabled (default): lipgloss handles colour-profile detection
//     (TrueColor / 256 / 16 / NoColor) automatically and emits the right
//     ANSI escapes for the host terminal.
//   - Colors disabled (--no-color or NO_COLOR env): plain whitespace where
//     the swatch would be — alignment stays intact.
//
// We use lipgloss for swatches but NOT tablewriter for columns: tablewriter
// v1.1+ transitively pulls in charmbracelet/x/cellbuf which has a moving-
// target relationship with charmbracelet/x/ansi (version skew breaks the
// build). Manual %-*s padding is ~5 lines and avoids that fragility.
//
// The package is shared by the CLI today and will back the Web UI's
// "preview" tab in Phase 3, so it deliberately exposes pure-Go helpers
// rather than committing to a specific TUI runtime at the API surface.
package cliutil

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// Options configures the renderers. Zero values produce sensible defaults
// (colors on, two-cell swatch, frequency column shown when populated).
type Options struct {
	// NoColor disables ANSI escape sequences. Honour this when stdout is
	// not a TTY or when the user explicitly opts out.
	NoColor bool

	// SwatchWidth is the number of cells per swatch. Default 2 — a single
	// cell renders too thin in most monospace fonts.
	SwatchWidth int
}

// ColorsEnabled reports whether ANSI escapes should be emitted. Honours
// opts.NoColor first, then the NO_COLOR environment variable
// (https://no-color.org).
func ColorsEnabled(opts Options) bool {
	if opts.NoColor {
		return false
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	return true
}

// Swatch renders a single colored block for c. lipgloss handles the
// colour-profile detection (TrueColor / 256 / 16 / NoColor) so the right
// ANSI sequence is emitted for the host terminal. When colors are
// disabled we emit plain whitespace of the same width to keep alignment.
func Swatch(c color.Color, opts Options) string {
	w := opts.SwatchWidth
	if w <= 0 {
		w = 2
	}
	pad := strings.Repeat(" ", w)
	if !ColorsEnabled(opts) {
		return pad
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color(c.Hex())).
		Render(pad)
}

// RenderPalette formats p as a multi-line text block: one row per color,
// columns swatch / hex / rgb / hsl / hsv / freq% (the freq column is
// omitted when every color has zero frequency, e.g. for harmony / gradient
// output).
func RenderPalette(p *palette.Palette, opts Options) string {
	if p == nil || p.Len() == 0 {
		return ""
	}
	return RenderColors(p.Colors, opts)
}

// RenderColors is the slice-level variant of RenderPalette. Used by
// commands that work on bare []color.Color (sort, convert).
func RenderColors(colors []color.Color, opts Options) string {
	if len(colors) == 0 {
		return ""
	}

	hasFreq := false
	maxHex, maxRGB, maxHSL, maxHSV := 0, 0, 0, 0
	for _, c := range colors {
		if c.Freq > 0 {
			hasFreq = true
		}
		if l := len(c.Hex()); l > maxHex {
			maxHex = l
		}
		if l := len(c.RGB()); l > maxRGB {
			maxRGB = l
		}
		if l := len(c.HSL()); l > maxHSL {
			maxHSL = l
		}
		if l := len(c.HSV()); l > maxHSV {
			maxHSV = l
		}
	}

	var b strings.Builder
	for _, c := range colors {
		// Both hex and rgb columns pad to the widest entry in this batch
		// (mixed opaque/translucent palettes can have 7- vs 9-char hex,
		// and rgb() length varies with digit count). Swatch comes first
		// and isn't fmt-padded — its width is fixed in Swatch().
		fmt.Fprintf(&b, "%s  %-*s  %-*s  %-*s  %-*s", Swatch(c, opts), maxHex, c.Hex(), maxRGB, c.RGB(), maxHSL, c.HSL(), maxHSV, c.HSV())
		if hasFreq {
			fmt.Fprintf(&b, "  %5.1f%%", c.Freq*100)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// RenderHeader formats a one-line header for palette-producing commands
// (e.g. "Extracted 4 colors from photo.jpg"). Returns the line with a
// trailing newline; callers prepend it to RenderPalette output. When verb
// is empty the header is suppressed.
func RenderHeader(verb, source string, count int) string {
	if verb == "" {
		return ""
	}
	if source == "" {
		return fmt.Sprintf("%s %d colors\n", verb, count)
	}
	return fmt.Sprintf("%s %d colors from %s\n", verb, count, source)
}
