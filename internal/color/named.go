package color

import (
	"math"
	"strings"
	"sync"
)

// CSS / SVG named colors (Level 4). Lookup is case-insensitive and ignores
// surrounding whitespace; an underscore or hyphen is treated as no separator
// for the convenience of users typing things like "rebecca-purple".
var cssNamed = map[string]string{
	"aliceblue":            "#f0f8ff",
	"antiquewhite":         "#faebd7",
	"aqua":                 "#00ffff",
	"aquamarine":           "#7fffd4",
	"azure":                "#f0ffff",
	"beige":                "#f5f5dc",
	"bisque":               "#ffe4c4",
	"black":                "#000000",
	"blanchedalmond":       "#ffebcd",
	"blue":                 "#0000ff",
	"blueviolet":           "#8a2be2",
	"brown":                "#a52a2a",
	"burlywood":            "#deb887",
	"cadetblue":            "#5f9ea0",
	"chartreuse":           "#7fff00",
	"chocolate":            "#d2691e",
	"coral":                "#ff7f50",
	"cornflowerblue":       "#6495ed",
	"cornsilk":             "#fff8dc",
	"crimson":              "#dc143c",
	"cyan":                 "#00ffff",
	"darkblue":             "#00008b",
	"darkcyan":             "#008b8b",
	"darkgoldenrod":        "#b8860b",
	"darkgray":             "#a9a9a9",
	"darkgreen":            "#006400",
	"darkgrey":             "#a9a9a9",
	"darkkhaki":            "#bdb76b",
	"darkmagenta":          "#8b008b",
	"darkolivegreen":       "#556b2f",
	"darkorange":           "#ff8c00",
	"darkorchid":           "#9932cc",
	"darkred":              "#8b0000",
	"darksalmon":           "#e9967a",
	"darkseagreen":         "#8fbc8f",
	"darkslateblue":        "#483d8b",
	"darkslategray":        "#2f4f4f",
	"darkslategrey":        "#2f4f4f",
	"darkturquoise":        "#00ced1",
	"darkviolet":           "#9400d3",
	"deeppink":             "#ff1493",
	"deepskyblue":          "#00bfff",
	"dimgray":              "#696969",
	"dimgrey":              "#696969",
	"dodgerblue":           "#1e90ff",
	"firebrick":            "#b22222",
	"floralwhite":          "#fffaf0",
	"forestgreen":          "#228b22",
	"fuchsia":              "#ff00ff",
	"gainsboro":            "#dcdcdc",
	"ghostwhite":           "#f8f8ff",
	"gold":                 "#ffd700",
	"goldenrod":            "#daa520",
	"gray":                 "#808080",
	"green":                "#008000",
	"greenyellow":          "#adff2f",
	"grey":                 "#808080",
	"honeydew":             "#f0fff0",
	"hotpink":              "#ff69b4",
	"indianred":            "#cd5c5c",
	"indigo":               "#4b0082",
	"ivory":                "#fffff0",
	"khaki":                "#f0e68c",
	"lavender":             "#e6e6fa",
	"lavenderblush":        "#fff0f5",
	"lawngreen":            "#7cfc00",
	"lemonchiffon":         "#fffacd",
	"lightblue":            "#add8e6",
	"lightcoral":           "#f08080",
	"lightcyan":            "#e0ffff",
	"lightgoldenrodyellow": "#fafad2",
	"lightgray":            "#d3d3d3",
	"lightgreen":           "#90ee90",
	"lightgrey":            "#d3d3d3",
	"lightpink":            "#ffb6c1",
	"lightsalmon":          "#ffa07a",
	"lightseagreen":        "#20b2aa",
	"lightskyblue":         "#87cefa",
	"lightslategray":       "#778899",
	"lightslategrey":       "#778899",
	"lightsteelblue":       "#b0c4de",
	"lightyellow":          "#ffffe0",
	"lime":                 "#00ff00",
	"limegreen":            "#32cd32",
	"linen":                "#faf0e6",
	"magenta":              "#ff00ff",
	"maroon":               "#800000",
	"mediumaquamarine":     "#66cdaa",
	"mediumblue":           "#0000cd",
	"mediumorchid":         "#ba55d3",
	"mediumpurple":         "#9370db",
	"mediumseagreen":       "#3cb371",
	"mediumslateblue":      "#7b68ee",
	"mediumspringgreen":    "#00fa9a",
	"mediumturquoise":      "#48d1cc",
	"mediumvioletred":      "#c71585",
	"midnightblue":         "#191970",
	"mintcream":            "#f5fffa",
	"mistyrose":            "#ffe4e1",
	"moccasin":             "#ffe4b5",
	"navajowhite":          "#ffdead",
	"navy":                 "#000080",
	"oldlace":              "#fdf5e6",
	"olive":                "#808000",
	"olivedrab":            "#6b8e23",
	"orange":               "#ffa500",
	"orangered":            "#ff4500",
	"orchid":               "#da70d6",
	"palegoldenrod":        "#eee8aa",
	"palegreen":            "#98fb98",
	"paleturquoise":        "#afeeee",
	"palevioletred":        "#db7093",
	"papayawhip":           "#ffefd5",
	"peachpuff":            "#ffdab9",
	"peru":                 "#cd853f",
	"pink":                 "#ffc0cb",
	"plum":                 "#dda0dd",
	"powderblue":           "#b0e0e6",
	"purple":               "#800080",
	"rebeccapurple":        "#663399",
	"red":                  "#ff0000",
	"rosybrown":            "#bc8f8f",
	"royalblue":            "#4169e1",
	"saddlebrown":          "#8b4513",
	"salmon":               "#fa8072",
	"sandybrown":           "#f4a460",
	"seagreen":             "#2e8b57",
	"seashell":             "#fff5ee",
	"sienna":               "#a0522d",
	"silver":               "#c0c0c0",
	"skyblue":              "#87ceeb",
	"slateblue":            "#6a5acd",
	"slategray":            "#708090",
	"slategrey":            "#708090",
	"snow":                 "#fffafa",
	"springgreen":          "#00ff7f",
	"steelblue":            "#4682b4",
	"tan":                  "#d2b48c",
	"teal":                 "#008080",
	"thistle":              "#d8bfd8",
	"tomato":               "#ff6347",
	"turquoise":            "#40e0d0",
	"violet":               "#ee82ee",
	"wheat":                "#f5deb3",
	"white":                "#ffffff",
	"whitesmoke":           "#f5f5f5",
	"yellow":               "#ffff00",
	"yellowgreen":          "#9acd32",
}

