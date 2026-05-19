package cli

import (
	"context"

	"github.com/spf13/cobra"

	hueweb "github.com/leporel/huetension/internal/web"
)

// webFlags collects the `huetension web` subcommand flags. Transport
// knobs (--address, --base-path, --auth-token, --cors, --log-level) plus
// the embedded image-sandbox knobs (--read-only, --root, --allow-host,
// --max-image-bytes, --block-private-networks) that gate /api/v1/extract.
//
// Auto-default rules apply when the bind is non-loopback: --read-only
// and --block-private-networks flip on unless the operator explicitly
// sets them, same safety net MCP uses for HTTP/SSE transports.
//
// CLI must not import internal/web types beyond Config; see CLAUDE.md
// "Project layers" — this file is the only seam between the command
// tree and the web server package.
type webFlags struct {
	address     string
	basePath    string
	authToken   string
	corsOrigins []string
	logLevel    string
	logFormat   string
	devProxy    string
	sandboxFlags
}

func newWebCmd() *cobra.Command {
	var wf webFlags

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Run the huetension web UI + REST API",
		Long: "Run the embedded Vue SPA and the REST API that backs it. " +
			"The SPA is served at the root path; the REST endpoints mount under --base-path (default /api/v1).\n\n" +
			"For non-loopback binds, --auth-token is mandatory and --read-only / --block-private-networks default to true: " +
			"the same safety net MCP uses for HTTP/SSE transports. Loopback binds (127.0.0.1, ::1, localhost) may run " +
			"unauthenticated with filesystem access enabled.\n\n" +
			"This command speaks the same huetension/v1 JSON envelope as `huetension --format json` and `huetension mcp`; " +
			"any client that consumes one transport can decode the others without change.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWeb(cmd, &wf)
		},
	}

	// Loopback default mirrors what vite/jupyter ship — the no-arg run
	// must succeed without needing --auth-token. Operators who want
	// network exposure pass --address explicitly (and then --auth-token
	// becomes mandatory; see Run's refusal in internal/web/server.go).
	cmd.Flags().StringVar(&wf.address, "address", "127.0.0.1:8080", "listen address (e.g. 127.0.0.1:8080 for loopback, 0.0.0.0:8080 for any interface)")
	cmd.Flags().StringVar(&wf.basePath, "base-path", "/api/v1", "URL prefix for the REST API")
	cmd.Flags().StringVar(&wf.authToken, "auth-token", "", "Bearer token required for every request; mandatory when binding non-loopback")
	cmd.Flags().StringSliceVar(&wf.corsOrigins, "cors", nil, "Access-Control-Allow-Origin values ('*' or explicit origins); empty disables CORS")
	cmd.Flags().StringVar(&wf.logLevel, "log-level", "info", "log level (debug|info|warn|error); written to stderr")
	cmd.Flags().StringVar(&wf.logFormat, "log-format", "text", "log output format: text (human-readable, default) or json")
	cmd.Flags().StringVar(&wf.devProxy, "dev", "", "reverse-proxy non-API paths to a Vite dev server (e.g. http://localhost:5173); enables HMR while the Go API runs unmodified")

	bindSandboxFlags(cmd, &wf.sandboxFlags, "/extract path inputs", "non-loopback binds")

	return cmd
}

func runWeb(cmd *cobra.Command, wf *webFlags) error {
	loopback := hueweb.IsLoopbackBind(wf.address)
	applyLoopbackSandboxDefaults(cmd, loopback, &wf.sandboxFlags)
	if err := validateLoopbackRootSandbox(cmd, loopback, &wf.sandboxFlags); err != nil {
		return err
	}

	lib, libPath, err := loadLibrary(dataDir)
	if err != nil {
		return err
	}

	cfg := hueweb.Config{
		Version:     version,
		Address:     wf.address,
		BasePath:    wf.basePath,
		AuthToken:   wf.authToken,
		CORSOrigins: wf.corsOrigins,
		LogLevel:    wf.logLevel,
		LogFormat:   wf.logFormat,
		DevProxy:    wf.devProxy,
		Sandbox:     wf.toImageSandbox(),
		Library:     lib,
		LibraryPath: libPath,
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return hueweb.Run(ctx, cfg)
}
