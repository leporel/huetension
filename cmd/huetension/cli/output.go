package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/cliutil"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/palette"
)

// stdoutWriter is the destination used when --output is "-" or unset.
// Tests swap it with a bytes.Buffer to capture command output without
// touching os.Stdout. Production code paths leave this at os.Stdout.
var stdoutWriter io.Writer = os.Stdout

// outputFlags collects the export-format / output-file / formatting options
// shared by every subcommand that emits a palette. addOutputFlags wires it
// onto a cobra command so the surface stays consistent.
type outputFlags struct {
	format     string
	defFormat  string // default --format value, set by addOutputFlags
	output     string
	pretty     bool
	prefix     string
	name       string
	swatchSize string // "WxH" e.g. "120x80"; empty → exporter defaults

	// tool and toolParams populate the JSON envelope's `tool` and `params`
	// fields when --format=json. Set by each command's RunE before calling
	// renderAndWrite so the wire output identifies which command produced
	// it (extract / harmony / gradient / ...).
	tool       string
	toolParams map[string]any

	// headerVerb and headerSource feed cliutil.RenderHeader for the
	// text-mode summary line (e.g. "Extracted 4 colors from photo.jpg").
	// Both unset → no header. Suppressed by the global --quiet flag.
	headerVerb   string
	headerSource string
}

// addOutputFlags registers the shared --format / --output / --pretty /
// --prefix / --name flags on cmd. defFormat is the format used when the
// user does not pass --format; palette-producing commands default to text
// (terminal-friendly ANSI swatches), machine-oriented commands default to
// json.
func addOutputFlags(cmd *cobra.Command, of *outputFlags, defFormat string) {
	of.defFormat = defFormat
	cmd.Flags().StringVarP(&of.format, "format", "f", defFormat, "output format (text|json|css|scss|less|tailwind|txt|gpl|png|jpeg)")
	cmd.Flags().StringVarP(&of.output, "output", "o", "-", "output file path; use \"-\" for stdout. PNG/JPEG extensions render a swatch image")
	cmd.Flags().BoolVar(&of.pretty, "pretty", false, "pretty-print JSON output")
	cmd.Flags().StringVar(&of.prefix, "prefix", "", "prefix for CSS/SCSS/Tailwind variable names (default \"color\")")
	cmd.Flags().StringVar(&of.name, "name", "", "palette name for JSON / GPL exports (default \"huetension\")")
	cmd.Flags().StringVar(&of.swatchSize, "swatch-size", "", "PNG/JPEG only: per-color swatch dimensions WxH (default 96x96)")
}

// renderAndWrite exports the palette using of's options and writes the
// bytes to the configured destination. If of.output is "" or "-" the
// output goes to stdout; otherwise it's written to the named file (parent
// directories are created if missing).
//
// The "text" mode bypasses the exporter and renders ANSI swatches via
// cliutil. Text is the default for palette-producing commands so
// `huetension extract photo.jpg` shows a terminal-friendly swatch table
// instead of dumping JSON the user must pipe to `jq`. All other modes
// delegate to the exporter package.
func renderAndWrite(p *palette.Palette, of *outputFlags) error {
	mode, format, err := resolveOutputMode(of)
	if err != nil {
		return err
	}
	if mode == modeText {
		return renderTextAndWrite(p, of)
	}

	w, h, err := parseSwatchSize(of.swatchSize)
	if err != nil {
		return err
	}
	data, err := exporter.Export(p, format, exporter.Options{
		Prefix:       of.prefix,
		Pretty:       of.pretty,
		Name:         of.name,
		SwatchWidth:  w,
		SwatchHeight: h,
		Tool:         of.tool,
		Params:       of.toolParams,
	})
	if err != nil {
		return err
	}
	return writeOutput(of.output, data)
}

const (
	modeText   = "text"
	modeExport = "export"
)

// resolveOutputMode decides between text rendering and exporter-driven
// output.
//
// Precedence:
//
//  1. Explicit --format text → text mode.
//  2. Explicit --format <known-export> → export mode with that format.
//  3. Explicit --format unknown → error.
//  4. --format unset (still at defFormat) AND -o has a known file
//     extension → export mode with the inferred format. This is what
//     makes `extract photo.jpg -o swatch.png` produce an image without a
//     redundant `--format png`.
//  5. Otherwise fall back to the command's defFormat (text for palette
//     commands today).
func resolveOutputMode(of *outputFlags) (string, exporter.Format, error) {
	raw := strings.ToLower(strings.TrimSpace(of.format))
	def := strings.ToLower(strings.TrimSpace(of.defFormat))
	userOverrode := raw != def

	if userOverrode {
		if raw == modeText {
			return modeText, "", nil
		}
		f, ok := exporter.FormatFromExtension(strings.TrimPrefix(raw, "."))
		if !ok {
			return "", "", unknownFormatError(of.format)
		}
		return modeExport, f, nil
	}

	if of.output != "" && of.output != "-" {
		ext := strings.ToLower(filepath.Ext(of.output))
		if f, ok := exporter.FormatFromExtension(ext); ok {
			return modeExport, f, nil
		}
	}

	if def == modeText {
		return modeText, "", nil
	}
	f, ok := exporter.FormatFromExtension(strings.TrimPrefix(def, "."))
	if !ok {
		return "", "", unknownFormatError(of.defFormat)
	}
	return modeExport, f, nil
}

func unknownFormatError(raw string) error {
	known := make([]string, 0, len(exporter.AllFormats)+1)
	known = append(known, modeText)
	for _, f := range exporter.AllFormats {
		known = append(known, string(f))
	}
	return fmt.Errorf("unknown format %q (known: %s)", raw, strings.Join(known, ", "))
}

// renderTextAndWrite renders p as ANSI text via cliutil and writes the
// result to of's destination. Honours the global --no-color and --quiet
// toggles; --quiet suppresses the per-command header line, leaving only
// the swatch table.
func renderTextAndWrite(p *palette.Palette, of *outputFlags) error {
	var b strings.Builder
	if !quiet && of.headerVerb != "" {
		b.WriteString(cliutil.RenderHeader(of.headerVerb, of.headerSource, p.Len()))
	}
	b.WriteString(cliutil.RenderPalette(p, cliutil.Options{NoColor: noColor}))
	return writeOutput(of.output, []byte(b.String()))
}

// parseSwatchSize parses a "WxH" string into width and height, returning
// (0, 0, nil) when the input is empty (callers fall back to exporter
// defaults). Both dimensions must be positive integers.
func parseSwatchSize(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(strings.ToLower(s), "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("swatch-size: want WxH, got %q", s)
	}
	var w, h int
	if _, err := fmt.Sscanf(parts[0], "%d", &w); err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("swatch-size: invalid width %q", parts[0])
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &h); err != nil || h <= 0 {
		return 0, 0, fmt.Errorf("swatch-size: invalid height %q", parts[1])
	}
	return w, h, nil
}

// writeOutput writes data to the path described by out. "-" / "" route to
// stdoutWriter (os.Stdout in production, a buffer in tests); any other
// value is treated as a filesystem path. Parent dirs are created on demand.
func writeOutput(out string, data []byte) error {
	if out == "" || out == "-" {
		_, err := io.Copy(stdoutWriter, bytes.NewReader(data))
		return err
	}
	if dir := filepath.Dir(out); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}
