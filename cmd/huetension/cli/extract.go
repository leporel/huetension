package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/extract"
	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/palette"
)

// extractFlags carries everything `extract` needs that isn't shared with
// other commands. Fields embedded into command-local locals so the cobra
// callbacks can read them after parsing.
type extractFlags struct {
	method        string
	size          int
	resize        int
	alphaMask     uint8
	minSat        float64
	minLight      float64
	maxLight      float64
	mergeEpsilon  float64
	sortBy        string
	reverse       bool
	timeout       time.Duration
	allowedHosts  []string
	maxBytes      int64
}

func newExtractCmd() *cobra.Command {
	var ef extractFlags
	var of outputFlags

	cmd := &cobra.Command{
		Use:   "extract <source>",
		Short: "Extract a palette from an image",
		Long: "Pull a palette from an image. <source> may be a file path, an http/https URL, " +
			"a data: URI, or \"-\" for stdin. The default algorithm (soft) is tuned for designer-friendly " +
			"output; pass --method to switch.\n\n" +
			"Use --output FILE (-o) to write the palette to a file; otherwise the result is printed to stdout.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtract(cmd.Context(), args[0], &ef, &of)
		},
	}

	addOutputFlags(cmd, &of, "text")

	cmd.Flags().StringVarP(&ef.method, "method", "m", string(extract.MethodSoft), methodFlagUsage())
	cmd.Flags().IntVarP(&ef.size, "size", "k", 5, "palette size (number of colors)")
	cmd.Flags().IntVarP(&ef.resize, "resize", "r", 512, "resize longest image side before extracting (0 = no resize)")
	cmd.Flags().Uint8Var(&ef.alphaMask, "alpha-mask", 0, "drop pixels with alpha strictly below this value (0..255)")
	cmd.Flags().Float64Var(&ef.minSat, "min-saturation", 0, "soft/softk only: drop pixels with HSL saturation below this value")
	cmd.Flags().Float64Var(&ef.minLight, "min-lightness", 0, "soft/softk only: drop pixels darker than this lightness")
	cmd.Flags().Float64Var(&ef.maxLight, "max-lightness", 0, "soft/softk only: drop pixels brighter than this lightness")
	cmd.Flags().Float64Var(&ef.mergeEpsilon, "merge-epsilon", 0, "soft only: ΔE76 threshold below which clusters are merged (0 = default)")
	cmd.Flags().StringVar(&ef.sortBy, "sort", "", "sort palette by (luminance|lightness|okl|hue|saturation|frequency)")
	cmd.Flags().BoolVar(&ef.reverse, "reverse", false, "reverse the sort order")
	cmd.Flags().DurationVar(&ef.timeout, "timeout", 30*time.Second, "HTTP timeout for URL sources")
	cmd.Flags().StringSliceVar(&ef.allowedHosts, "allow-host", nil, "restrict URL sources to one of the given host suffixes (repeatable)")
	cmd.Flags().Int64Var(&ef.maxBytes, "max-bytes", 0, "maximum payload size for URL/file sources in bytes (0 = 64 MiB default)")

	return cmd
}

func runExtract(ctx context.Context, source string, ef *extractFlags, of *outputFlags) error {
	if ctx == nil {
		ctx = context.Background()
	}

	opts := extract.Options{
		Method:             extract.Method(ef.method),
		PaletteSize:        ef.size,
		Resize:             ef.resize,
		AlphaMaskThreshold: ef.alphaMask,
		MinSaturation:      ef.minSat,
		MinLightness:       ef.minLight,
		MaxLightness:       ef.maxLight,
		MergeEpsilon:       ef.mergeEpsilon,
		SortBy:             palette.SortBy(ef.sortBy),
		Reverse:            ef.reverse,
	}
	ioOpts := imageio.LoadOptions{
		MaxBytes:     ef.maxBytes,
		Timeout:      ef.timeout,
		AllowedHosts: ef.allowedHosts,
	}

	p, err := extract.FromSource(ctx, source, opts, ioOpts)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	// Populate JSON envelope identity. The metadata.params already
	// captures the algorithmic params; here we record what the user
	// invoked the CLI with — they overlap heavily but the CLI form
	// includes flags that don't make it into the algorithm (output path,
	// sort key, etc.).
	of.tool = "extract"
	of.toolParams = map[string]any{
		"source": source,
		"method": ef.method,
		"size":   ef.size,
		"resize": ef.resize,
		"sort":   ef.sortBy,
	}
	of.headerVerb = "Extracted"
	of.headerSource = source
	return renderAndWrite(p, of)
}

// methodFlagUsage builds the human-readable usage string for the --method
// flag, listing every supported algorithm.
func methodFlagUsage() string {
	parts := make([]string, len(extract.AllMethods))
	for i, m := range extract.AllMethods {
		parts[i] = string(m)
	}
	return "extraction method (" + strings.Join(parts, "|") + ")"
}
