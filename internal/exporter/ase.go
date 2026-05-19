package exporter

import (
	"bytes"
	"math"
	"unicode/utf16"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// ASE (Adobe Swatch Exchange) wire constants. The format is big-endian
// throughout; layout per http://www.selapa.net/swatches/colors/fileformats.php.
const (
	aseSignature       = "ASEF"
	aseVersionMajor    = 1
	aseVersionMinor    = 0
	aseBlockColor      = 0x0001 // color-block tag
	aseColorTypeGlobal = 0      // 0 global, 1 spot, 2 normal
	aseRGBModel        = "RGB " // 4-char color-model tag (trailing space)
)

// renderASE encodes the palette as an Adobe Swatch Exchange (.ase) file.
// Each color becomes one RGB color block in a flat list — ASE color
// groups are not used. Output is binary.
func renderASE(p *palette.Palette) []byte {
	var buf bytes.Buffer
	buf.WriteString(aseSignature)
	writeBE16(&buf, aseVersionMajor)
	writeBE16(&buf, aseVersionMinor)
	writeBE32(&buf, uint32(p.Len()))

	for _, c := range p.Colors {
		block := aseColorBlock(c)
		writeBE16(&buf, aseBlockColor)
		writeBE32(&buf, uint32(len(block)))
		buf.Write(block)
	}
	return buf.Bytes()
}

// aseColorBlock builds the body of one color block: the UTF-16BE swatch
// name, the "RGB " model tag, the three 0..1 float32 components, and the
// color-type trailer. The block-length prefix is written by the caller.
func aseColorBlock(c color.Color) []byte {
	var b bytes.Buffer
	writeASEName(&b, c.Hex())
	b.WriteString(aseRGBModel)
	writeBE32(&b, math.Float32bits(float32(c.R)/255))
	writeBE32(&b, math.Float32bits(float32(c.G)/255))
	writeBE32(&b, math.Float32bits(float32(c.B)/255))
	writeBE16(&b, aseColorTypeGlobal)
	return b.Bytes()
}

// writeASEName writes an ASE name field: a uint16 count of UTF-16 code
// units including the trailing null, the UTF-16BE characters, then the
// two-byte null terminator.
func writeASEName(b *bytes.Buffer, s string) {
	units := utf16.Encode([]rune(s))
	writeBE16(b, uint16(len(units)+1))
	for _, u := range units {
		writeBE16(b, u)
	}
	writeBE16(b, 0)
}
