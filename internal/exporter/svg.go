package exporter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/leporel/huetension/internal/palette"
)

// renderSVG emits a standalone SVG document carrying a <linearGradient>
// built from the palette's colors — each color becomes an evenly-spaced
// <stop>. A filled <rect> references the gradient so the file previews
// directly in a browser or vector editor; the <linearGradient> block can
// also be lifted out and reused on its own.
func renderSVG(p *palette.Palette, opts Options) []byte {
	colors := p.Colors
	n := len(colors)

	name := paletteName(p, opts)
	if name == "" {
		name = opts.Name
	}
	id := svgIdent(name)

	var buf bytes.Buffer
	buf.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="320" height="64" viewBox="0 0 320 64">` + "\n")
	buf.WriteString("  <defs>\n")
	fmt.Fprintf(&buf, "    <linearGradient id=%q x1=\"0%%\" y1=\"0%%\" x2=\"100%%\" y2=\"0%%\">\n", id)
	for i, c := range colors {
		offset := 0.0
		if n > 1 {
			offset = float64(i) / float64(n-1) * 100
		}
		fmt.Fprintf(&buf, "      <stop offset=\"%.1f%%\" stop-color=%q />\n", offset, c.Hex())
	}
	buf.WriteString("    </linearGradient>\n")
	buf.WriteString("  </defs>\n")
	fmt.Fprintf(&buf, "  <rect width=\"320\" height=\"64\" fill=\"url(#%s)\" />\n", id)
	buf.WriteString("</svg>\n")
	return buf.Bytes()
}

// svgIdent sanitises name into a valid SVG id / CSS token: only letters,
// digits, '-' and '_' survive, runs of other characters collapse to a
// single '-'. Never returns empty, and never starts with a digit (an id
// may not, and neither may a CSS class reusing it).
func svgIdent(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		return "gradient"
	}
	if id[0] >= '0' && id[0] <= '9' {
		id = "g-" + id
	}
	return id
}
