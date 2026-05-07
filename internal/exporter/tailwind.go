package exporter

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/palette"
)

// renderTailwind emits a JavaScript snippet ready to drop into
// tailwind.config.js under theme.extend.colors.
//
// Two shapes:
//   - flat (opts.TailwindShades == 0): one entry per input color
//     "brand-1": "#aabbcc", "brand-2": "#ddeeff", …
//   - shade-scale (opts.TailwindShades > 0): each input expands into a nested
//     scale of monochromatic variants keyed by Tailwind shade numbers
//     "brand-1": { "100": …, "500": …, "900": … }
//
// Returns an error only when shade-scale generation fails (invalid base
// color); flat mode is infallible.
func renderTailwind(p *palette.Palette, opts Options) ([]byte, error) {
	if opts.TailwindShades > 0 {
		return renderTailwindShades(p.Colors, opts.Prefix, TailwindShadeStops(opts.TailwindShades))
	}
	var buf bytes.Buffer
	buf.WriteString("// huetension palette — paste into tailwind.config.js theme.extend.colors\n")
	buf.WriteString("module.exports = {\n")
	for i, c := range p.Colors {
		fmt.Fprintf(&buf, "  %q: %q,\n", fmt.Sprintf("%s-%d", opts.Prefix, i+1), c.Hex())
	}
	buf.WriteString("};\n")
	return buf.Bytes(), nil
}

// TailwindShadeStops returns the numeric shade labels for a given count.
// 5 and 10 use the canonical Tailwind scales; any other count falls back to
// a linear 100..N00 spread.
func TailwindShadeStops(count int) []int {
	switch count {
	case 5:
		return []int{100, 300, 500, 700, 900}
	case 10:
		return []int{50, 100, 200, 300, 400, 500, 600, 700, 800, 900}
	}
	stops := make([]int, count)
	for i := range count {
		stops[i] = (i + 1) * 100
	}
	return stops
}

// renderTailwindShades walks each input color, generates len(stops)
// monochromatic variants, sorts them lightest→darkest to match the Tailwind
// convention (50 = lightest, 900 = darkest), and emits a nested-shape
// module.exports object.
func renderTailwindShades(colors []color.Color, prefix string, stops []int) ([]byte, error) {
	count := len(stops)
	type group struct {
		name   string
		shades map[int]string
	}
	groups := make([]group, len(colors))

	for i, base := range colors {
		variants, err := harmony.Generate(harmony.Monochromatic, base, harmony.Options{Count: count})
		if err != nil {
			return nil, fmt.Errorf("color %d: %w", i+1, err)
		}
		sort.SliceStable(variants, func(a, b int) bool {
			return variants[a].Lightness() > variants[b].Lightness()
		})
		shadeMap := make(map[int]string, count)
		for j, v := range variants {
			shadeMap[stops[j]] = v.Hex()
		}
		groups[i] = group{
			name:   fmt.Sprintf("%s-%d", prefix, i+1),
			shades: shadeMap,
		}
	}

	var buf bytes.Buffer
	buf.WriteString("module.exports = {\n")
	for _, g := range groups {
		fmt.Fprintf(&buf, "  %q: {\n", g.name)
		keys := make([]int, 0, len(g.shades))
		for k := range g.shades {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			fmt.Fprintf(&buf, "    %q: %q,\n", strconv.Itoa(k), g.shades[k])
		}
		buf.WriteString("  },\n")
	}
	buf.WriteString("};\n")
	return buf.Bytes(), nil
}
