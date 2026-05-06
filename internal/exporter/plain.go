package exporter

import (
	"bytes"

	"github.com/leporel/huetension/internal/palette"
)

// renderPlain emits one #RRGGBB hex code per line. The simplest possible
// format — useful for piping into other tools or for `cat` previews.
func renderPlain(p *palette.Palette) []byte {
	var buf bytes.Buffer
	for _, c := range p.Colors {
		buf.WriteString(c.Hex())
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}
