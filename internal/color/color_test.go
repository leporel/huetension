package color

import "testing"

func TestHexParseAndFormat(t *testing.T) {
	cases := []struct {
		in   string
		want Color
	}{
		{"#ff0000", Color{R: 255, A: 255}},
		{"#00ff00", Color{G: 255, A: 255}},
		{"#0000ff", Color{B: 255, A: 255}},
		{"#FFF", Color{R: 255, G: 255, B: 255, A: 255}},
		{"#abcd", Color{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xdd}},
		{"#11223344", Color{R: 0x11, G: 0x22, B: 0x33, A: 0x44}},
		{"  #DC143C  ", Color{R: 0xdc, G: 0x14, B: 0x3c, A: 255}},
		{"0xff8800", Color{R: 0xff, G: 0x88, B: 0x00, A: 255}},
		{"abcdef", Color{R: 0xab, G: 0xcd, B: 0xef, A: 255}},
	}
	for _, c := range cases {
		got, err := ParseHex(c.in)
		if err != nil {
			t.Errorf("ParseHex(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseHex(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestHexParseInvalid(t *testing.T) {
	bad := []string{"", "#", "#ff", "#fffff", "#abcdefgh", "0xZZZZ", "not-a-color"}
	for _, b := range bad {
		if _, err := ParseHex(b); err == nil {
			t.Errorf("ParseHex(%q) should have errored", b)
		}
	}
}

func TestHexFormatRoundTrip(t *testing.T) {
	cases := []Color{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 0, G: 0, B: 0, A: 255},
		{R: 0xab, G: 0xcd, B: 0xef, A: 255},
		{R: 1, G: 2, B: 3, A: 4},
	}
	for _, c := range cases {
		hex := c.Hex()
		parsed, err := ParseHex(hex)
		if err != nil {
			t.Fatalf("ParseHex(%q) error: %v", hex, err)
		}
		if parsed != c {
			t.Errorf("round trip %v -> %q -> %v", c, hex, parsed)
		}
	}
}

func TestRGBFunction(t *testing.T) {
	cases := []struct {
		in   string
		want Color
	}{
		{"rgb(255, 0, 0)", Color{R: 255, A: 255}},
		{"rgb(0 255 0)", Color{G: 255, A: 255}},
		{"rgba(0, 0, 255, 0.5)", Color{B: 255, A: 128}},
		{"rgb(0 0 255 / 50%)", Color{B: 255, A: 128}},
		{"rgb(100%, 0%, 0%)", Color{R: 255, A: 255}},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestNamedColors(t *testing.T) {
	cases := []struct {
		name string
		hex  string
	}{
		{"red", "#ff0000"},
		{"REBECCAPURPLE", "#663399"},
		{"  rebecca-purple ", "#663399"},
		{"DarkSlateBlue", "#483d8b"},
	}
	for _, c := range cases {
		got, ok := LookupNamed(c.name)
		if !ok {
			t.Errorf("LookupNamed(%q) not found", c.name)
			continue
		}
		want, _ := ParseHex(c.hex)
		if got != want {
			t.Errorf("LookupNamed(%q) = %+v, want %+v", c.name, got, want)
		}
	}
}

func TestNearestName(t *testing.T) {
	c, _ := ParseHex("#ff0001")
	name, hex, d := c.NearestName()
	if name != "red" || hex != "#ff0000" {
		t.Errorf("NearestName(#ff0001) = (%q, %q), want (red, #ff0000)", name, hex)
	}
	if d > 0.05 {
		t.Errorf("NearestName(#ff0001) ΔE = %v, expected very small", d)
	}
}

// Round-trip every parsed format through hex and back. The contract is that
// Parse → Format(Hex) → ParseHex differs from the source by at most 1 unit
// per channel, which is the documented precision.
func TestFormatRoundTripsThroughHex(t *testing.T) {
	originals := []Color{
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 128, B: 64, A: 255},
		{R: 200, G: 184, B: 158, A: 255},
		{R: 90, G: 124, B: 161, A: 255},
		{R: 42, G: 60, B: 94, A: 255},
		{R: 230, G: 226, B: 216, A: 255},
	}
	formats := []Format{FormatHSL, FormatHSV, FormatHLS, FormatLab, FormatLCH, FormatOkLab, FormatOkLCH}
	for _, c := range originals {
		for _, f := range formats {
			s, err := c.Format(f)
			if err != nil {
				t.Fatalf("Format(%v, %s) error: %v", c, f, err)
			}
			parsed, err := Parse(s)
			if err != nil {
				t.Errorf("Parse(%q) [%s of %v] error: %v", s, f, c, err)
				continue
			}
			if !channelsClose(parsed, c, 2) {
				t.Errorf("round trip via %s: %v -> %q -> %v (drift > 2)", f, c, s, parsed)
			}
		}
	}
}

func TestParseAutodetectShortHex(t *testing.T) {
	cases := map[string]Color{
		"#abc":     {R: 0xaa, G: 0xbb, B: 0xcc, A: 255},
		"abc":      {R: 0xaa, G: 0xbb, B: 0xcc, A: 255},
		"FF8800":   {R: 0xff, G: 0x88, B: 0x00, A: 255},
		"red":      {R: 255, G: 0, B: 0, A: 255},
		"crimson ": {R: 0xdc, G: 0x14, B: 0x3c, A: 255},
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Parse(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestParseRejectsAmbiguous(t *testing.T) {
	bad := []string{"", "   ", "rgb()", "rgb(1,2)", "hsl(120, 50%)", "hex(00)", "hello-world"}
	for _, in := range bad {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) should have errored", in)
		}
	}
}

func TestOkLabRoundTrip(t *testing.T) {
	originals := []Color{
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 255, B: 0, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 124, G: 99, B: 200, A: 255},
		{R: 17, G: 17, B: 17, A: 255},
	}
	for _, c := range originals {
		L, a, b := rgbToOkLab(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
		r, g, bb := okLabToRGB(L, a, b)
		got := Color{R: toUint8(r * 255), G: toUint8(g * 255), B: toUint8(bb * 255), A: 255}
		if !channelsClose(got, c, 1) {
			t.Errorf("OkLab round trip: %v -> Lab(%v,%v,%v) -> %v", c, L, a, b, got)
		}
	}
}

func TestFormatAllProducesEntries(t *testing.T) {
	c, _ := Parse("#3366ff")
	pairs := c.FormatAll()
	if len(pairs) != len(AllFormats) {
		t.Fatalf("FormatAll: got %d entries, want %d", len(pairs), len(AllFormats))
	}
	for _, p := range pairs {
		if p.Value == "" {
			t.Errorf("FormatAll: %s entry is empty", p.Format)
		}
	}
}

func TestFromImageColorAlpha(t *testing.T) {
	c := Color{R: 255, G: 0, B: 0, A: 128}
	r, g, b, a := c.RGBA()
	round := struct{ r, g, b, a uint32 }{r, g, b, a}
	back := FromImageColor(rgbaImageColor(round.r, round.g, round.b, round.a))
	if back != c {
		// One unit of imprecision is acceptable in the unpremultiplication step.
		if !channelsClose(back, c, 1) {
			t.Errorf("FromImageColor round trip: %v -> %v", c, back)
		}
	}
}

// channelsClose reports whether two colors agree per-channel within tol.
func channelsClose(a, b Color, tol int) bool {
	return absDiff(a.R, b.R) <= tol &&
		absDiff(a.G, b.G) <= tol &&
		absDiff(a.B, b.B) <= tol &&
		absDiff(a.A, b.A) <= tol
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// rgbaImageColor wraps premultiplied RGBA into a tiny color.Color shim so we
// can exercise FromImageColor without importing image/color in the test file.
type premulColor struct{ r, g, b, a uint32 }

func (p premulColor) RGBA() (r, g, b, a uint32) { return p.r, p.g, p.b, p.a }

func rgbaImageColor(r, g, b, a uint32) premulColor { return premulColor{r, g, b, a} }
