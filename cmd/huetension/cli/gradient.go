package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/gradient"
	"github.com/leporel/huetension/internal/palette"
)

type gradientFlags struct {
	steps   int
	space   string
	easing  string
	sortBy  string
	reverse bool
}

func newGradientCmd() *cobra.Command {
	var gf gradientFlags
	var of outputFlags

	cmd := &cobra.Command{
		Use:   "gradient <color1> <color2> [color3 ...]",
		Short: "Generate a gradient palette between two or more colors",
		Long: "Build a gradient between two or more stops. With exactly 2 stops the gradient runs " +
			"end-to-end; with 3+ stops they are evenly spaced. Default interpolation space is OkLab — " +
			"perceptually uniform, no muddy mid-tones.\n\n" +
			"Stops accept hex / CSS named colors / rgb()-hsl() function notation. The default --steps is 5.",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGradient(args, &gf, &of)
		},
	}

	addOutputFlags(cmd, &of, "text")
	cmd.Flags().IntVarP(&gf.steps, "steps", "s", 5, "number of colors in the gradient (must be >= number of stops)")
	cmd.Flags().StringVar(&gf.space, "space", string(gradient.SpaceOkLab), "interpolation space (rgb|lab|oklab|oklch|hsl)")
	cmd.Flags().StringVar(&gf.easing, "easing", string(gradient.EasingLinear), "easing curve (linear|ease-in|ease-out|ease-in-out)")
	cmd.Flags().StringVar(&gf.sortBy, "sort", "", "sort palette by (luminance|lightness|okl|hue|saturation|frequency)")
	cmd.Flags().BoolVar(&gf.reverse, "reverse", false, "reverse the sort order")

	return cmd
}

func runGradient(rawStops []string, gf *gradientFlags, of *outputFlags) error {
	stops := make([]color.Color, len(rawStops))
	for i, s := range rawStops {
		c, err := color.Parse(s)
		if err != nil {
			return fmt.Errorf("stop %d (%q): %w", i+1, s, err)
		}
		stops[i] = c
	}

	opts := gradient.Options{
		Steps:  gf.steps,
		Space:  gradient.Space(strings.ToLower(gf.space)),
		Easing: gradient.Easing(strings.ToLower(gf.easing)),
	}

	var colors []color.Color
	var err error
	if len(stops) == 2 {
		colors, err = gradient.Build(stops[0], stops[1], opts)
	} else {
		colors, err = gradient.MultiStop(stops, opts)
	}
	if err != nil {
		return err
	}

	p := palette.New(colors)
	p.Name = "gradient"
	p.Metadata.Method = "gradient"
	p.Metadata.Source = "gradient"
	stopHex := make([]string, len(stops))
	for i, c := range stops {
		stopHex[i] = c.Hex()
	}
	p.Metadata.Params = map[string]any{
		"stops":  stopHex,
		"steps":  gf.steps,
		"space":  string(opts.Space),
		"easing": string(opts.Easing),
	}

	if gf.sortBy != "" {
		if err := p.Sort(palette.SortBy(gf.sortBy), gf.reverse); err != nil {
			return err
		}
	}

	of.tool = "gradient"
	of.toolParams = map[string]any{
		"stops":  stopHex,
		"steps":  gf.steps,
		"space":  string(opts.Space),
		"easing": string(opts.Easing),
	}
	of.headerVerb = "Built"
	of.headerSource = "gradient (" + strings.Join(stopHex, " → ") + ")"
	return renderAndWrite(p, of)
}
