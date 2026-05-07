package tools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/blindness"
)

// BlindnessSimulateParams is the typed input for blindness.simulate.
type BlindnessSimulateParams struct {
	Colors []string `json:"colors" jsonschema:"colors to simulate"`
	Kind   string   `json:"kind,omitempty" jsonschema:"deficiency: protan|deutan|tritan|achroma|all (default: all)"`
}

// BlindnessKindResult is one (kind, simulated colors) pair. Used inside
// BlindnessSimulateResult so the wire shape is a deterministic array
// rather than a map (avoids map iteration order surprises across MCP
// transport boundaries).
type BlindnessKindResult struct {
	Kind   string       `json:"kind" jsonschema:"deficiency identifier"`
	Colors []ColorEntry `json:"colors" jsonschema:"colors as perceived under this deficiency"`
}

// BlindnessSimulateResult is the result block for blindness.simulate.
type BlindnessSimulateResult struct {
	Variants []BlindnessKindResult `json:"variants" jsonschema:"one entry per requested deficiency"`
}

// BlindnessSimulateOutput wraps the simulation in the huetension/v1 envelope.
type BlindnessSimulateOutput struct {
	Schema string                  `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string                  `json:"tool" jsonschema:"the tool that produced this result"`
	Params BlindnessSimulateParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result BlindnessSimulateResult `json:"result"`
}

func handleBlindnessSimulate(_ context.Context, _ *sdk.CallToolRequest, p BlindnessSimulateParams) (*sdk.CallToolResult, BlindnessSimulateOutput, error) {
	if len(p.Colors) == 0 {
		return nil, BlindnessSimulateOutput{}, fmt.Errorf("blindness.simulate: no colors provided")
	}
	cs, err := parseColors(p.Colors)
	if err != nil {
		return nil, BlindnessSimulateOutput{}, err
	}

	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	if kind == "" {
		kind = "all"
	}

	var variants []BlindnessKindResult
	if kind == "all" {
		all, serr := blindness.SimulateAll(cs)
		if serr != nil {
			return nil, BlindnessSimulateOutput{}, serr
		}
		variants = make([]BlindnessKindResult, 0, len(blindness.AllKinds))
		for _, k := range blindness.AllKinds {
			variants = append(variants, BlindnessKindResult{
				Kind:   string(k),
				Colors: encodeColors(all[k]),
			})
		}
	} else {
		out, serr := blindness.SimulatePalette(cs, blindness.Kind(kind))
		if serr != nil {
			return nil, BlindnessSimulateOutput{}, serr
		}
		variants = []BlindnessKindResult{{
			Kind:   kind,
			Colors: encodeColors(out),
		}}
	}

	return nil, BlindnessSimulateOutput{
		Schema: schemaVersion,
		Tool:   "blindness.simulate",
		Params: BlindnessSimulateParams{Colors: p.Colors, Kind: kind},
		Result: BlindnessSimulateResult{Variants: variants},
	}, nil
}

// RegisterBlindnessSimulate installs the blindness.simulate tool on srv.
func RegisterBlindnessSimulate(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "blindness.simulate",
		Description: "Simulate how a list of colors is perceived under protanopia, deuteranopia, tritanopia, or achromatopsia (Brettel/Viénot matrices).",
	}, handleBlindnessSimulate)
}
