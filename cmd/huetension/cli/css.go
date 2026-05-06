package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/palette"
)

type cssFlags struct {
	kind   string
	name   string
	output string
}

func newCSSCmd() *cobra.Command {
	var cf cssFlags

	cmd := &cobra.Command{
		Use:   "css <color> [color2 ...]",
		Short: "Render explicit colors as CSS / SCSS / LESS variables",
		Long: "Render the given colors as a stylesheet variable block. Pick the dialect with --kind:\n" +
			"  vars  → CSS custom properties (`:root { --name-1: #...; }`) — default\n" +
			"  scss  → Sass variables (`$name-1: #...;`) plus a `$name-list` for iteration\n" +
			"  less  → LESS variables (`@name-1: #...;`) plus a `@name-list`\n\n" +
			"Use --name to set the variable prefix (default `color`). Reads colors from stdin when no args " +
			"are given (one per line).",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCSS(args, &cf)
		},
	}

	cmd.Flags().StringVar(&cf.kind, "kind", "vars", "stylesheet dialect (vars|scss|less)")
	cmd.Flags().StringVar(&cf.name, "name", "color", "variable name prefix")
	cmd.Flags().StringVarP(&cf.output, "output", "o", "-", "output file path; use \"-\" for stdout")

	return cmd
}

func runCSS(args []string, cf *cssFlags) error {
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

	format, err := cssKindToFormat(cf.kind)
	if err != nil {
		return err
	}

	p := palette.New(colors)
	p.Name = cf.name
	data, err := exporter.Export(p, format, exporter.Options{
		Prefix: cf.name,
		Name:   cf.name,
	})
	if err != nil {
		return err
	}
	return writeOutput(cf.output, data)
}

func cssKindToFormat(kind string) (exporter.Format, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "vars", "var", "css":
		return exporter.FormatCSS, nil
	case "scss", "sass":
		return exporter.FormatSCSS, nil
	case "less":
		return exporter.FormatLESS, nil
	}
	return "", fmt.Errorf("unknown --kind %q (want vars|scss|less)", kind)
}
