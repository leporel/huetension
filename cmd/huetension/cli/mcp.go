package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	huemcp "github.com/leporel/huetension/internal/mcp"
)

// mcpFlags collects the `huetension mcp` subcommand flags.
//
// Auto-default rules (applied when the flag is not explicitly set on the
// command line) live in applyAutoDefaults — chiefly: HTTP/SSE transports
// flip ReadOnly and BlockPrivateNetworks on by default, because exposing
// filesystem access or letting an LLM SSRF the local network through a
// public-facing MCP server is the obvious mistake to prevent.
type mcpFlags struct {
	transports  []string
	enable      []string
	disable     []string
	listTools   bool
	listFmt     string
	logLevel    string
	logFormat   string
	address     string
	basePath    string
	authToken   string
	corsOrigins []string
	sandboxFlags
}

func newMCPCmd() *cobra.Command {
	var mf mcpFlags

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run huetension as a Model Context Protocol server",
		Long: "Run huetension as an MCP server. The server speaks the same JSON wire contract as the CLI's --format json mode, " +
			"so anything an LLM extracts from a tool result lines up with what the CLI would emit.\n\n" +
			"Transports: stdio (default; child-process MCP for Claude Desktop / editors), http (modern streamable, recommended for remote use), sse (legacy server-sent events). " +
			"Pass --transport stdio,http or --transport all to run multiple concurrently.\n\n" +
			"For HTTP/SSE the server refuses to bind a non-loopback address without --auth-token, and ReadOnly + BlockPrivateNetworks default to true. " +
			"Pin --root, --allow-host, --max-image-bytes when exposing the server beyond localhost.\n\n" +
			"The --enable / --disable tool selectors can also be set in the mcp: section of config.yaml inside --data-dir; " +
			"a flag passed on the command line overrides the file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCP(cmd, &mf)
		},
	}

	cmd.Flags().StringSliceVar(&mf.transports, "transport", []string{"stdio"}, "comma-separated transports: stdio|http|sse|all")
	cmd.Flags().StringSliceVar(&mf.enable, "enable", nil, "comma-separated whitelist; bare names target tools, prefix with resources: or prompts: for those kinds, kind:* for wildcards")
	cmd.Flags().StringSliceVar(&mf.disable, "disable", nil, "comma-separated blacklist; same namespaced syntax as --enable; applied after --enable")
	cmd.Flags().BoolVar(&mf.listTools, "list-tools", false, "print the registered tool catalogue and exit")
	cmd.Flags().StringVar(&mf.listFmt, "list-format", "text", "format for --list-tools output (text|json)")
	cmd.Flags().StringVar(&mf.logLevel, "log-level", "info", "log level (debug|info|warn|error); written to stderr")
	cmd.Flags().StringVar(&mf.logFormat, "log-format", "text", "log output format: text (human-readable, default) or json")

	bindSandboxFlags(cmd, &mf.sandboxFlags, "image.extract path inputs", "http/sse transports")

	// Loopback default mirrors `huetension web` / `serve`: a no-arg
	// `--transport http` run must succeed locally without --auth-token.
	// Exposing the server is an explicit `--address 0.0.0.0:7337` opt-in
	// (which then makes --auth-token mandatory; see runHTTP's refusal).
	cmd.Flags().StringVar(&mf.address, "address", "127.0.0.1:7337", "listen address for http/sse transports (e.g. 127.0.0.1:7337 for loopback, 0.0.0.0:7337 for any interface)")
	cmd.Flags().StringVar(&mf.basePath, "base-path", "/mcp", "URL prefix for the streamable HTTP handler (sse mounts at base-path/sse)")
	cmd.Flags().StringVar(&mf.authToken, "auth-token", "", "Bearer token required for http/sse requests; mandatory when binding non-loopback")
	cmd.Flags().StringSliceVar(&mf.corsOrigins, "cors", nil, "Access-Control-Allow-Origin values for http/sse responses ('*' or explicit origins); empty disables CORS")

	return cmd
}

