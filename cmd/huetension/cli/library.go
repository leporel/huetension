package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leporel/huetension/internal/palette/library"
)

// newLibraryCmd builds the `library` command group. Today it holds one
// subcommand — `add` — which appends a palette to the on-disk catalogue
// (library.json inside the data directory).
func newLibraryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Manage the palette library",
		Long: "Grow the palette library — the curated catalogue plus your own saved palettes, " +
			"kept in library.json inside the data directory and shared by the CLI, the MCP server, " +
			"and the web UI.",
	}
	cmd.AddCommand(newLibraryAddCmd())
	return cmd
}

// newLibraryAddCmd builds `huetension library add`, which saves a
// palette to library.json. The same library.Store path the web and MCP
// save surfaces use, so an id generated here follows the identical
// slug + collision rule.
func newLibraryAddCmd() *cobra.Command {
	var (
		name        string
		categories  []string
		tags        []string
		description string
	)
	cmd := &cobra.Command{
		Use:   "add [color...]",
		Short: "Save a palette to the library",
		Long: "Save a palette to the on-disk library (library.json in the data directory), " +
			"alongside the curated defaults. Colors come from the positional arguments, or " +
			"one-per-line from stdin when none are given. The id is generated from the name; " +
			"every saved palette is filed under the \"" + library.SavedCategory + "\" category.",
		RunE: func(cmd *cobra.Command, args []string) error {
			colors, err := collectColorInputs(args)
			if err != nil {
				return err
			}
			idx, path, err := loadLibrary(dataDir)
			if err != nil {
				return err
			}
			if path == "" {
				return fmt.Errorf("library: no data directory resolved — pass --data-dir to choose where library.json lives")
			}
			saved, err := library.NewStore(idx, path).Save(library.SaveInput{
				Name:        name,
				Description: description,
				Colors:      colors,
				Categories:  categories,
				Tags:        tags,
			})
			if err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(),
					"saved %q to the library as %q (%d colors)\n  %s\n",
					saved.Name, saved.ID, len(saved.Colors), path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name for the palette (required)")
	cmd.Flags().StringArrayVar(&categories, "category", nil,
		"extra category to file the palette under (repeatable; \""+library.SavedCategory+"\" is always added)")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated free-form tags")
	cmd.Flags().StringVar(&description, "description", "", "one-line description")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}
