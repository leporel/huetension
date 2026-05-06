package cli

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/palette"
)

type tailwindFlags struct {
	name   string
	shades string
	output string
}

func newTailwindCmd() *cobra.Command {
	var tf tailwindFlags

	cmd := &cobra.Command{
		Use:   "tailwind <color> [color2 ...]",
		Short: "Render explicit colors as a Tailwind theme.extend.colors snippet",
		Long: "Render the given colors as a Tailwind module.exports object suitable for pasting into a " +
			"Tailwind config's `theme.extend.colors`. With --shades each input is expanded into a shade scale " +
			"(50/100..900 by Tailwind convention) using a monochromatic harmony around the base lightness.\n\n" +
			"  --shades none → flat shape: {\"brand-1\": \"#3366cc\", ...} (default)\n" +
			"  --shades 5    → five shades: {\"brand-1\": {\"100\": ..., \"300\": ..., \"500\": ..., \"700\": ..., \"900\": ...}}\n" +
			"  --shades 10   → ten shades: 50/100/200/300/.../900\n" +
			"  --shades auto → same as 10\n" +
			"  --shades N    → N shades labelled 100..N00 (linear spread)\n\n" +
			"Reads colors from stdin when no args are given (one per line).",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTailwind(args, &tf)
		},
	}

	cmd.Flags().StringVar(&tf.name, "name", "color", "color group name prefix")
	cmd.Flags().StringVar(&tf.shades, "shades", "none", "shade expansion (none|auto|5|10|N)")
	cmd.Flags().StringVarP(&tf.output, "output", "o", "-", "output file path; use \"-\" for stdout")

	return cmd
}

func runTailwind(args []string, tf *tailwindFlags) error {
	inputs, err := collectColorInputs(args)
	if err != nil {
		return err
	}
	colors := make([]color.Color, 0, len(inputs))
	for i, raw := range inputs {
		c, err := color.Parse(raw)
		if err != nil {
			return fmt.Errorf("color %d (%q): %w", i+1, raw, err)
		}
		colors = append(colors, c)
	}

	mode := strings.ToLower(strings.TrimSpace(tf.shades))
	if mode == "" || mode == "none" {
		// Flat shape — defer to the existing exporter path.
		p := palette.New(colors)
		p.Name = tf.name
		data, err := exporter.Export(p, exporter.FormatTailwind, exporter.Options{
			Prefix: tf.name,
			Name:   tf.name,
		})
		if err != nil {
			return err
		}
		return writeOutput(tf.output, data)
	}

	count, err := parseShadeCount(mode)
	if err != nil {
		return err
	}
	stops := tailwindShadeStops(count)

	data, err := renderTailwindWithShades(colors, tf.name, stops)
	if err != nil {
		return err
	}
	return writeOutput(tf.output, data)
}

// parseShadeCount maps the --shades user input to a positive integer count.
// "auto" maps to 10 (matching Tailwind's standard 50/100..900 scale plus 950).
func parseShadeCount(mode string) (int, error) {
	if mode == "auto" {
		return 10, nil
	}
	n, err := strconv.Atoi(mode)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid --shades %q (want none|auto|5|10|N)", mode)
	}
	return n, nil
}

// tailwindShadeStops returns the numeric labels for a given count. We
// hard-code the canonical scales for 5 and 10 because those are the values
// designers actually expect; any other count falls back to a linear 100..N00
// spread which is unambiguous if not idiomatic.
func tailwindShadeStops(count int) []int {
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

// renderTailwindWithShades walks each input, generates `len(stops)`
// monochromatic variants around the input's lightness, and emits the
// Tailwind nested-shape JS object.
func renderTailwindWithShades(colors []color.Color, prefix string, stops []int) ([]byte, error) {
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
		// Monochromatic returns light→dark via lightness sweep around base.
		// Sort by lightness so 50 = lightest, 900 = darkest, mirroring the
		// Tailwind convention that 950 should be the darkest of the lot.
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
		// Stable iteration order for deterministic output.
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
