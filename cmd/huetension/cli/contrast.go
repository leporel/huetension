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
	algo   string
	output string
	pretty bool
}

func newContrastCmd() *cobra.Command {
	var cf contrastFlags

	cmd := &cobra.Command{
		Use:   "contrast <foreground> <background>",
		Short: "Compute contrast score between two colors",
		Long: "Compute the contrast score between a foreground and a background color using either WCAG 2.1 " +
			"(luminance ratio, 1..21) or APCA (Lc, signed perceptual score). Use --algo to switch.\n\n" +
			"Output is always JSON. Use -o / --output to write the result to a file instead of stdout.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContrast(args[0], args[1], &cf)
		},
	}

	cmd.Flags().StringVarP(&cf.algo, "algo", "a", string(contrast.AlgoWCAG21), "contrast algorithm (wcag21|apca)")
	cmd.Flags().StringVarP(&cf.output, "output", "o", "-", "output file path; use \"-\" for stdout")
	cmd.Flags().BoolVar(&cf.pretty, "pretty", true, "pretty-print JSON output")

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

	var data []byte
	if cf.pretty {
		data, err = json.MarshalIndent(result, "", "  ")
	} else {
		data, err = json.Marshal(result)
	}
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeOutput(cf.output, data)
}
