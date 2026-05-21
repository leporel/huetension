package exporter

import (
	"bytes"

	"github.com/leporel/huetension/internal/palette"
)

// renderPlain emits one color value per line in the notation selected in opts.ColorNotation.
// The simplest possible format — useful for piping into other tools or for `cat` previews.
func renderPlain(p *palette.Palette, opts Options) []byte {
	var buf bytes.Buffer
	for _, c := range p.Colors {
		buf.WriteString(colorValue(c, opts))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}
