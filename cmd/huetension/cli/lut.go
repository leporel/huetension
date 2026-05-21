package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/lut"
	"github.com/leporel/huetension/internal/palette"
)

type lutFlags struct {
	format       string
	radius       float64
	distribution float64
	intensity    float64
	blend        int
	saturation   bool
	size         string
	output       string
}

func newLutCmd() *cobra.Command {
	var lf lutFlags

	cmd := &cobra.Command{
		Use:   "lut <color> [color2 ...]",
		Short: "Generate a 3D color-grading LUT from a palette",
		Long: "Generate a 3D lookup table (LUT) that pulls colours toward a palette in OkLCH colorspace. " +
			"The LUT can be exported as a Cube file (.cube, dependable input for ffmpeg's lut3d filter, " +
			"DaVinci Resolve, etc.) or as a 2D LUT texture PNG (a visualisation of the cube — not the " +
			"standard HALD layout despite the similar look). " +
			"With no positional args, reads one color per line from stdin.\n\n" +
			"Use --format to select output (cube|png, default cube). " +
			"Adjust the pull strength with --radius (OkLab distance zone), --distribution (falloff shape, 0..1), " +
			"and --intensity (pull strength, 0..1). " +
			"Use --blend N for K-NN weighted blending across multiple palette colours. " +
			"Pass --saturation to also shift chroma (hue-only by default).\n\n" +
			"For --format png, --size must be a perfect square (4, 9, 16, 25, 36, 49, 64, ...). " +
			"The resulting image side is size·√size pixels (e.g. size 16 → 64×64; size 64 → 512×512). " +
			"--size also accepts the `standard` preset (= cube edge 64, 512×512 texture).",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLUT(args, &lf)
		},
	}

	cmd.Flags().StringVar(&lf.format, "format", "cube", "output format (cube|png)")
	cmd.Flags().Float64Var(&lf.radius, "radius", 0.40, "OkLab distance pull zone (0..∞)")
	cmd.Flags().Float64Var(&lf.distribution, "distribution", 0.15, "falloff shape (0..1; 0.5=linear, <0.5=edges, >0.5=center)")
	cmd.Flags().Float64Var(&lf.intensity, "intensity", 0.90, "pull strength (0..1)")
	cmd.Flags().IntVar(&lf.blend, "blend", 2, "K-NN: number of nearest colours to blend (≥1)")
	cmd.Flags().BoolVar(&lf.saturation, "saturation", false, "also shift chroma (default: hue only — chroma frozen)")
	cmd.Flags().StringVar(&lf.size, "size", "", "cube edge: integer or preset (obs|ffmpeg|standard|small|medium|large). Default: 33 cube, 64 png")
	cmd.Flags().StringVarP(&lf.output, "output", "o", "-", "output file; use \"-\" for stdout")

	return cmd
}

func runLUT(args []string, lf *lutFlags) error {
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

	format := strings.ToLower(strings.TrimSpace(lf.format))
	if format != "cube" && format != "png" {
		return fmt.Errorf("unknown --format %q (want cube|png)", lf.format)
	}

	lutSize, err := resolveLutSizeFlag(lf.size, format)
	if err != nil {
		return err
	}

	opts := lut.Options{
		Size:              lutSize,
		Radius:            lf.radius,
		Distribution:      lf.distribution,
		Intensity:         lf.intensity,
		BlendNeighbors:    lf.blend,
		IncludeSaturation: lf.saturation,
	}

	p := palette.New(colors)
	generatedLUT, err := lut.Generate(p, opts)
	if err != nil {
		return fmt.Errorf("generate LUT: %w", err)
	}

	var data []byte
	switch format {
	case "cube":
		data = lut.EncodeCube(generatedLUT, "huetension")
	case "png":
		texturePNG, err := lut.EncodeHaldPNG(generatedLUT)
		if err != nil {
			return fmt.Errorf("encode LUT texture PNG: %w", err)
		}
		data = texturePNG
	}

	return writeOutput(lf.output, data)
}

// resolveLutSizeFlag mirrors the /lut handler's size parsing — accepts
// an integer, the `standard` preset (cube edge 64), or empty (format-
// specific default). Keeps CLI and Web behaviour in sync so
// `--size standard` and `{"size": "standard"}` resolve identically.
func resolveLutSizeFlag(raw, format string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if format == "png" {
			return 64, nil
		}
		return 33, nil
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n, nil
	}
	switch strings.ToLower(raw) {
	case "obs", "ffmpeg", "standard":
		return 64, nil
	}
	return 0, fmt.Errorf("--size: unknown preset %q (want an integer or obs|ffmpeg|standard)", raw)
}
