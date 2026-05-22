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
	FG      string  `json:"fg" jsonschema:"foreground color (text)"`
	BG      string  `json:"bg" jsonschema:"background color"`
	Algo    string  `json:"algo,omitempty" jsonschema:"algorithm: wcag21 (default) | apca | both"`
	Suggest bool    `json:"suggest,omitempty" jsonschema:"also search the foreground's OkLCH lightness sweep for the nearest passing value; chroma and hue are held fixed. Single-algo only — not compatible with algo=both."`
	Target  float64 `json:"target,omitempty" jsonschema:"contrast target for the suggest search (WCAG ratio or |APCA Lc|). Defaults: 4.5 for wcag21, 60 for apca."`
}

// ContrastCheckResult is the result block for contrast.check. WCAG21 is
// always present when requested algo is wcag21 or both; APCA likewise.
// Suggest is present only when the caller asked for a lightness-fix
// search and a single algo was selected.
type ContrastCheckResult struct {
	WCAG21  *contrast.WCAG21Result  `json:"wcag21,omitempty"`
	APCA    *contrast.APCAResult    `json:"apca,omitempty"`
	Suggest *contrast.SuggestResult `json:"suggest,omitempty"`
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

	if p.Suggest {
		if algo == "both" {
			return nil, ContrastCheckOutput{}, fmt.Errorf("suggest=true requires a single algo (wcag21|apca), got %q", algo)
		}
		target, err := defaultSuggestTarget(contrast.Algo(algo), p.Target)
		if err != nil {
			return nil, ContrastCheckOutput{}, err
		}
		s, err := contrast.Suggest(fg, bg, contrast.Algo(algo), target)
		if err != nil {
			return nil, ContrastCheckOutput{}, err
		}
		res.Suggest = &s
	}

	return nil, ContrastCheckOutput{
		Schema: schemaVersion,
		Tool:   "contrast.check",
		Params: ContrastCheckParams{FG: p.FG, BG: p.BG, Algo: algo, Suggest: p.Suggest, Target: p.Target},
		Result: res,
	}, nil
}

// defaultSuggestTarget mirrors the CLI helper — pick a WCAG / APCA
// threshold when the caller leaves Target at 0 (the typed default), so
// MCP clients can flip suggest on without first looking up the per-algo
// scale.
func defaultSuggestTarget(algo contrast.Algo, target float64) (float64, error) {
	if target > 0 {
		return target, nil
	}
	switch algo {
	case "", contrast.AlgoWCAG21:
		return 4.5, nil
	case contrast.AlgoAPCA:
		return 60, nil
	}
	return 0, fmt.Errorf("cannot pick a default target for algo %q", string(algo))
}

// RegisterContrastCheck installs the contrast.check tool on srv.
func RegisterContrastCheck(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "contrast.check",
		Description: "Compute foreground/background contrast using WCAG 2.1 (luminance ratio with AA/AAA flags), APCA (perceptual Lc), or both. Pass suggest=true (single-algo only) to also receive the nearest passing OkLCH lightness for the foreground — chroma and hue are held fixed.",
	}, handleContrastCheck)
}
