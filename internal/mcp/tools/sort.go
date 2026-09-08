package tools

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/palette"
)

// ColorSortParams is the typed input for color.sort.
type ColorSortParams struct {
	Colors  []string `json:"colors" jsonschema:"colors to sort (hex, rgb(), hsl(), CSS named, etc.)"`
	By      string   `json:"by,omitempty" jsonschema:"sort key: luminance|lightness|okl|hue|saturation|frequency (default: luminance)"`
	Reverse bool     `json:"reverse,omitempty" jsonschema:"reverse the sort order (e.g. light→dark for luminance)"`
}

// ColorSortOutput wraps a sorted palette in the huetension/v1 envelope.
type ColorSortOutput struct {
	Schema string          `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string          `json:"tool" jsonschema:"the tool that produced this result"`
	Params ColorSortParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result PaletteResult   `json:"result"`
}

func handleColorSort(_ context.Context, _ *sdk.CallToolRequest, p ColorSortParams) (*sdk.CallToolResult, ColorSortOutput, error) {
	if len(p.Colors) == 0 {
		return nil, ColorSortOutput{}, fmt.Errorf("color.sort: no colors provided")
	}
	cs, err := parseColors(p.Colors)
	if err != nil {
		return nil, ColorSortOutput{}, err
	}
	by := palette.SortBy(p.By)
	if by == "" {
		by = palette.SortByLuminance
	}
	pal := palette.New(cs)
	if err := pal.Sort(by, p.Reverse); err != nil {
		return nil, ColorSortOutput{}, err
	}
	return nil, ColorSortOutput{
		Schema: schemaVersion,
		Tool:   "color.sort",
		Params: ColorSortParams{Colors: p.Colors, By: string(by), Reverse: p.Reverse},
		Result: exporter.EncodeResult(pal),
	}, nil
}

// RegisterColorSort installs the color.sort tool on srv.
func RegisterColorSort(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "color.sort",
		Description: "Sort a list of colors by luminance, lightness, OkLab L, hue, saturation, or frequency. Returns the sorted palette.",
	}, handleColorSort)
}
