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
	format     string
	method     string
	saturation bool
	size       string
	output     string

	// K-NN method.
	radius       float64
	distribution float64
	intensity    float64
	blend        int

	// RBF method.
	reach     float64
	sharpness float64
	strength  float64

	// Grade method.
	compression float64
	mute        float64
}

func newLutCmd() *cobra.Command {
	var lf lutFlags

	cmd := &cobra.Command{
		Use:   "lut <color> [color2 ...]",
		Short: "Generate a 3D color-grading LUT from a palette",
		Long: "Generate a 3D lookup table (LUT) that pulls colours toward a palette in OkLab. " +
			"The LUT can be exported as a Cube file (.cube, dependable input for ffmpeg's lut3d filter, " +
			"DaVinci Resolve, etc.) or as a 2D LUT texture PNG (a visualisation of the cube — not the " +
			"standard HALD layout despite the similar look). " +
			"With no positional args, reads one color per line from stdin.\n\n" +
			"Use --format to select output (cube|png, default cube). " +
			"Pick the algorithm with --method (grade|rbf|knn, default grade). The grade path squeezes the " +
			"hue wheel onto the palette's hues like a vectorscope compression — lightness is never touched, " +
			"--compression sets how hard hues snap to the palette and --mute desaturates hues that fall " +
			"between palette colours. The smooth RBF path blends every " +
			"palette colour through a Gaussian-like kernel in OkLab a/b — controlled by --reach " +
			"(kernel σ), --sharpness (kernel exponent, p=2 is Gaussian, higher = closer to nearest-only), " +
			"and --strength (pull factor 0..1). The legacy K-NN path takes --radius, --distribution, " +
			"--intensity, and --blend. " +
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
	cmd.Flags().StringVar(&lf.method, "method", "grade", "algorithm: grade (hue-wheel compression, lightness preserved) | rbf (smooth Gaussian over all palette colours) | knn (legacy K-nearest)")
	cmd.Flags().BoolVar(&lf.saturation, "saturation", false, "also shift chroma (default: hue only — chroma frozen)")
	cmd.Flags().StringVar(&lf.size, "size", "", "cube edge: integer or preset (obs|ffmpeg|standard|small|medium|large). Default: 33 cube, 64 png")
	cmd.Flags().StringVarP(&lf.output, "output", "o", "-", "output file; use \"-\" for stdout")

	// Grade knobs (default method).
	cmd.Flags().Float64Var(&lf.compression, "compression", 0.70, "grade: how hard hues are squeezed onto the palette (0..1; 0 = identity)")
	cmd.Flags().Float64Var(&lf.mute, "mute", 0.30, "grade: desaturate hues that sit between palette colours (0..1)")

	// RBF knobs.
	cmd.Flags().Float64Var(&lf.reach, "reach", 0.20, "RBF: kernel σ in OkLab — how far each palette colour reaches (>0)")
	cmd.Flags().Float64Var(&lf.sharpness, "sharpness", 2.0, "RBF: kernel exponent p in exp(-(d/σ)^p); 2=Gaussian, higher=sharper")
	cmd.Flags().Float64Var(&lf.strength, "strength", 0.90, "RBF: pull factor (0..1)")

	// Legacy K-NN knobs.
	cmd.Flags().Float64Var(&lf.radius, "radius", 0.40, "K-NN: OkLab distance pull zone (0..∞)")
	cmd.Flags().Float64Var(&lf.distribution, "distribution", 0.15, "K-NN: falloff shape (0..1; 0.5=linear, <0.5=edges, >0.5=center)")
	cmd.Flags().Float64Var(&lf.intensity, "intensity", 0.90, "K-NN: pull strength (0..1)")
	cmd.Flags().IntVar(&lf.blend, "blend", 2, "K-NN: number of nearest colours to blend (≥1)")

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

	method := strings.ToLower(strings.TrimSpace(lf.method))
	switch method {
	case "", lut.MethodGrade:
		method = lut.MethodGrade
	case lut.MethodRBF, lut.MethodKNN:
	default:
		return fmt.Errorf("unknown --method %q (want grade|rbf|knn)", lf.method)
	}

	lutSize, err := resolveLutSizeFlag(lf.size, format)
	if err != nil {
		return err
	}

	opts := lut.Options{
		Size:              lutSize,
		Method:            method,
		IncludeSaturation: lf.saturation,
		Radius:            lf.radius,
		Distribution:      lf.distribution,
		Intensity:         lf.intensity,
		BlendNeighbors:    lf.blend,
		Reach:             lf.reach,
		Sharpness:         lf.sharpness,
		Strength:          lf.strength,
		Compression:       lf.compression,
		Mute:              lf.mute,
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
