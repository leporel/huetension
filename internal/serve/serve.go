// Package serve composes huetension's three network frontends — the Vue
// SPA, the REST API, and the MCP HTTP/SSE transports — behind a single
// listener. It is the backing for the `huetension serve` command.
//
// Each sub-stack is built by its own package as a bare http.Handler
// (web.Handler, mcp.HTTPHandler); serve mounts them on one ServeMux and
// wraps the result in the shared internal/httputil middleware, so a
// single --auth-token, --cors, and access log cover every surface.
package serve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/leporel/huetension/internal/httputil"
	"github.com/leporel/huetension/internal/mcp"
	"github.com/leporel/huetension/internal/palette/library"
	"github.com/leporel/huetension/internal/sandbox"
	"github.com/leporel/huetension/internal/web"
)

const (
	defaultWebBasePath = "/"
	defaultAPIBasePath = "/api/v1"
	defaultMCPBasePath = "/mcp"
	httpShutdownGrace  = 5 * time.Second
)

// Config controls the combined SPA + REST + MCP server.
type Config struct {
	// Version is reported to clients (web bundle version, MCP serverInfo).
	Version string

	// Address is the listen address (e.g. "127.0.0.1:8080"). Required. A
	// non-loopback bind with an empty AuthToken is refused at start — the
	// same safety net the standalone web and mcp commands apply.
	Address string

	// AuthToken, when set, is required as a Bearer token on every request
	// to every surface (SPA, REST, MCP). One token covers the process.
	AuthToken string

	// CORSOrigins, when non-empty, sets Access-Control-Allow-Origin for
	// every surface ("*" for allow-any). Empty disables CORS.
	CORSOrigins []string

	// LogLevel selects verbosity for the logger built when Logger is nil.
	// "debug"|"info"|"warn"|"error"; empty defaults to "info".
	LogLevel string

	// LogFormat selects the default logger's handler: "text" (default,
	// human-readable) or "json". Ignored when Logger is set explicitly.
	LogFormat string

	// Logger overrides the default logger. When nil a logger to stderr is
	// built in LogFormat at LogLevel. The resolved logger is shared by the
	// web handler, the MCP server middleware, and the access log, so one
	// process emits one log stream.
	Logger *slog.Logger

	// WebBasePath is where the SPA mounts. Only "/" is supported — the
	// bundled Vue build assumes a root base; a non-root value is rejected.
	WebBasePath string

	// APIBasePath is the REST prefix (default "/api/v1"). Must not be "/"
	// and must not nest with MCPBasePath.
	APIBasePath string

	// MCPBasePath is the MCP prefix (default "/mcp"); SSE mounts at
	// MCPBasePath+"/sse". Must not be "/" and must not nest with
	// APIBasePath.
	MCPBasePath string

	// MCPTransports selects the MCP HTTP transports: any subset of "http"
	// and "sse". Empty defaults to "http". "stdio" / "all" are rejected —
	// a shared HTTP listener cannot host a child-process transport.
	MCPTransports []string

	// MCPEnable / MCPDisable filter which MCP tools the hosted server
	// registers, using the same namespaced selector syntax as
	// `huetension mcp` --enable / --disable. The serve command fills these
	// from config.yaml's `mcp:` section; empty leaves the default set. An
	// unknown tool name fails BuildHandler loudly (not silently ignored).
	MCPEnable  []string
	MCPDisable []string

	// DevProxy, when set, reverse-proxies non-API/MCP paths to a Vite dev
	// server instead of serving the embedded SPA — same as `web --dev`.
	DevProxy string

	// Sandbox gates image extraction on both the REST /extract endpoint
	// and the MCP image.extract tool — one posture, both surfaces.
	Sandbox sandbox.ImageSandbox

	// Library is the curated palette catalogue served by the REST
	// /library* endpoints and the MCP library.* tools. nil makes those
	// surfaces respond as unconfigured (503 / tool error).
	Library *library.Index

	// LibraryPath is the on-disk library.json that the REST
	// POST /library/palette endpoint persists saved palettes to. Empty
	// disables saving (the route answers 503). The MCP surface is
	// read-only and ignores this.
	LibraryPath string
}

// Run starts the combined server and blocks until ctx is cancelled or the
// listener fails. The non-loopback-without-auth refusal happens here,
// before a listener is opened — the same gate web.Run and mcp.Run apply.
func Run(ctx context.Context, cfg Config) error {
	if strings.TrimSpace(cfg.Address) == "" {
		return errors.New("serve: --address is required")
	}
	if !httputil.IsLoopbackAddr(cfg.Address) && strings.TrimSpace(cfg.AuthToken) == "" {
		return fmt.Errorf("serve: bind address %q is non-loopback but --auth-token is empty; refusing to start", cfg.Address)
	}

	// Resolve the logger once and hand it to BuildHandler via cfg so the
	// startup line below and every surface share one sink.
	logger, err := resolveLogger(cfg)
	if err != nil {
		return err
	}
	cfg.Logger = logger

	handler, err := BuildHandler(cfg)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Bind first, then announce — a bind failure surfaces as an error,
	// never a misleading "listening" log.
	ln, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("serve: listen %s: %w", cfg.Address, err)
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "serve.listening",
		slog.String("address", ln.Addr().String()),
		slog.Bool("auth", strings.TrimSpace(cfg.AuthToken) != ""),
		slog.String("api_base", normaliseBase(cfg.APIBasePath, defaultAPIBasePath)),
		slog.String("mcp_base", normaliseBase(cfg.MCPBasePath, defaultMCPBasePath)),
	)

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), httpShutdownGrace)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: serve %s: %w", cfg.Address, err)
	}
	return nil
}

