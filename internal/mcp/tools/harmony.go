package tools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/palette"
)

// HarmonyGenerateParams is the typed input for harmony.generate.
type HarmonyGenerateParams struct {
	Type  string  `json:"type" jsonschema:"harmony type: complementary|analogous|triadic|split-complementary|tetradic|square|double-complementary|compound|monochromatic|shades"`
	Base  string  `json:"base" jsonschema:"base color (hex, CSS named, rgb()/hsl()/oklch())"`
	Count int     `json:"count,omitempty" jsonschema:"palette size; for hue harmonies the extras cycle anchors with HSV tint/shade variations"`
	Step  float64 `json:"step,omitempty" jsonschema:"analogous-only: angular step between neighbours in degrees (default 30)"`
}

// HarmonyGenerateOutput wraps the generated harmony palette in the
// huetension/v1 envelope.
type HarmonyGenerateOutput struct {
	Schema string                `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string                `json:"tool" jsonschema:"the tool that produced this result"`
	Params HarmonyGenerateParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result PaletteResult         `json:"result"`
}

func handleHarmonyGenerate(_ context.Context, _ *sdk.CallToolRequest, p HarmonyGenerateParams) (*sdk.CallToolResult, HarmonyGenerateOutput, error) {
	if strings.TrimSpace(p.Type) == "" {
		return nil, HarmonyGenerateOutput{}, fmt.Errorf("harmony.generate: type is required")
	}
	if strings.TrimSpace(p.Base) == "" {
		return nil, HarmonyGenerateOutput{}, fmt.Errorf("harmony.generate: base is required")
	}
	t, err := resolveHarmonyType(p.Type)
	if err != nil {
		return nil, HarmonyGenerateOutput{}, err
	}
	base, err := color.Parse(p.Base)
	if err != nil {
		return nil, HarmonyGenerateOutput{}, fmt.Errorf("base color: %w", err)
	}
	colors, err := harmony.Generate(t, base, harmony.Options{Count: p.Count, Step: p.Step})
	if err != nil {
		return nil, HarmonyGenerateOutput{}, err
	}
	pal := palette.New(colors)
	pal.Name = string(t)
	pal.Metadata.Method = "harmony"
	pal.Metadata.Source = "harmony:" + string(t)
	pal.Metadata.Params = map[string]any{
		"type":  string(t),
		"base":  base.Hex(),
		"count": p.Count,
		"step":  p.Step,
	}
	return nil, HarmonyGenerateOutput{
		Schema: schemaVersion,
		Tool:   "harmony.generate",
		Params: HarmonyGenerateParams{Type: string(t), Base: p.Base, Count: p.Count, Step: p.Step},
		Result: exporter.EncodeResult(pal),
	}, nil
}

// resolveHarmonyType matches user-supplied input against the canonical
// harmony.Type set with the same aliases the CLI accepts ("split", "double").
func resolveHarmonyType(raw string) (harmony.Type, error) {
	low := strings.ToLower(strings.TrimSpace(raw))
	switch low {
	case "split":
		return harmony.Split, nil
	case "double":
		return harmony.DoubleComplementary, nil
	}
	known := []harmony.Type{
		harmony.Complementary,
		harmony.Analogous,
		harmony.Triadic,
		harmony.Split,
		harmony.Tetradic,
		harmony.Square,
		harmony.DoubleComplementary,
		harmony.Compound,
		harmony.Monochromatic,
		harmony.Shades,
	}
	for _, t := range known {
		if string(t) == low {
			return t, nil
		}
	}
	names := make([]string, len(known))
	for i, t := range known {
		names[i] = string(t)
	}
	return "", fmt.Errorf("unknown harmony type %q (known: %s)", raw, strings.Join(names, ", "))
}

// RegisterHarmonyGenerate installs the harmony.generate tool on srv.
func RegisterHarmonyGenerate(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "harmony.generate",
		Description: "Generate a color harmony (complementary, analogous, triadic, split, tetradic/square, double-complementary, compound, monochromatic, shades) around a base color.",
	}, handleHarmonyGenerate)
}
