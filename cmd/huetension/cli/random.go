package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/palette"
)

type randomFlags struct {
	count    int
	seed     uint64
	harmony  string
	sortBy   string
	reverse  bool
}

func newRandomCmd() *cobra.Command {
	var rf randomFlags
	var of outputFlags

	cmd := &cobra.Command{
		Use:   "random",
		Short: "Generate a random palette",
		Long: "Generate a random palette by spreading hues evenly around the wheel and randomising saturation/" +
			"lightness within designer-friendly ranges. Pass --seed for reproducible output (useful in tests).\n\n" +
			"With --harmony, a single random base color is picked and the remaining colors come from the chosen " +
			"harmony rule (complementary / analogous / triadic / …) — handy for spec-quality palettes that still " +
			"have an algorithmic flavour.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRandom(&rf, &of)
		},
	}

	addOutputFlags(cmd, &of, "text")
	cmd.Flags().IntVarP(&rf.count, "count", "n", 5, "number of colors in the palette")
	cmd.Flags().Uint64Var(&rf.seed, "seed", 0, "PRNG seed for deterministic output (0 = random)")
	cmd.Flags().StringVar(&rf.harmony, "harmony", "none", "harmony around a random base (none|complementary|analogous|triadic|split-complementary|tetradic|square|double-complementary|monochromatic|shades)")
	cmd.Flags().StringVar(&rf.sortBy, "sort", "", "sort palette by (luminance|lightness|okl|hue|saturation|frequency)")
	cmd.Flags().BoolVar(&rf.reverse, "reverse", false, "reverse the sort order")

	return cmd
}

func runRandom(rf *randomFlags, of *outputFlags) error {
	htype := strings.ToLower(strings.TrimSpace(rf.harmony))

	var p *palette.Palette
	if htype == "" || htype == "none" {
		p = palette.Random(palette.RandomOptions{
			Count: rf.count,
			Seed:  rf.seed,
		})
		p.Name = "random"
		p.Metadata.Method = "random"
		p.Metadata.Source = "random"
		p.Metadata.Params = map[string]any{
			"count": rf.count,
			"seed":  rf.seed,
		}
	} else {
		// Pick a random base color (count=1, same seed) so the output is
		// fully reproducible from --seed alone.
		seed := palette.Random(palette.RandomOptions{Count: 1, Seed: rf.seed})
		base := seed.Colors[0]

		t, err := resolveHarmonyType(htype)
		if err != nil {
			return err
		}
		colors, err := harmony.Generate(t, base, harmony.Options{Count: rf.count})
		if err != nil {
			return err
		}
		p = palette.New(colors)
		p.Name = "random-" + string(t)
		p.Metadata.Method = "random+harmony"
		p.Metadata.Source = "random:" + string(t)
		p.Metadata.Params = map[string]any{
			"count":   rf.count,
			"seed":    rf.seed,
			"harmony": string(t),
			"base":    base.Hex(),
		}
	}

	if rf.sortBy != "" {
		if err := p.Sort(palette.SortBy(rf.sortBy), rf.reverse); err != nil {
			return err
		}
	}

	if p.Len() == 0 {
		return fmt.Errorf("random: produced empty palette")
	}

	of.tool = "random"
	of.toolParams = map[string]any{
		"count":   rf.count,
		"seed":    rf.seed,
		"harmony": htype,
	}
	of.headerVerb = "Generated"
	if htype == "" || htype == "none" {
		of.headerSource = "random palette"
	} else {
		of.headerSource = "random " + htype + " palette"
	}
	return renderAndWrite(p, of)
}
