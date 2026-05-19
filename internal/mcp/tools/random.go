package tools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/palette"
)

// PaletteRandomParams is the typed input for palette.random.
type PaletteRandomParams struct {
	Count   int    `json:"count,omitempty" jsonschema:"palette size (default 5, max 32)"`
	Seed    uint64 `json:"seed,omitempty" jsonschema:"PRNG seed for deterministic output (0 = wall clock)"`
	Harmony string `json:"harmony,omitempty" jsonschema:"optional harmony rule (none|complementary|analogous|triadic|split-complementary|tetradic|square|double-complementary|compound|monochromatic|shades); when set, a random base is picked and the palette comes from harmony.generate"`
}

// PaletteRandomOutput wraps a random palette in the huetension/v1 envelope.
type PaletteRandomOutput struct {
	Schema string              `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string              `json:"tool" jsonschema:"the tool that produced this result"`
	Params PaletteRandomParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result PaletteResult       `json:"result"`
}

func handlePaletteRandom(_ context.Context, _ *sdk.CallToolRequest, p PaletteRandomParams) (*sdk.CallToolResult, PaletteRandomOutput, error) {
	htype := strings.ToLower(strings.TrimSpace(p.Harmony))

	var pal *palette.Palette
	if htype == "" || htype == "none" {
		pal = palette.Random(palette.RandomOptions{
			Count: p.Count,
			Seed:  p.Seed,
		})
		pal.Name = "random"
	} else {
		// Pick a random base (count=1, same seed) so output is reproducible
		// from --seed alone, then generate a harmony around it.
		seed := palette.Random(palette.RandomOptions{Count: 1, Seed: p.Seed})
		base := seed.Colors[0]

		t, err := resolveHarmonyType(htype)
		if err != nil {
			return nil, PaletteRandomOutput{}, err
		}
		colors, err := harmony.Generate(t, base, harmony.Options{Count: p.Count})
		if err != nil {
			return nil, PaletteRandomOutput{}, err
		}
		pal = palette.New(colors)
		pal.Name = "random-" + string(t)
		pal.Metadata.Method = "random+harmony"
		pal.Metadata.Source = "random:" + string(t)
		pal.Metadata.Params = map[string]any{
			"count":   p.Count,
			"seed":    p.Seed,
			"harmony": string(t),
			"base":    base.Hex(),
		}
	}

	if pal.Len() == 0 {
		return nil, PaletteRandomOutput{}, fmt.Errorf("palette.random: produced empty palette")
	}

	return nil, PaletteRandomOutput{
		Schema: schemaVersion,
		Tool:   "palette.random",
		Params: PaletteRandomParams{Count: p.Count, Seed: p.Seed, Harmony: htype},
		Result: encodePalette(pal),
	}, nil
}

// RegisterPaletteRandom installs the palette.random tool on srv.
func RegisterPaletteRandom(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "palette.random",
		Description: "Generate a random palette by spreading hues evenly with bounded saturation/lightness. Optionally drive a harmony rule from a random base color (deterministic when --seed is provided).",
	}, handlePaletteRandom)
}
