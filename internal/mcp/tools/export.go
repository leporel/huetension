package tools

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/palette"
)

// ExportResult is the wire shape for export.* tools. Format echoes the
// rendered dialect so a downstream consumer can save with the right
// extension; Filename is a suggested name (caller-side hint, no IO).
type ExportResult struct {
	Format   string `json:"format" jsonschema:"rendered format identifier (css|scss|less|tailwind)"`
	Kind     string `json:"kind,omitempty" jsonschema:"sub-dialect (vars|scss|less for export.css)"`
	Content  string `json:"content" jsonschema:"the rendered text"`
	Filename string `json:"filename" jsonschema:"suggested filename based on the format"`
}

// ExportCSSParams is the typed input for export.css.
type ExportCSSParams struct {
	Colors []string `json:"colors" jsonschema:"colors to render"`
	Name   string   `json:"name,omitempty" jsonschema:"variable name prefix (default: color)"`
	Kind   string   `json:"kind,omitempty" jsonschema:"dialect: vars (CSS custom properties, default) | scss | less"`
}

// ExportCSSOutput wraps an export.css result in the huetension/v1 envelope.
type ExportCSSOutput struct {
	Schema string          `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string          `json:"tool" jsonschema:"the tool that produced this result"`
	Params ExportCSSParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result ExportResult    `json:"result"`
}

// cssKindToFormat maps the --kind user value to an exporter.Format. Mirrors
// cmd/huetension/cli/css.go's cssKindToFormat (kept in sync deliberately;
// CLI and MCP must accept the same dialect aliases).
func cssKindToFormat(kind string) (exporter.Format, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "vars", "var", "css":
		return exporter.FormatCSS, nil
	case "scss", "sass":
		return exporter.FormatSCSS, nil
	case "less":
		return exporter.FormatLESS, nil
	}
	return "", fmt.Errorf("unknown kind %q (want vars|scss|less)", kind)
}

func handleExportCSS(_ context.Context, _ *sdk.CallToolRequest, p ExportCSSParams) (*sdk.CallToolResult, ExportCSSOutput, error) {
	if len(p.Colors) == 0 {
		return nil, ExportCSSOutput{}, fmt.Errorf("export.css: no colors provided")
	}
	cs, err := parseColors(p.Colors)
	if err != nil {
		return nil, ExportCSSOutput{}, err
	}
	format, err := cssKindToFormat(p.Kind)
	if err != nil {
		return nil, ExportCSSOutput{}, err
	}
	name := p.Name
	if strings.TrimSpace(name) == "" {
		name = "color"
	}

	pal := palette.New(cs)
	pal.Name = name
	data, err := exporter.Export(pal, format, exporter.Options{
		Prefix: name,
		Name:   name,
	})
	if err != nil {
		return nil, ExportCSSOutput{}, err
	}

	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	if kind == "" {
		kind = "vars"
	}

	return nil, ExportCSSOutput{
		Schema: schemaVersion,
		Tool:   "export.css",
		Params: ExportCSSParams{Colors: p.Colors, Name: name, Kind: kind},
		Result: ExportResult{
			Format:   string(format),
			Kind:     kind,
			Content:  string(data),
			Filename: fmt.Sprintf("%s.%s", name, exporter.FileExtension(format)),
		},
	}, nil
}

// ExportTailwindParams is the typed input for export.tailwind. Shades=0
// produces the flat shape (one entry per input color); Shades>0 expands each
// input into a monochromatic shade scale: 5 → 100/300/500/700/900, 10 →
// 50/100..900, any other N → linear 100..N00.
type ExportTailwindParams struct {
	Colors []string `json:"colors" jsonschema:"colors to render"`
	Name   string   `json:"name,omitempty" jsonschema:"color group name prefix (default: color)"`
	Shades int      `json:"shades,omitempty" jsonschema:"shade-scale count: 0 (default) flat shape; 5 → 100/300/500/700/900; 10 → 50/100..900; N → linear 100..N00"`
}

// ExportTailwindOutput wraps an export.tailwind result in the huetension/v1
// envelope.
type ExportTailwindOutput struct {
	Schema string               `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string               `json:"tool" jsonschema:"the tool that produced this result"`
	Params ExportTailwindParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result ExportResult         `json:"result"`
}

func handleExportTailwind(_ context.Context, _ *sdk.CallToolRequest, p ExportTailwindParams) (*sdk.CallToolResult, ExportTailwindOutput, error) {
	if len(p.Colors) == 0 {
		return nil, ExportTailwindOutput{}, fmt.Errorf("export.tailwind: no colors provided")
	}
	cs, err := parseColors(p.Colors)
	if err != nil {
		return nil, ExportTailwindOutput{}, err
	}
	name := p.Name
	if strings.TrimSpace(name) == "" {
		name = "color"
	}

	if p.Shades < 0 {
		return nil, ExportTailwindOutput{}, fmt.Errorf("export.tailwind: shades must be ≥ 0 (got %d)", p.Shades)
	}

	pal := palette.New(cs)
	pal.Name = name
	data, err := exporter.Export(pal, exporter.FormatTailwind, exporter.Options{
		Prefix:         name,
		Name:           name,
		TailwindShades: p.Shades,
	})
	if err != nil {
		return nil, ExportTailwindOutput{}, err
	}

	return nil, ExportTailwindOutput{
		Schema: schemaVersion,
		Tool:   "export.tailwind",
		Params: ExportTailwindParams{Colors: p.Colors, Name: name, Shades: p.Shades},
		Result: ExportResult{
			Format:   string(exporter.FormatTailwind),
			Content:  string(data),
			Filename: name + "." + exporter.FileExtension(exporter.FormatTailwind),
		},
	}, nil
}

// RegisterExportCSS installs the export.css tool on srv.
func RegisterExportCSS(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "export.css",
		Description: "Render a list of colors as CSS custom properties (default), SCSS variables, or LESS variables.",
	}, handleExportCSS)
}

// RegisterExportTailwind installs the export.tailwind tool on srv.
func RegisterExportTailwind(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "export.tailwind",
		Description: "Render a list of colors as a Tailwind theme.extend.colors snippet (flat shape, one entry per input).",
	}, handleExportTailwind)
}
