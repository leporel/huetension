package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/palette"
)

// harmonyFlags carries options that are specific to the `harmony` command —
// the rest (output format, file path, prefix, etc.) come from outputFlags.
type harmonyFlags struct {
	count   int
	step    float64
	sortBy  string
	reverse bool
}

// allHarmonyTypes is the canonical iteration order. Used to build the
// --type flag usage string and to validate user input.
var allHarmonyTypes = []harmony.Type{
	harmony.Complementary,
	harmony.Analogous,
	harmony.Triadic,
	harmony.Split,
	harmony.Tetradic,
	harmony.Square,
	harmony.DoubleComplementary,
	harmony.Monochromatic,
	harmony.Shades,
}

func newHarmonyCmd() *cobra.Command {
	var hf harmonyFlags
	var of outputFlags

	cmd := &cobra.Command{
		Use:   "harmony <type> <color>",
		Short: "Generate a color harmony around a base color",
		Long: "Generate a color harmony (complementary, analogous, triadic, …) around <color>. " +
			"<color> accepts hex (#3366cc, 0x3366cc, 3366cc), CSS named colors (royalblue), " +
			"or function notation (rgb(51,102,204) / hsl(225,60%,50%)).\n\n" +
			"The default output format is CSS — designer-oriented usage tends to dominate this command. " +
			"Use --output to write to a file (PNG/JPEG paths render a swatch image).",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHarmony(args[0], args[1], &hf, &of)
		},
	}

	addOutputFlags(cmd, &of, "text")
	cmd.Flags().IntVarP(&hf.count, "count", "c", 0, "palette size; for hue harmonies extra slots cycle anchors with HSV variations (Adobe Kuler style); defaults for analogous/monochromatic/shades: 3/5/5")
	cmd.Flags().Float64Var(&hf.step, "step", 0, "analogous only: angular step in degrees (default 30)")
	cmd.Flags().StringVar(&hf.sortBy, "sort", "", "sort palette by (luminance|lightness|okl|hue|saturation|frequency)")
	cmd.Flags().BoolVar(&hf.reverse, "reverse", false, "reverse the sort order")

	return cmd
}

func runHarmony(rawType, rawColor string, hf *harmonyFlags, of *outputFlags) error {
	t, err := resolveHarmonyType(rawType)
	if err != nil {
		return err
	}
	base, err := color.Parse(rawColor)
	if err != nil {
		return fmt.Errorf("base color: %w", err)
	}

	colors, err := harmony.Generate(t, base, harmony.Options{
		Count: hf.count,
		Step:  hf.step,
	})
	if err != nil {
		return err
	}

	p := palette.New(colors)
	p.Name = string(t)
	p.Metadata.Method = "harmony"
	p.Metadata.Source = "harmony:" + string(t)
	p.Metadata.Params = map[string]any{
		"type":  string(t),
		"base":  base.Hex(),
		"count": hf.count,
		"step":  hf.step,
	}

	if hf.sortBy != "" {
		if err := p.Sort(palette.SortBy(hf.sortBy), hf.reverse); err != nil {
			return err
		}
	}

	of.tool = "harmony"
	of.toolParams = map[string]any{
		"type":  string(t),
		"base":  base.Hex(),
		"count": hf.count,
		"step":  hf.step,
	}
	of.headerVerb = "Generated"
	of.headerSource = fmt.Sprintf("%s harmony of %s", t, base.Hex())
	return renderAndWrite(p, of)
}

// resolveHarmonyType matches user-supplied input against the canonical
// harmony.Type set. Match is case-insensitive; common aliases ("split",
// "double") map onto their full forms so the CLI feels forgiving.
func resolveHarmonyType(raw string) (harmony.Type, error) {
	low := strings.ToLower(strings.TrimSpace(raw))
	switch low {
	case "split":
		return harmony.Split, nil
	case "double":
		return harmony.DoubleComplementary, nil
	}
	for _, t := range allHarmonyTypes {
		if string(t) == low {
			return t, nil
		}
	}
	known := make([]string, len(allHarmonyTypes))
	for i, t := range allHarmonyTypes {
		known[i] = string(t)
	}
	return "", fmt.Errorf("unknown harmony type %q (known: %s)", raw, strings.Join(known, ", "))
}
