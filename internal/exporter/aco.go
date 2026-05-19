package exporter

import (
	"bytes"
	"unicode/utf16"

	"github.com/leporel/huetension/internal/palette"
)

// ACO (Adobe Photoshop Color Swatch) wire constants. All values are
// big-endian; colors are written in the RGB color space with 16-bit
// channels. Spec: Adobe Photoshop file-format reference.
const (
	acoVersion1 = 1
	acoVersion2 = 2
	acoSpaceRGB = 0
	// acoChannelScale maps an 8-bit channel (0..255) onto ACO's full
	// 16-bit range (0..65535). 65535 / 255 is exactly 257.
	acoChannelScale = 257
)

// renderACO encodes the palette as an Adobe Photoshop Color Swatch
// (.aco) file. Both the version 1 and the version 2 sections are
// written; only the v2 section carries swatch names. Output is binary.
func renderACO(p *palette.Palette) []byte {
	var buf bytes.Buffer
	writeACOSection(&buf, p, acoVersion1)
	writeACOSection(&buf, p, acoVersion2)
	return buf.Bytes()
}

// writeACOSection writes one ACO section. The v1 and v2 sections carry
// the same color records; v2 additionally appends a name after every
// color.
func writeACOSection(b *bytes.Buffer, p *palette.Palette, version uint16) {
	writeBE16(b, version)
	writeBE16(b, uint16(p.Len()))
	for _, c := range p.Colors {
		writeBE16(b, acoSpaceRGB)
		writeBE16(b, uint16(c.R)*acoChannelScale)
		writeBE16(b, uint16(c.G)*acoChannelScale)
		writeBE16(b, uint16(c.B)*acoChannelScale)
		writeBE16(b, 0) // 4th component, unused for RGB
		if version == acoVersion2 {
			writeACOName(b, c.Hex())
		}
	}
}

// writeACOName writes a color name in Adobe's pascal-style format: a
// uint32 count of UTF-16 code units including the trailing null, the
// UTF-16BE characters, then the two-byte null terminator.
func writeACOName(b *bytes.Buffer, s string) {
	units := utf16.Encode([]rune(s))
	writeBE32(b, uint32(len(units)+1))
	for _, u := range units {
		writeBE16(b, u)
	}
	writeBE16(b, 0)
}
