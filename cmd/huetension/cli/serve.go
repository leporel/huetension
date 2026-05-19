package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	hueserve "github.com/leporel/huetension/internal/serve"
	hueweb "github.com/leporel/huetension/internal/web"
)

// serveFlags collects the `huetension serve` subcommand flags. serve
// composes the SPA, the REST API, and the MCP HTTP/SSE transports behind
// one listener, so its flags are the union of the three surfaces' path
// prefixes plus the shared transport / sandbox knobs.
//
// One --auth-token, one --cors, one --log-level cover every surface — the
// combined server is a single security boundary. The embedded sandbox
// flags gate both POST /extract and the MCP image.extract tool.
type serveFlags struct {
	address       string
	webBasePath   string
	apiBasePath   string
	mcpBasePath   string
	mcpTransports []string
	authToken     string
	corsOrigins   []string
	logLevel      string
	logFormat     string
	devProxy      string
	sandboxFlags
}

func newServeCmd() *cobra.Command {
	var sf serveFlags

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the SPA, REST API, and MCP server on one listener",
		Long: "Run huetension's three network frontends — the embedded Vue SPA, the REST API, and the MCP " +
			"HTTP/SSE transports — behind a single address. This is the deployment-friendly counterpart to running " +
			"`huetension web` and `huetension mcp` as separate processes.\n\n" +
			"Routes: the SPA serves at the root, the REST API mounts under --api-base-path (default /api/v1), and the " +
			"MCP transports mount under --mcp-base-path (default /mcp; SSE at /mcp/sse). The MCP and API prefixes must " +
			"not overlap.\n\n" +
			"One --auth-token, --cors, and --log-level cover every surface. For non-loopback binds --auth-token is " +
			"mandatory and --read-only / --block-private-networks default to true, the same safety net the standalone " +
			"`web` and `mcp` commands apply. Container deployments bind 0.0.0.0 and pass --auth-token explicitly.\n\n" +
			"To expose only a subset of MCP tools, set enable / disable in the mcp: section of config.yaml inside " +
			"--data-dir — serve has no --enable/--disable flags of its own, and shares that section with `huetension mcp`.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, &sf)
		},
	}

	// Loopback default mirrors `huetension web` — the no-arg run must
	// succeed without --auth-token; network exposure is an explicit
	// --address opt-in that then makes --auth-token mandatory.
	cmd.Flags().StringVar(&sf.address, "address", "127.0.0.1:8080", "listen address (e.g. 127.0.0.1:8080 for loopback, 0.0.0.0:8080 for any interface)")
	cmd.Flags().StringVar(&sf.webBasePath, "web-base-path", "/", "URL prefix for the SPA (only \"/\" is supported by the bundled build)")
	cmd.Flags().StringVar(&sf.apiBasePath, "api-base-path", "/api/v1", "URL prefix for the REST API")
	cmd.Flags().StringVar(&sf.mcpBasePath, "mcp-base-path", "/mcp", "URL prefix for the MCP transports (SSE mounts at <prefix>/sse)")
	cmd.Flags().StringSliceVar(&sf.mcpTransports, "mcp-transport", []string{"http"}, "comma-separated MCP transports: http|sse (stdio is not valid on a shared listener)")
	cmd.Flags().StringVar(&sf.authToken, "auth-token", "", "Bearer token required for every request; mandatory when binding non-loopback")
	cmd.Flags().StringSliceVar(&sf.corsOrigins, "cors", nil, "Access-Control-Allow-Origin values ('*' or explicit origins); empty disables CORS")
	cmd.Flags().StringVar(&sf.logLevel, "log-level", "info", "log level (debug|info|warn|error); written to stderr")
	cmd.Flags().StringVar(&sf.logFormat, "log-format", "text", "log output format: text (human-readable, default) or json")
	cmd.Flags().StringVar(&sf.devProxy, "dev", "", "reverse-proxy non-API/MCP paths to a Vite dev server (e.g. http://localhost:5173); enables HMR while the Go backend runs unmodified")

	bindSandboxFlags(cmd, &sf.sandboxFlags, "/extract + image.extract path inputs", "non-loopback binds")

	return cmd
}

func runServe(cmd *cobra.Command, sf *serveFlags) error {
	loopback := hueweb.IsLoopbackBind(sf.address)
	applyLoopbackSandboxDefaults(cmd, loopback, &sf.sandboxFlags)
	if err := validateLoopbackRootSandbox(cmd, loopback, &sf.sandboxFlags); err != nil {
		return err
	}

	lib, libPath, err := loadLibrary(dataDir)
	if err != nil {
		return err
	}

	cfg := hueserve.Config{
		Version:       version,
		Address:       sf.address,
		AuthToken:     sf.authToken,
		CORSOrigins:   sf.corsOrigins,
		LogLevel:      sf.logLevel,
		LogFormat:     sf.logFormat,
		WebBasePath:   sf.webBasePath,
		APIBasePath:   sf.apiBasePath,
		MCPBasePath:   sf.mcpBasePath,
		MCPTransports: sf.mcpTransports,
		// serve has no --enable/--disable flags by design; the MCP tool
		// set it hosts comes from config.yaml's `mcp:` section (and the
		// HUETENSION_MCP_* env), shared with `huetension mcp`.
		MCPEnable:  viper.GetStringSlice(cfgKeyMCPEnable),
		MCPDisable: viper.GetStringSlice(cfgKeyMCPDisable),
		DevProxy:    sf.devProxy,
		Sandbox:     sf.toImageSandbox(),
		Library:     lib,
		LibraryPath: libPath,
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return hueserve.Run(ctx, cfg)
}