// applyAutoDefaults flips security-relevant defaults to safer values when
// the operator runs HTTP/SSE without explicitly opting out. The user can
// still override by passing --read-only=false (etc.) — Cobra's
// Flag.Changed lets us distinguish "left at default" from "explicitly set".
func applyAutoDefaults(cmd *cobra.Command, mf *mcpFlags) {
	if !hasNetworkTransport(mf.transports) {
		return
	}
	if !cmd.Flags().Changed("read-only") {
		mf.readOnly = true
	}
	if !cmd.Flags().Changed("block-private-networks") {
		mf.blockPrivateNetworks = true
	}
}

func hasNetworkTransport(transports []string) bool {
	for _, t := range transports {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "http", "sse", "all":
			return true
		}
	}
	return false
}

// validateRootSandbox refuses to start when an operator runs a network
// transport with --read-only=false but never pinned --root. The default
// "." (cwd) is too broad to be a sandbox — the auto-defaults would have
// set ReadOnly=true had the operator not explicitly cleared it, so this
// path is reached only when --read-only=false was passed by hand. We
// catch that case rather than silently exposing the cwd to the network.
func validateRootSandbox(cmd *cobra.Command, mf *mcpFlags) error {
	if !hasNetworkTransport(mf.transports) {
		return nil
	}
	if mf.readOnly {
		return nil
	}
	if cmd.Flags().Changed("root") {
		return nil
	}
	return fmt.Errorf("refusing to start: --read-only=false on a network transport requires an explicit --root pinning the sandbox directory; %q (cwd) is too broad", mf.root)
}

func runMCP(cmd *cobra.Command, mf *mcpFlags) error {
	applyAutoDefaults(cmd, mf)
	if err := validateRootSandbox(cmd, mf); err != nil {
		return err
	}

	// Layer config.yaml's `mcp:` section (and HUETENSION_MCP_* env) under
	// the explicit --enable / --disable flags: a flag passed on the
	// command line wins, otherwise the file/env value is used.
	if err := viper.BindPFlag(cfgKeyMCPEnable, cmd.Flags().Lookup("enable")); err != nil {
		return err
	}
	if err := viper.BindPFlag(cfgKeyMCPDisable, cmd.Flags().Lookup("disable")); err != nil {
		return err
	}

	lib, libPath, err := loadLibrary(dataDir)
	if err != nil {
		return err
	}

	cfg := huemcp.Config{
		Version:              version,
		Enable:               viper.GetStringSlice(cfgKeyMCPEnable),
		Disable:              viper.GetStringSlice(cfgKeyMCPDisable),
		ReadOnly:             mf.readOnly,
		Root:                 mf.root,
		AllowHosts:           mf.allowHosts,
		MaxImageBytes:        mf.maxImageBytes,
		BlockPrivateNetworks: mf.blockPrivateNetworks,
		Transports:           mf.transports,
		Address:              mf.address,
		BasePath:             mf.basePath,
		AuthToken:            mf.authToken,
		CORSOrigins:          mf.corsOrigins,
		LogLevel:             mf.logLevel,
		LogFormat:            mf.logFormat,
		Library:              lib,
		LibraryPath:          libPath,
	}

	if mf.listTools {
		enabled, err := huemcp.ResolveEnabled(cfg)
		if err != nil {
			return err
		}
		return printToolList(enabled, mf.listFmt)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return huemcp.Run(ctx, cfg)
}

// printToolList writes the catalogue to stdoutWriter (which tests swap for
// a buffer). JSON form mirrors the shape sketched in .prompts/01-phase2-mcp.md
// so MCP clients can parse it directly.
func printToolList(tools []huemcp.Descriptor, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		var b strings.Builder
		for _, d := range tools {
			fmt.Fprintf(&b, "%-30s %s\n", d.Name, d.Description)
		}
		_, err := fmt.Fprint(stdoutWriter, b.String())
		return err
	case "json":
		type entry struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Enabled     bool   `json:"enabled"`
		}
		out := make([]entry, len(tools))
		for i, d := range tools {
			out[i] = entry{Name: d.Name, Description: d.Description, Enabled: true}
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdoutWriter, string(data))
		return err
	default:
		return fmt.Errorf("unknown --list-format %q (want text|json)", format)
	}
}
