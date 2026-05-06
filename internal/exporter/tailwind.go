package exporter

import (
	"bytes"
	"fmt"

	"github.com/leporel/huetension/internal/palette"
)

// renderTailwind emits a JavaScript snippet ready to drop into
// tailwind.config.js under theme.extend.colors. We wrap the colors object
// in a self-contained module.exports stanza so the file is also executable
// as-is for users who want to import it.
//
// Output shape:
//
//	// huetension palette — paste into tailwind.config.js theme.extend.colors
//	module.exports = {
//	  "color-1": "#aabbcc",
//	  "color-2": "#ddeeff",
//	};
func renderTailwind(p *palette.Palette, opts Options) []byte {
	var buf bytes.Buffer
	buf.WriteString("// huetension palette — paste into tailwind.config.js theme.extend.colors\n")
	buf.WriteString("module.exports = {\n")
	for i, c := range p.Colors {
		fmt.Fprintf(&buf, "  %q: %q,\n", fmt.Sprintf("%s-%d", opts.Prefix, i+1), c.Hex())
	}
	buf.WriteString("};\n")
	return buf.Bytes()
}
