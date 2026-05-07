package tools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/contrast"
)

// ContrastCheckParams is the typed input for contrast.check.
type ContrastCheckParams struct {
	FG   string `json:"fg" jsonschema:"foreground color (text)"`
	BG   string `json:"bg" jsonschema:"background color"`
	Algo string `json:"algo,omitempty" jsonschema:"algorithm: wcag21 (default) | apca | both"`
}

// ContrastCheckResult is the result block for contrast.check. WCAG21 is
// always present when requested algo is wcag21 or both; APCA likewise.
type ContrastCheckResult struct {
	WCAG21 *contrast.WCAG21Result `json:"wcag21,omitempty"`
	APCA   *contrast.APCAResult   `json:"apca,omitempty"`
}

// ContrastCheckOutput wraps the contrast scores in the huetension/v1
// envelope. The result is tool-specific (not palette-shaped).
type ContrastCheckOutput struct {
	Schema string              `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string              `json:"tool" jsonschema:"the tool that produced this result"`
	Params ContrastCheckParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result ContrastCheckResult `json:"result"`
}

func handleContrastCheck(_ context.Context, _ *sdk.CallToolRequest, p ContrastCheckParams) (*sdk.CallToolResult, ContrastCheckOutput, error) {
	fg, err := color.Parse(p.FG)
	if err != nil {
		return nil, ContrastCheckOutput{}, fmt.Errorf("fg: %w", err)
	}
	bg, err := color.Parse(p.BG)
	if err != nil {
		return nil, ContrastCheckOutput{}, fmt.Errorf("bg: %w", err)
	}
	algo := strings.ToLower(strings.TrimSpace(p.Algo))
	if algo == "" {
		algo = string(contrast.AlgoWCAG21)
	}

	res := ContrastCheckResult{}
	switch algo {
	case string(contrast.AlgoWCAG21):
		w := contrast.WCAG21(fg, bg)
		res.WCAG21 = &w
	case string(contrast.AlgoAPCA):
		a := contrast.APCA(fg, bg)
		res.APCA = &a
	case "both":
		w := contrast.WCAG21(fg, bg)
		a := contrast.APCA(fg, bg)
		res.WCAG21 = &w
		res.APCA = &a
	default:
		return nil, ContrastCheckOutput{}, fmt.Errorf("unknown algo %q (want wcag21|apca|both)", p.Algo)
	}

	return nil, ContrastCheckOutput{
		Schema: schemaVersion,
		Tool:   "contrast.check",
		Params: ContrastCheckParams{FG: p.FG, BG: p.BG, Algo: algo},
		Result: res,
	}, nil
}

// RegisterContrastCheck installs the contrast.check tool on srv.
func RegisterContrastCheck(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "contrast.check",
		Description: "Compute foreground/background contrast using WCAG 2.1 (luminance ratio with AA/AAA flags), APCA (perceptual Lc), or both.",
	}, handleContrastCheck)
}
