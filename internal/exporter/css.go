package exporter

import (
	"bytes"
	"fmt"

	"github.com/leporel/huetension/internal/palette"
)

// renderCSS emits a `:root { ... }` block of CSS custom properties. Variable
// names follow `--{prefix}-{1-based-index}` for stable ordering, e.g.
// `--color-1`, `--color-2`. Index starts at 1 because that's what
// designers expect when reading "color one, color two".
func renderCSS(p *palette.Palette, opts Options) []byte {
	var buf bytes.Buffer
	buf.WriteString(":root {\n")
	for i, c := range p.Colors {
		fmt.Fprintf(&buf, "  --%s-%d: %s;\n", opts.Prefix, i+1, c.Hex())
	}
	buf.WriteString("}\n")
	return buf.Bytes()
}
