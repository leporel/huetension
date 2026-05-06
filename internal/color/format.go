package color

import "fmt"

// Format identifies a target output representation.
type Format string

// Recognised formats.
const (
	FormatHex   Format = "hex"
	FormatHexU  Format = "hex-upper"
	FormatRGB   Format = "rgb"
	FormatHSL   Format = "hsl"
	FormatHSV   Format = "hsv"
	FormatHLS   Format = "hls"
	FormatLab   Format = "lab"
	FormatLCH   Format = "lch"
	FormatOkLab Format = "oklab"
	FormatOkLCH Format = "oklch"
	FormatName  Format = "name"
	FormatInt   Format = "int"
)

// AllFormats is the canonical iteration order used by the "all" output mode.
// Hex first (most useful), human-friendly forms next, perceptual spaces last.
var AllFormats = []Format{
	FormatHex,
	FormatRGB,
	FormatHSL,
	FormatHSV,
	FormatHLS,
	FormatLab,
	FormatLCH,
	FormatOkLab,
	FormatOkLCH,
	FormatName,
	FormatInt,
}

// Format renders the color in the requested format. The "name" format returns
// the closest CSS named color along with the ΔE distance, in the form
// "<name> (ΔE=<n>)" — exact matches drop the suffix.
func (c Color) Format(f Format) (string, error) {
	switch f {
	case FormatHex:
		return c.Hex(), nil
	case FormatHexU:
		return c.HexUpper(), nil
	case FormatRGB:
		return c.RGB(), nil
	case FormatHSL:
		return c.HSL(), nil
	case FormatHSV:
		return c.HSV(), nil
	case FormatHLS:
		return c.HLS(), nil
	case FormatLab:
		return c.Lab(), nil
	case FormatLCH:
		return c.LCH(), nil
	case FormatOkLab:
		return c.OkLab(), nil
	case FormatOkLCH:
		return c.OkLCH(), nil
	case FormatName:
		name, hex, d := c.NearestName()
		if name == "" {
			return "", fmt.Errorf("color: no named color match")
		}
		// Treat ΔE < 0.005 (in go-colorful's normalised Lab, ≈ 0.5 in CIE)
		// as an exact match.
		if d < 0.005 || hex == c.Hex() {
			return name, nil
		}
		return fmt.Sprintf("%s (ΔE=%s)", name, formatFloat(d*100, 2)), nil
	case FormatInt:
		return c.Int(), nil
	}
	return "", fmt.Errorf("color: unknown format %q", string(f))
}

// FormatAll returns all formats as an ordered slice of (name, value) pairs.
// Useful for the CLI `convert --to all` mode and the MCP `color.convert` tool.
type FormatPair struct {
	Format Format
	Value  string
}

// FormatAll returns the color in every format from AllFormats.
func (c Color) FormatAll() []FormatPair {
	out := make([]FormatPair, 0, len(AllFormats))
	for _, f := range AllFormats {
		v, err := c.Format(f)
		if err != nil {
			v = ""
		}
		out = append(out, FormatPair{Format: f, Value: v})
	}
	return out
}
