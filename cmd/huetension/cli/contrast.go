package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/contrast"
)

type contrastFlags struct {
	algo    string
	output  string
	pretty  bool
	suggest bool
	target  float64
}

func newContrastCmd() *cobra.Command {
	var cf contrastFlags

	cmd := &cobra.Command{
		Use:   "contrast <foreground> <background>",
		Short: "Compute contrast score between two colors",
		Long: "Compute the contrast score between a foreground and a background color using either WCAG 2.1 " +
			"(luminance ratio, 1..21) or APCA (Lc, signed perceptual score). Use --algo to switch.\n\n" +
			"Pass --suggest to also search the foreground's OkLCH lightness sweep for the nearest passing " +
			"value (chroma and hue held fixed). The threshold comes from --target; defaults are 4.5 for " +
			"wcag21 (AA body text) and 60 for apca (Content). The suggest output wraps the score in " +
			"{ \"score\": ..., \"suggest\": ... } — without --suggest the original flat score shape is preserved.\n\n" +
			"Output is always JSON. Use -o / --output to write the result to a file instead of stdout.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContrast(args[0], args[1], &cf)
		},
	}

	cmd.Flags().StringVarP(&cf.algo, "algo", "a", string(contrast.AlgoWCAG21), "contrast algorithm (wcag21|apca)")
	cmd.Flags().StringVarP(&cf.output, "output", "o", "-", "output file path; use \"-\" for stdout")
	cmd.Flags().BoolVar(&cf.pretty, "pretty", true, "pretty-print JSON output")
	cmd.Flags().BoolVar(&cf.suggest, "suggest", false, "also search for the nearest passing OkLCH lightness for the foreground")
	cmd.Flags().Float64Var(&cf.target, "target", 0, "contrast target for --suggest (defaults: 4.5 for wcag21, 60 for apca)")

	return cmd
}

func runContrast(rawFG, rawBG string, cf *contrastFlags) error {
	fg, err := color.Parse(rawFG)
	if err != nil {
		return fmt.Errorf("foreground: %w", err)
	}
	bg, err := color.Parse(rawBG)
	if err != nil {
		return fmt.Errorf("background: %w", err)
	}

	algo := contrast.Algo(strings.ToLower(strings.TrimSpace(cf.algo)))
	result, err := contrast.Check(fg, bg, algo)
	if err != nil {
		return err
	}

	var payload any = result
	if cf.suggest {
		target, err := defaultSuggestTarget(algo, cf.target)
		if err != nil {
			return err
		}
		suggestion, err := contrast.Suggest(fg, bg, algo, target)
		if err != nil {
			return err
		}
		payload = struct {
			Score   any                    `json:"score"`
			Suggest contrast.SuggestResult `json:"suggest"`
		}{Score: result, Suggest: suggestion}
	}

	var data []byte
	if cf.pretty {
		data, err = json.MarshalIndent(payload, "", "  ")
	} else {
		data, err = json.Marshal(payload)
	}
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeOutput(cf.output, data)
}

// defaultSuggestTarget picks a sensible WCAG / APCA target when --target
// is left at 0 (the default), so casual users can pass --suggest without
// having to memorise the per-algo scales. An explicit non-zero --target
// is passed through unchanged.
func defaultSuggestTarget(algo contrast.Algo, target float64) (float64, error) {
	if target > 0 {
		return target, nil
	}
	switch algo {
	case "", contrast.AlgoWCAG21:
		return 4.5, nil
	case contrast.AlgoAPCA:
		return 60, nil
	}
	return 0, fmt.Errorf("contrast: cannot pick a default --target for algo %q", string(algo))
}
