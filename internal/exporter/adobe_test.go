package exporter

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// mustExport runs Export and fails the test on error.
func mustExport(t *testing.T, p *palette.Palette, f Format) []byte {
	t.Helper()
	out, err := Export(p, f, Options{})
	if err != nil {
		t.Fatalf("Export(%s): %v", f, err)
	}
	return out
}

// ---------- ASE ----------

// TestASEExactBytesSingleColor pins the full ASE byte layout for one
// fully-saturated red color — signature, version, block count, the color
// block header, the UTF-16BE name, the "RGB " model tag, the three
// float32 components, and the color-type trailer.
func TestASEExactBytesSingleColor(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})
	out := mustExport(t, p, FormatASE)

	want := []byte{
		'A', 'S', 'E', 'F',
		0x00, 0x01, // version major
		0x00, 0x00, // version minor
		0x00, 0x00, 0x00, 0x01, // 1 block
		0x00, 0x01, // block type: color
		0x00, 0x00, 0x00, 0x24, // block length 36
		0x00, 0x08, // name length 8 (7 chars + null)
		0x00, 0x23, 0x00, 0x66, 0x00, 0x66, // "#ff"
		0x00, 0x30, 0x00, 0x30, 0x00, 0x30, 0x00, 0x30, // "0000"
		0x00, 0x00, // name null terminator
		'R', 'G', 'B', ' ',
		0x3F, 0x80, 0x00, 0x00, // R = 1.0
		0x00, 0x00, 0x00, 0x00, // G = 0.0
		0x00, 0x00, 0x00, 0x00, // B = 0.0
		0x00, 0x00, // color type: global
	}
	if !bytes.Equal(out, want) {
		t.Errorf("ASE bytes mismatch\n got: % x\nwant: % x", out, want)
	}
}

// TestASEHeaderAndBlocks checks a multi-color palette: the header fields
// and that the declared block lengths consume the buffer exactly.
func TestASEHeaderAndBlocks(t *testing.T) {
	p := fixturePalette() // 3 colors
	out := mustExport(t, p, FormatASE)

	if string(out[:4]) != "ASEF" {
		t.Fatalf("signature = %q, want ASEF", out[:4])
	}
	if v := binary.BigEndian.Uint16(out[4:6]); v != 1 {
		t.Errorf("major version = %d, want 1", v)
	}
	if v := binary.BigEndian.Uint16(out[6:8]); v != 0 {
		t.Errorf("minor version = %d, want 0", v)
	}
	if n := binary.BigEndian.Uint32(out[8:12]); n != 3 {
		t.Errorf("block count = %d, want 3", n)
	}

	off := 12
	for i := range 3 {
		if off+6 > len(out) {
			t.Fatalf("block %d: truncated at offset %d", i, off)
		}
		if typ := binary.BigEndian.Uint16(out[off : off+2]); typ != 0x0001 {
			t.Errorf("block %d type = %#x, want 0x0001", i, typ)
		}
		length := binary.BigEndian.Uint32(out[off+2 : off+6])
		off += 6 + int(length)
	}
	if off != len(out) {
		t.Errorf("blocks consumed %d bytes, buffer is %d", off, len(out))
	}
}

// ---------- ACO ----------

// TestACOExactBytesSingleColor pins the full ACO byte layout for one red
// color — both the version 1 and version 2 sections, the 16-bit RGB
// components, and the v2 pascal-style name.
func TestACOExactBytesSingleColor(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})
	out := mustExport(t, p, FormatACO)

	want := []byte{
		// version 1 section
		0x00, 0x01, // version
		0x00, 0x01, // 1 color
		0x00, 0x00, // color space: RGB
		0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // R,G,B,unused
		// version 2 section
		0x00, 0x02, // version
		0x00, 0x01, // 1 color
		0x00, 0x00, // color space: RGB
		0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // R,G,B,unused
		0x00, 0x00, 0x00, 0x08, // name length 8 (7 chars + null)
		0x00, 0x23, 0x00, 0x66, 0x00, 0x66, // "#ff"
		0x00, 0x30, 0x00, 0x30, 0x00, 0x30, 0x00, 0x30, // "0000"
		0x00, 0x00, // name null terminator
	}
	if !bytes.Equal(out, want) {
		t.Errorf("ACO bytes mismatch\n got: % x\nwant: % x", out, want)
	}
}

// TestACOSections checks a multi-color palette: both sections, their
// versions, and the color counts.
func TestACOSections(t *testing.T) {
	p := fixturePalette() // 3 colors
	out := mustExport(t, p, FormatACO)

	if v := binary.BigEndian.Uint16(out[0:2]); v != 1 {
		t.Errorf("v1 version = %d, want 1", v)
	}
	if n := binary.BigEndian.Uint16(out[2:4]); n != 3 {
		t.Errorf("v1 count = %d, want 3", n)
	}
	if cs := binary.BigEndian.Uint16(out[4:6]); cs != 0 {
		t.Errorf("first color space = %d, want 0 (RGB)", cs)
	}

	// Each v1 record is 10 bytes (2 space + 4×2 components); the v2
	// section starts after the 4-byte v1 header plus 3 records.
	v2 := 4 + 3*10
	if v := binary.BigEndian.Uint16(out[v2 : v2+2]); v != 2 {
		t.Errorf("v2 version = %d, want 2", v)
	}
	if n := binary.BigEndian.Uint16(out[v2+2 : v2+4]); n != 3 {
		t.Errorf("v2 count = %d, want 3", n)
	}
}

// TestACOChannelScaling verifies an 8-bit channel maps onto the full
// 16-bit ACO range (255 → 65535, 128 → 128×257).
func TestACOChannelScaling(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 128, 0)})
	out := mustExport(t, p, FormatACO)

	cases := []struct {
		off  int
		want uint16
	}{
		{6, 65535}, // R: 255  → 65535
		{8, 32896}, // G: 128  → 128×257
		{10, 0},    // B: 0    → 0
	}
	for _, c := range cases {
		if got := binary.BigEndian.Uint16(out[c.off : c.off+2]); got != c.want {
			t.Errorf("component at offset %d = %d, want %d", c.off, got, c.want)
		}
	}
}

// TestAdobeExportsDeterministic guards the determinism rule — repeated
// exports of the same palette are byte-identical.
func TestAdobeExportsDeterministic(t *testing.T) {
	p := fixturePalette()
	for _, f := range []Format{FormatASE, FormatACO} {
		first := mustExport(t, p, f)
		second := mustExport(t, p, f)
		if !bytes.Equal(first, second) {
			t.Errorf("%s: output not deterministic", f)
		}
	}
}

// TestAdobeFormatsAreBinary confirms IsBinary classifies ASE/ACO so the
// JSON/web transports base64-encode them.
func TestAdobeFormatsAreBinary(t *testing.T) {
	for _, f := range []Format{FormatASE, FormatACO} {
		if !IsBinary(f) {
			t.Errorf("IsBinary(%s) = false, want true", f)
		}
	}
}
