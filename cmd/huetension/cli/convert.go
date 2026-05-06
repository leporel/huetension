package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/color"
)

type convertFlags struct {
	to     string // target format, or "all"
	mode   string // "text" or "json"
	output string
	pretty bool
}

// convertEntry is one row of convert's output. Either Formats (when --to=all)
// or Value (single format) is populated; Error is non-empty when the input
// could not be parsed.
type convertEntry struct {
	Input   string            `json:"input"`
	Hex     string            `json:"hex,omitempty"`
	Formats map[string]string `json:"formats,omitempty"`
	Value   string            `json:"value,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func newConvertCmd() *cobra.Command {
	var cf convertFlags

	cmd := &cobra.Command{
		Use:   "convert <color> [color2 ...]",
		Short: "Convert one or more colors between formats",
		Long: "Parse each <color> and emit it in one or all known formats. Accepts hex, CSS named colors, " +
			"rgb()/hsl()/hsv()/lab()/lch()/oklab()/oklch() function notation. With no positional args, " +
			"reads one color per line from stdin — useful for piping (`cat colors.txt | huetension convert --to lab`).\n\n" +
			"Default --to is `all` (every format). Default output is text; use --format json for a structured " +
			"document suitable for downstream tooling.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConvert(args, &cf)
		},
	}

	cmd.Flags().StringVar(&cf.to, "to", "all", "target format (hex|rgb|hsl|hsv|hls|lab|lch|oklab|oklch|name|int|all)")
	cmd.Flags().StringVarP(&cf.mode, "format", "f", "text", "output mode (text|json)")
	cmd.Flags().StringVarP(&cf.output, "output", "o", "-", "output file path; use \"-\" for stdout")
	cmd.Flags().BoolVar(&cf.pretty, "pretty", true, "pretty-print JSON output")

	return cmd
}

func runConvert(args []string, cf *convertFlags) error {
	inputs, err := collectColorInputs(args)
	if err != nil {
		return err
	}

	target := strings.ToLower(strings.TrimSpace(cf.to))
	results := make([]convertEntry, 0, len(inputs))
	hadError := false

	for _, raw := range inputs {
		c, err := color.Parse(raw)
		if err != nil {
			hadError = true
			results = append(results, convertEntry{Input: raw, Error: err.Error()})
			continue
		}
		e := convertEntry{Input: raw, Hex: c.Hex()}
		if target == "all" {
			pairs := c.FormatAll()
			e.Formats = make(map[string]string, len(pairs))
			for _, p := range pairs {
				e.Formats[string(p.Format)] = p.Value
			}
		} else {
			v, err := c.Format(color.Format(target))
			if err != nil {
				return err
			}
			e.Value = v
		}
		results = append(results, e)
	}

	var out []byte
	switch strings.ToLower(strings.TrimSpace(cf.mode)) {
	case "", "text":
		out = []byte(renderConvertText(results, target))
	case "json":
		var marshalErr error
		if cf.pretty {
			out, marshalErr = json.MarshalIndent(results, "", "  ")
		} else {
			out, marshalErr = json.Marshal(results)
		}
		if marshalErr != nil {
			return marshalErr
		}
		out = append(out, '\n')
	default:
		return fmt.Errorf("unknown --format %q (want text|json)", cf.mode)
	}

	if err := writeOutput(cf.output, out); err != nil {
		return err
	}
	if hadError {
		return errPartialFailure
	}
	return nil
}

// renderConvertText formats convert's results as terminal-friendly text.
// Single-format mode prints one value per line; "all" mode prints the
// input header followed by an indented block of `format: value` lines.
func renderConvertText(results []convertEntry, target string) string {
	var b strings.Builder
	for i, r := range results {
		if r.Error != "" {
			fmt.Fprintf(&b, "%s: error: %s\n", r.Input, r.Error)
			continue
		}
		if target == "all" {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%s\n", r.Input)
			for _, f := range color.AllFormats {
				if v, ok := r.Formats[string(f)]; ok {
					fmt.Fprintf(&b, "  %-7s %s\n", string(f)+":", v)
				}
			}
			continue
		}
		fmt.Fprintln(&b, r.Value)
	}
	return b.String()
}
