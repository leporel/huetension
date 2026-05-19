// Package cli wires the huetension command tree on top of cobra and viper.
//
// The binary itself (cmd/huetension/main.go) only constructs the root
// command and calls Execute. Each subcommand lives in its own file so the
// tree is easy to grow.
//
// Configuration precedence: explicit flag → env var (HUETENSION_*) → config
// file (config.yaml inside --data-dir, defaulting to the user config
// directory and creating it when missing) → built-in defaults. This is the
// usual viper layering and lets users keep host allowlists, default export
// formats, etc. in a single YAML file. The same data dir also holds
// library.json for user-added palettes; see data_dir.go.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const envPrefix = "HUETENSION"

// Config-file keys consumed via viper. config.yaml mirrors the flag tree
// under per-command sections; only the keys listed here are wired so far.
// Precedence is the usual viper layering — explicit flag > HUETENSION_*
// env > config.yaml > built-in default.
//
// The `mcp:` section's tool selectors are honoured by both `huetension
// mcp` and `huetension serve`: "which MCP tools to expose" is one setting
// regardless of which command hosts the server.
const (
	cfgKeyMCPEnable  = "mcp.enable"
	cfgKeyMCPDisable = "mcp.disable"
)

var (
	dataDir string
	version = "dev"
	// noColor is the global --no-color toggle. It's read by cliutil's
	// rendering helpers via the Options struct. Defaults to false (colors
	// on); flip on by passing --no-color or setting NO_COLOR=1.
	noColor bool
	// quiet suppresses the per-command header line in text mode (e.g.
	// "Extracted 4 colors from photo.jpg"). Useful when piping output to
	// other tools. JSON output is unaffected — the envelope already
	// carries the same context machine-readably.
	quiet bool
)

// Execute is the binary entry point. It parses os.Args, dispatches to the
// matching subcommand, and exits with a status code matching Pylette's
// convention:
//
//	0 — success
//	1 — total failure (could not run, fatal error, single-input failure)
//	2 — partial failure (some inputs processed, others failed) — signalled
//	    by a command returning errPartialFailure
func Execute(v string) {
	version = v
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		if errors.Is(err, errPartialFailure) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "huetension",
		Short:         "Color palette toolkit — extract, generate, harmonise, export",
		Long:          "huetension is a color palette toolkit. It extracts palettes from images, generates harmonies and gradients around a base color, computes contrast scores, simulates colour-vision deficiency, and exports the result to JSON / CSS / SCSS / Tailwind / plain hex / GIMP .gpl.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.PersistentFlags().StringVar(&dataDir, "data-dir", "", "data directory holding "+configFilename+" + "+libraryFilename+" (default: "+userConfigDirHint()+", created when missing)")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable ANSI color escape sequences in text output")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress the header line in text-mode output (no effect on JSON)")

	cobra.OnInitialize(initConfig)

	root.AddCommand(newExtractCmd())
	root.AddCommand(newHarmonyCmd())
	root.AddCommand(newGradientCmd())
	root.AddCommand(newContrastCmd())
	root.AddCommand(newBlindnessCmd())
	root.AddCommand(newConvertCmd())
	root.AddCommand(newSortCmd())
	root.AddCommand(newRandomCmd())
	root.AddCommand(newCSSCmd())
	root.AddCommand(newTailwindCmd())
	root.AddCommand(newLibraryCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newWebCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newCompletionCmd())
	root.AddCommand(newVersionCmd())
	return root
}

func initConfig() {
	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	dir, err := resolveDataDir(dataDir)
	if err != nil {
		// Surface --data-dir typos / missing dirs but don't abort —
		// every command can still run on built-in defaults.
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return
	}
	if dir == "" {
		return
	}
	cfgPath := filepath.Join(dir, configFilename)
	info, statErr := os.Stat(cfgPath)
	if statErr != nil || info.IsDir() {
		// No config.yaml in the resolved dir — common case before
		// any user customisation. Move on without warning.
		return
	}
	viper.SetConfigFile(cfgPath)
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			fmt.Fprintf(os.Stderr, "warning: reading %s: %v\n", cfgPath, err)
		}
	}
}

// userConfigDirHint returns the expanded default path used in --data-dir help.
func userConfigDirHint() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "huetension")
	}
	return "~/.config/huetension"
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the huetension version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "huetension %s\n", version)
		},
	}
}