// LookupNamed returns the Color matching a CSS named color. The lookup is
// case-insensitive and tolerates hyphens / underscores / whitespace.
func LookupNamed(name string) (Color, bool) {
	key := normaliseName(name)
	if hex, ok := cssNamed[key]; ok {
		c, err := ParseHex(hex)
		if err != nil {
			return Color{}, false
		}
		return c, true
	}
	return Color{}, false
}

// NamedHex returns the canonical hex string for a CSS named color.
func NamedHex(name string) (string, bool) {
	hex, ok := cssNamed[normaliseName(name)]
	return hex, ok
}

func normaliseName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	// Drop hyphens, underscores, and inner whitespace.
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '-' || r == '_' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// nearestCache memoises the Lab vectors of the named-color table.
type namedSample struct {
	name string
	lab  [3]float64
}

var (
	nearestOnce  sync.Once
	nearestCache []namedSample
)

func ensureNamedSamples() {
	nearestOnce.Do(func() {
		// Iteration order is irrelevant — we exhaustively compare distances.
		// Skip duplicate spellings (gray/grey, etc.) so the result is canonical.
		seen := make(map[string]bool, len(cssNamed))
		for name, hex := range cssNamed {
			if seen[hex] {
				continue
			}
			seen[hex] = true
			c, err := ParseHex(hex)
			if err != nil {
				continue
			}
			l, a, b := c.toColorful().Lab()
			nearestCache = append(nearestCache, namedSample{name: name, lab: [3]float64{l, a, b}})
		}
	})
}

// NearestName returns the closest CSS named color to c by ΔE (CIE76 in
// go-colorful's normalised Lab) along with its hex code and the distance.
// Distance is comparable; smaller is closer.
func (c Color) NearestName() (name, hex string, deltaE float64) {
	ensureNamedSamples()
	tl, ta, tb := c.toColorful().Lab()

	bestIdx := -1
	bestDist := math.Inf(1)
	for i, s := range nearestCache {
		dl := s.lab[0] - tl
		da := s.lab[1] - ta
		db := s.lab[2] - tb
		d := dl*dl + da*da + db*db
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return "", "", math.Inf(1)
	}
	s := nearestCache[bestIdx]
	hex = cssNamed[s.name]
	return s.name, hex, math.Sqrt(bestDist)
}
