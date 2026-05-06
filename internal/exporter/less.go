package exporter

import (
	"bytes"
	"fmt"

	"github.com/leporel/huetension/internal/palette"
)

// renderLESS emits LESS variable declarations: `@prefix-1: #aabbcc;`. LESS
// uses `@` as the variable sigil rather than SCSS's `$`. Mirrors renderSCSS
// — same per-color line + a list variable for iteration via `each(@list)`.
func renderLESS(p *palette.Palette, opts Options) []byte {
	var buf bytes.Buffer
	for i, c := range p.Colors {
		fmt.Fprintf(&buf, "@%s-%d: %s;\n", opts.Prefix, i+1, c.Hex())
	}
	buf.WriteByte('\n')
	fmt.Fprintf(&buf, "@%s-list: ", opts.Prefix)
	for i := range p.Colors {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "@%s-%d", opts.Prefix, i+1)
	}
	buf.WriteString(";\n")
	return buf.Bytes()
}
