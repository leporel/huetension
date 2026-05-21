package exporter

import (
	"bytes"
	"fmt"

	"github.com/leporel/huetension/internal/palette"
)

// renderSCSS emits SCSS variable declarations: `$prefix-1: <color>;`. We
// also emit a `$prefix-list` Sass list at the end so consumers can iterate
// over all swatches with `@each $c in $color-list`.
func renderSCSS(p *palette.Palette, opts Options) []byte {
	var buf bytes.Buffer
	for i, c := range p.Colors {
		fmt.Fprintf(&buf, "$%s-%d: %s;\n", opts.Prefix, i+1, colorValue(c, opts))
	}
	buf.WriteByte('\n')
	fmt.Fprintf(&buf, "$%s-list: (", opts.Prefix)
	for i := range p.Colors {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "$%s-%d", opts.Prefix, i+1)
	}
	buf.WriteString(");\n")
	return buf.Bytes()
}
