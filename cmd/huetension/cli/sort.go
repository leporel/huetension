package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

type sortFlags struct {
	by      string
	reverse bool
	mode    string // "text" or "json"
	output  string
	pretty  bool
}

func newSortCmd() *cobra.Command {
	var sf sortFlags

	cmd := &cobra.Command{
		Use:   "sort [color1 color2 ...]",
		Short: "Sort colors by luminance / hue / saturation / lightness / OkLab L / frequency",
		Long: "Sort a list of colors by the chosen key, ascending by default. Pass --reverse to flip. " +
			"With no positional args, reads one color per line from stdin so it composes naturally with " +
			"`cat colors.txt | huetension sort --by hue`.\n\n" +
			"Default sort key is luminance (WCAG relative luminance — dark to light).",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSort(args, &sf)
		},
	}

	cmd.Flags().StringVarP(&sf.by, "by", "b", string(palette.SortByLuminance), "sort key (luminance|lightness|okl|hue|saturation|frequency)")
	cmd.Flags().BoolVarP(&sf.reverse, "reverse", "r", false, "reverse the sort order")
	cmd.Flags().StringVarP(&sf.mode, "format", "f", "text", "output mode (text|json)")
	cmd.Flags().StringVarP(&sf.output, "output", "o", "-", "output file path; use \"-\" for stdout")
	cmd.Flags().BoolVar(&sf.pretty, "pretty", true, "pretty-print JSON output")

	return cmd
}

func runSort(args []string, sf *sortFlags) error {
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

	sorted, err := palette.SortColors(colors, palette.SortBy(strings.ToLower(strings.TrimSpace(sf.by))), sf.reverse)
	if err != nil {
		return err
	}

	var out []byte
	switch strings.ToLower(strings.TrimSpace(sf.mode)) {
	case "", "text":
		var b strings.Builder
		for _, c := range sorted {
			fmt.Fprintln(&b, c.Hex())
		}
		out = []byte(b.String())
	case "json":
		hexes := make([]string, len(sorted))
		for i, c := range sorted {
			hexes[i] = c.Hex()
		}
		var marshalErr error
		if sf.pretty {
			out, marshalErr = json.MarshalIndent(hexes, "", "  ")
		} else {
			out, marshalErr = json.Marshal(hexes)
		}
		if marshalErr != nil {
			return marshalErr
		}
		out = append(out, '\n')
	default:
		return fmt.Errorf("unknown --format %q (want text|json)", sf.mode)
	}

	return writeOutput(sf.output, out)
}
