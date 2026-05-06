package exporter

import (
	"bytes"
	"fmt"

	"github.com/leporel/huetension/internal/palette"
)

// renderGPL emits a GIMP palette (.gpl) — a plain-text format also
// understood by Inkscape, Krita, and a wide range of other open-source
// design tools. Spec is informal but the de-facto layout is:
//
//	GIMP Palette
//	Name: <palette name>
//	Columns: 0
//	#
//	  R   G   B  Color Name
//	...
//
// Three-digit zero-padded columns are conventional; we follow them so the
// output looks the way GIMP itself emits palettes.
func renderGPL(p *palette.Palette, opts Options) []byte {
	name := paletteName(p, opts)
	if name == "" {
		name = opts.Name
	}

	var buf bytes.Buffer
	buf.WriteString("GIMP Palette\n")
	fmt.Fprintf(&buf, "Name: %s\n", name)
	buf.WriteString("Columns: 0\n")
	buf.WriteString("#\n")
	for i, c := range p.Colors {
		fmt.Fprintf(&buf, "%3d %3d %3d\t%s-%d\n", c.R, c.G, c.B, opts.Prefix, i+1)
	}
	return buf.Bytes()
}
