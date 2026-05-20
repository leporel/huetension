package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/analyze"
	"github.com/leporel/huetension/internal/imageio"
)

type analyzeFlags struct {
	strip          string
	distanceTarget string
	space          string
	width          int
	height         int
	output         string
	resize         int
	timeout        time.Duration
	allowedHosts   []string
	maxBytes       int64
}

func newAnalyzeCmd() *cobra.Command {
	var af analyzeFlags

	cmd := &cobra.Command{
		Use:   "analyze <source>",
		Short: "Render colour-distribution strips for an image",
		Long: "Render four colour-distribution strips for an image — pixels sorted by hue, luminance, saturation, " +
			"and Euclidean RGB distance to a chosen primary. <source> may be a file path, an http/https URL, " +
			"a data: URI, or \"-\" for stdin.\n\n" +
			"Use --strip to pick one metric instead of all four. With --strip all (the default) the strips are " +
			"stacked vertically into a single PNG (Width × Height×4); with a single metric the output is a " +
			"plain Width × Height PNG.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(args[0], &af)
		},
	}

	cmd.Flags().StringVar(&af.strip, "strip", "all", "metric to render (hue|luminance|saturation|distance|all)")
	cmd.Flags().StringVar(&af.distanceTarget, "distance-target", string(analyze.DefaultDistanceTarget), "primary used by the distance strip (red|green|blue)")
	cmd.Flags().StringVar(&af.space, "space", string(analyze.DefaultSpace), "colour space for hue/luminance/saturation sort keys (oklch|hsl); distance is unaffected")
	cmd.Flags().IntVar(&af.width, "width", analyze.DefaultWidth, "strip width in pixels")
	cmd.Flags().IntVar(&af.height, "height", analyze.DefaultHeight, "strip height in pixels (per metric when --strip all)")
	cmd.Flags().StringVarP(&af.output, "output", "o", "-", "output file path; use \"-\" for stdout")
	cmd.Flags().IntVar(&af.resize, "resize", analyze.DefaultResize, "resize longest image side before analysis (0 = default, negative = no resize)")
	cmd.Flags().DurationVar(&af.timeout, "timeout", 30*time.Second, "HTTP timeout for URL sources")
	cmd.Flags().StringSliceVar(&af.allowedHosts, "allow-host", nil, "restrict URL sources to one of the given host suffixes (repeatable)")
	cmd.Flags().Int64Var(&af.maxBytes, "max-bytes", 0, "maximum payload size for URL/file sources in bytes (0 = 64 MiB default)")

	return cmd
}

func runAnalyze(source string, af *analyzeFlags) error {
	stripSel := strings.ToLower(strings.TrimSpace(af.strip))
	var single analyze.Metric
	all := stripSel == "all" || stripSel == ""
	if !all {
		m, ok := analyze.ParseMetric(stripSel)
		if !ok {
			return fmt.Errorf("--strip: unknown metric %q (want hue|luminance|saturation|distance|all)", af.strip)
		}
		single = m
	}

	target, ok := analyze.ParseDistanceTarget(strings.ToLower(strings.TrimSpace(af.distanceTarget)))
	if !ok {
		return fmt.Errorf("--distance-target: unknown primary %q (want red|green|blue)", af.distanceTarget)
	}

	space, ok := analyze.ParseSpace(strings.ToLower(strings.TrimSpace(af.space)))
	if !ok {
		return fmt.Errorf("--space: unknown colour space %q (want oklch|hsl)", af.space)
	}

	if af.width <= 0 {
		return fmt.Errorf("--width must be > 0")
	}
	if af.height <= 0 {
		return fmt.Errorf("--height must be > 0")
	}

	loaded, err := imageio.Load(source, imageio.LoadOptions{
		MaxBytes:     af.maxBytes,
		Timeout:      af.timeout,
		AllowedHosts: af.allowedHosts,
	})
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}

	result, err := analyze.Strips(loaded.Image, analyze.Options{
		Width:          af.width,
		Height:         af.height,
		Resize:         af.resize,
		DistanceTarget: target,
		Space:          space,
	})
	if err != nil {
		return err
	}

	if all {
		data, err := analyze.Combined(result.Strips)
		if err != nil {
			return err
		}
		return writeOutput(af.output, data)
	}

	for _, s := range result.Strips {
		if s.Metric == single {
			return writeOutput(af.output, s.PNG)
		}
	}
	return fmt.Errorf("analyze: strip %q not found in result (internal bug)", single)
}
