package tools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/gradient"
	"github.com/leporel/huetension/internal/palette"
)

// GradientGenerateParams is the typed input for gradient.generate.
//
// Two modes:
//   - From + To: simple two-stop gradient.
//   - Stops: ≥2 colors blended through with even spacing (overrides
//     From/To when set).
type GradientGenerateParams struct {
	From   string   `json:"from,omitempty" jsonschema:"start color (used with 'to' for a 2-stop gradient)"`
	To     string   `json:"to,omitempty" jsonschema:"end color (used with 'from' for a 2-stop gradient)"`
	Stops  []string `json:"stops,omitempty" jsonschema:"≥2 colors blended through with even spacing; overrides from/to when set"`
	Steps  int      `json:"steps" jsonschema:"number of colors in the output (≥2)"`
	Space  string   `json:"space,omitempty" jsonschema:"interpolation space: oklab|oklch|lab|rgb|hsl (default oklab)"`
	Easing string   `json:"easing,omitempty" jsonschema:"easing curve: linear|ease-in|ease-out|ease-in-out (default linear)"`
}

// GradientGenerateOutput wraps the generated gradient as a palette in the
// huetension/v1 envelope.
type GradientGenerateOutput struct {
	Schema string                 `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string                 `json:"tool" jsonschema:"the tool that produced this result"`
	Params GradientGenerateParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result PaletteResult          `json:"result"`
}

func handleGradientGenerate(_ context.Context, _ *sdk.CallToolRequest, p GradientGenerateParams) (*sdk.CallToolResult, GradientGenerateOutput, error) {
	opts := gradient.Options{
		Steps:  p.Steps,
		Space:  gradient.Space(strings.ToLower(strings.TrimSpace(p.Space))),
		Easing: gradient.Easing(strings.ToLower(strings.TrimSpace(p.Easing))),
	}

	var (
		colors []color.Color
		err    error
	)
	switch {
	case len(p.Stops) >= 2:
		stops, perr := parseColors(p.Stops)
		if perr != nil {
			return nil, GradientGenerateOutput{}, perr
		}
		colors, err = gradient.MultiStop(stops, opts)
	case p.From != "" && p.To != "":
		from, ferr := color.Parse(p.From)
		if ferr != nil {
			return nil, GradientGenerateOutput{}, fmt.Errorf("from color: %w", ferr)
		}
		to, terr := color.Parse(p.To)
		if terr != nil {
			return nil, GradientGenerateOutput{}, fmt.Errorf("to color: %w", terr)
		}
		colors, err = gradient.Build(from, to, opts)
	default:
		return nil, GradientGenerateOutput{}, fmt.Errorf("gradient.generate: provide either {from, to} or stops (≥2)")
	}
	if err != nil {
		return nil, GradientGenerateOutput{}, err
	}

	pal := palette.New(colors)
	pal.Name = "gradient"
	pal.Metadata.Method = "gradient"
	pal.Metadata.Params = map[string]any{
		"steps":  p.Steps,
		"space":  string(opts.Space),
		"easing": string(opts.Easing),
	}
	return nil, GradientGenerateOutput{
		Schema: schemaVersion,
		Tool:   "gradient.generate",
		Params: p,
		Result: exporter.EncodeResult(pal),
	}, nil
}

// RegisterGradientGenerate installs the gradient.generate tool on srv.
func RegisterGradientGenerate(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "gradient.generate",
		Description: "Build a color gradient between two endpoints, or through ≥2 stops, in OkLab/OkLCH/Lab/RGB/HSL space with optional easing.",
	}, handleGradientGenerate)
}
