// Package tools holds the per-feature MCP tool implementations. Every file
// in this package wires one or more huetension internal libraries (color,
// palette, harmony, …) up as an MCP tool — input schema, handler, structured
// output type — and exposes a Register* function that the parent mcp package
// installs onto an *sdk.Server.
package tools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/color"
)

// schemaVersion is the wire-contract identifier emitted in the structured
// output envelope. It mirrors the constant used by the exporter package so
// CLI / library / MCP all key off the same string.
const schemaVersion = "huetension/v1"

// ColorConvertParams is the typed input for the color.convert tool. Fields
// drive both JSON-schema generation (jsonschema tags) and unmarshalling on
// the SDK side.
type ColorConvertParams struct {
	Color string `json:"color" jsonschema:"color to convert (e.g. #3366cc, rgb(51,102,204), hsl(225,60%,50%), royalblue)"`
	To    string `json:"to,omitempty" jsonschema:"target format: hex|rgb|hsl|hsv|hls|lab|lch|oklab|oklch|name|int|all (default: all)"`
}

// ColorConvertResult is the inner result block of a color.convert response.
// Either Formats (target=all) or Value (single format) is populated.
type ColorConvertResult struct {
	Input   string            `json:"input" jsonschema:"the color string the caller supplied"`
	Hex     string            `json:"hex" jsonschema:"canonical hex representation"`
	Formats map[string]string `json:"formats,omitempty" jsonschema:"all known representations, populated when to=all"`
	Value   string            `json:"value,omitempty" jsonschema:"the single requested format, populated when to is not 'all'"`
}

// ColorConvertOutput is the full structured envelope returned to the
// caller. The shape mirrors the huetension/v1 wire contract — schema, tool,
// params, result — so MCP clients see the same JSON the CLI emits in JSON
// mode.
type ColorConvertOutput struct {
	Schema string             `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string             `json:"tool" jsonschema:"the tool that produced this result"`
	Params ColorConvertParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result ColorConvertResult `json:"result"`
}

// handleColorConvert is the typed ToolHandlerFor implementation. The SDK
// derives input / output JSON schemas from the struct tags, validates the
// caller's arguments against the input schema before invoking us, and packs
// the returned struct into the CallToolResult's StructuredContent.
func handleColorConvert(_ context.Context, _ *sdk.CallToolRequest, p ColorConvertParams) (*sdk.CallToolResult, ColorConvertOutput, error) {
	target := strings.ToLower(strings.TrimSpace(p.To))
	if target == "" {
		target = "all"
	}
	c, err := color.Parse(p.Color)
	if err != nil {
		return nil, ColorConvertOutput{}, fmt.Errorf("color: %w", err)
	}

	res := ColorConvertResult{
		Input: p.Color,
		Hex:   c.Hex(),
	}
	if target == "all" {
		pairs := c.FormatAll()
		res.Formats = make(map[string]string, len(pairs))
		for _, pair := range pairs {
			res.Formats[string(pair.Format)] = pair.Value
		}
	} else {
		v, err := c.Format(color.Format(target))
		if err != nil {
			return nil, ColorConvertOutput{}, err
		}
		res.Value = v
	}

	return nil, ColorConvertOutput{
		Schema: schemaVersion,
		Tool:   "color.convert",
		Params: ColorConvertParams{Color: p.Color, To: target},
		Result: res,
	}, nil
}

// RegisterColorConvert installs the color.convert tool on srv.
func RegisterColorConvert(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "color.convert",
		Description: "Convert a color between hex, rgb, hsl, hsv, hls, lab, lch, oklab, oklch, CSS named, and integer formats.",
	}, handleColorConvert)
}
