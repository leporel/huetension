package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/exporter"
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

	shadeCount, err := parseShadeCount(tf.shades)
	if err != nil {
		return err
	}

	p := palette.New(colors)
	p.Name = tf.name
	data, err := exporter.Export(p, exporter.FormatTailwind, exporter.Options{
		Prefix:         tf.name,
		Name:           tf.name,
		TailwindShades: shadeCount,
	})
	if err != nil {
		return err
	}
	return writeOutput(tf.output, data)
}

// parseShadeCount maps the --shades user input to the integer count consumed
// by exporter.Options.TailwindShades. "none" or "" → 0 (flat); "auto" → 10;
// any positive integer → itself.
func parseShadeCount(mode string) (int, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "", "none":
		return 0, nil
	case "auto":
		return 10, nil
	}
	n, err := strconv.Atoi(mode)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid --shades %q (want none|auto|5|10|N)", mode)
	}
	return n, nil
}
