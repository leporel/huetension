package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/blindness"
	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

type blindnessFlags struct {
	kind string // single CVD type, or "all" to simulate every kind
}

func newBlindnessCmd() *cobra.Command {
	var bf blindnessFlags
	var of outputFlags

	cmd := &cobra.Command{
		Use:   "blindness <color> [color2 ...]",
		Short: "Simulate how colors look under color-vision deficiency",
		Long: "Apply a Brettel-Viénot-Mollon channel-mixing matrix to one or more colors so the output " +
			"approximates how someone with the chosen CVD would perceive them. Single-kind mode (--kind protan / " +
			"deutan / tritan / achroma) returns a palette of simulated colors; --kind all emits a JSON object " +
			"keyed by kind so you can compare side-by-side.\n\n" +
			"Use -o to write the result to a file (PNG/JPEG paths render a swatch image — single-kind only).",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlindness(args, &bf, &of)
		},
	}

	addOutputFlags(cmd, &of, "text")
	cmd.Flags().StringVarP(&bf.kind, "kind", "k", string(blindness.Protan), "CVD type (protan|deutan|tritan|achroma|all)")

	return cmd
}

func runBlindness(rawColors []string, bf *blindnessFlags, of *outputFlags) error {
	in := make([]color.Color, len(rawColors))
	for i, s := range rawColors {
		c, err := color.Parse(s)
		if err != nil {
			return fmt.Errorf("color %d (%q): %w", i+1, s, err)
		}
		in[i] = c
	}

	kind := strings.ToLower(strings.TrimSpace(bf.kind))
	if kind == "all" {
		return runBlindnessAll(in, of)
	}

	out, err := blindness.SimulatePalette(in, blindness.Kind(kind))
	if err != nil {
		return err
	}
	p := palette.New(out)
	p.Name = "cvd-" + kind
	p.Metadata.Method = "blindness"
	p.Metadata.Source = "blindness:" + kind
	p.Metadata.Params = map[string]any{
		"kind":   kind,
		"inputs": rawColors,
	}

	of.tool = "blindness"
	of.toolParams = map[string]any{
		"kind":   kind,
		"inputs": rawColors,
	}
	of.headerVerb = "Simulated"
	of.headerSource = kind + " CVD"
	return renderAndWrite(p, of)
}

// runBlindnessAll handles --kind=all. The output is always JSON, since the
// other formats can't carry the per-kind grouping. We sidestep renderAndWrite
// here because exporter.Export operates on a single palette.
func runBlindnessAll(in []color.Color, of *outputFlags) error {
	all, err := blindness.SimulateAll(in)
	if err != nil {
		return err
	}
	wrapped := make(map[string][]map[string]string, len(all))
	for kind, colors := range all {
		entries := make([]map[string]string, len(colors))
		for i, c := range colors {
			entries[i] = map[string]string{"hex": c.Hex()}
		}
		wrapped[string(kind)] = entries
	}
	var data []byte
	if of.pretty {
		data, err = json.MarshalIndent(wrapped, "", "  ")
	} else {
		data, err = json.Marshal(wrapped)
	}
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeOutput(of.output, data)
}
