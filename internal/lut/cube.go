package lut

import (
	"fmt"
	"strings"
)

// EncodeCube encodes a LUT as a .cube file (Autodesk LUT format).
func EncodeCube(l *LUT, title string) []byte {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# huetension LUT\n")
	fmt.Fprintf(&sb, "TITLE \"%s\"\n", title)
	fmt.Fprintf(&sb, "LUT_3D_SIZE %d\n", l.Size)
	sb.WriteByte('\n')

	// Output nodes in red-fastest order.
	for _, node := range l.Nodes {
		r := float64(node.R) / 255.0
		g := float64(node.G) / 255.0
		b := float64(node.B) / 255.0
		fmt.Fprintf(&sb, "%.6f %.6f %.6f\n", r, g, b)
	}

	return []byte(sb.String())
}