// BuildHandler composes the SPA + REST + MCP handlers into one
// http.Handler. It does not open a listener (Run does) and does not apply
// the non-loopback auth refusal (a Run-time concern). Exposed for tests
// and for callers embedding the combined handler.
func BuildHandler(cfg Config) (http.Handler, error) {
	logger, err := resolveLogger(cfg)
	if err != nil {
		return nil, err
	}

	webBase := normaliseBase(cfg.WebBasePath, defaultWebBasePath)
	if webBase != "/" {
		return nil, fmt.Errorf("serve: --web-base-path %q is unsupported; the bundled SPA assumes a root base "+
			"(Vite `base` and the vue-router history base would have to change first)", cfg.WebBasePath)
	}
	apiBase := normaliseBase(cfg.APIBasePath, defaultAPIBasePath)
	mcpBase := normaliseBase(cfg.MCPBasePath, defaultMCPBasePath)
	if apiBase == "/" {
		return nil, errors.New(`serve: --api-base-path must not be "/" (it would shadow the SPA)`)
	}
	if mcpBase == "/" {
		return nil, errors.New(`serve: --mcp-base-path must not be "/" (it would shadow the SPA)`)
	}
	if basesCollide(apiBase, mcpBase) {
		return nil, fmt.Errorf("serve: --api-base-path %q and --mcp-base-path %q collide; pick non-overlapping prefixes", apiBase, mcpBase)
	}

	mcpTransports, err := resolveMCPTransports(cfg.MCPTransports)
	if err != nil {
		return nil, err
	}

	// Web handler: REST under apiBase + SPA at "/", no middleware (serve
	// wraps the composition once below). AuthToken / CORSOrigins are left
	// off Config deliberately — the outer wrap owns them.
	webHandler, err := web.Handler(web.Config{
		Version:     cfg.Version,
		BasePath:    apiBase,
		Logger:      logger,
		Sandbox:     cfg.Sandbox,
		Library:     cfg.Library,
		LibraryPath: cfg.LibraryPath,
		DevProxy:    cfg.DevProxy,
	})
	if err != nil {
		return nil, err
	}

	// MCP handler: streamable HTTP (and SSE) rooted at mcpBase, no
	// listener, no middleware. The sandbox is the same posture the REST
	// /extract endpoint uses, flattened into mcp.Config's fields.
	mcpHandler, err := mcp.HTTPHandler(mcp.Config{
		Version:              cfg.Version,
		Enable:               cfg.MCPEnable,
		Disable:              cfg.MCPDisable,
		Transports:           mcpTransports,
		BasePath:             mcpBase,
		Logger:               logger,
		ReadOnly:             cfg.Sandbox.ReadOnly,
		Root:                 cfg.Sandbox.Root,
		AllowHosts:           cfg.Sandbox.AllowHosts,
		MaxImageBytes:        cfg.Sandbox.MaxImageBytes,
		BlockPrivateNetworks: cfg.Sandbox.BlockPrivateNetworks,
		Library:              cfg.Library,
		LibraryPath:          cfg.LibraryPath,
	})
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	// MCP is mounted on its own prefix; the web handler takes "/".
	// ServeMux longest-prefix routing sends mcpBase + mcpBase/* into the
	// MCP handler, and everything else — including apiBase/* and SPA deep
	// links — into the web handler, which then routes the REST surface
	// and the SPA fallback internally.
	mux.Handle(mcpBase, mcpHandler)
	mux.Handle(mcpBase+"/", mcpHandler)
	mux.Handle("/", webHandler)

	var h http.Handler = mux
	h = httputil.WithCORS(cfg.CORSOrigins, h)
	h = httputil.WithBearerAuth(cfg.AuthToken, h)
	h = httputil.WithAccessLog(logger, "serve.http", h)
	return h, nil
}

// normaliseBase canonicalises a URL prefix: a leading "/" is ensured, a
// trailing "/" trimmed, and an empty input falls back to def.
func normaliseBase(p, def string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return def
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

// basesCollide reports whether two normalised, non-root path prefixes are
// equal or nest (one is a path-segment prefix of the other). Either case
// would let one surface shadow the other on the shared mux.
func basesCollide(a, b string) bool {
	if a == b {
		return true
	}
	return strings.HasPrefix(a+"/", b+"/") || strings.HasPrefix(b+"/", a+"/")
}

// resolveMCPTransports validates the serve-side MCP transport list. Only
// "http" and "sse" are meaningful on a shared HTTP listener; "stdio" and
// "all" are rejected with a pointed error. Empty defaults to "http".
func resolveMCPTransports(in []string) ([]string, error) {
	if len(in) == 0 {
		return []string{"http"}, nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		t := strings.ToLower(strings.TrimSpace(raw))
		switch t {
		case "":
			continue
		case "http", "sse":
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				out = append(out, t)
			}
		case "stdio":
			return nil, errors.New("serve: --mcp-transport stdio is invalid; serve hosts MCP over HTTP — use http and/or sse")
		case "all":
			return nil, errors.New("serve: --mcp-transport all is invalid here; list http and/or sse explicitly")
		default:
			return nil, fmt.Errorf("serve: unknown --mcp-transport %q (want http|sse)", t)
		}
	}
	if len(out) == 0 {
		return []string{"http"}, nil
	}
	return out, nil
}

// resolveLogger returns the logger shared by every surface. Delegates to
// httputil.NewLogger so a process started via `serve` formats logs
// identically to one started via `web` or `mcp`.
func resolveLogger(cfg Config) (*slog.Logger, error) {
	if cfg.Logger != nil {
		return cfg.Logger, nil
	}
	return httputil.NewLogger(os.Stderr, cfg.LogFormat, cfg.LogLevel)
}
