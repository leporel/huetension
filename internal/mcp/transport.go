package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/httputil"
)

// httpShutdownGrace is the grace period given to in-flight HTTP requests
// when the parent context is cancelled.
const httpShutdownGrace = 5 * time.Second

// runHTTP serves the given srv over either streamable HTTP (sse=false) or
// SSE (sse=true). The same address+base-path can host both at once because
// "all" transport mode runs two runHTTP calls concurrently — this is fine
// only if they pick non-conflicting URL prefixes (HTTP at base, SSE at
// base+"/sse"); see Run() for orchestration.
//
// Refuses to start when:
//   - cfg.Address is non-loopback AND cfg.AuthToken is empty (mitigates
//     accidental public exposure of an LLM-driven server).
//   - cfg.Address is empty (no implicit default — caller must pass one).
func runHTTP(ctx context.Context, srv *sdk.Server, logger *slog.Logger, cfg Config, enabled []Descriptor, sse bool) error {
	if strings.TrimSpace(cfg.Address) == "" {
		return errors.New("mcp: http transport requires --address")
	}
	if !httputil.IsLoopbackAddr(cfg.Address) && strings.TrimSpace(cfg.AuthToken) == "" {
		return fmt.Errorf("mcp: http transport binds to non-loopback address %q but --auth-token is empty; refusing to start", cfg.Address)
	}

	base := normaliseBasePath(cfg.BasePath)

	mux := http.NewServeMux()
	// Share ONE handler instance across the trailing-slash variants. The
	// streamable / SSE handlers carry their own session map; mounting two
	// instances would split sessions across independent tables and break
	// any client whose follow-up request normalises the path differently
	// from its initial POST.
	if sse {
		ssePath := base + "/sse"
		h := sdk.NewSSEHandler(func(*http.Request) *sdk.Server { return srv }, nil)
		mux.Handle(ssePath, h)
		mux.Handle(ssePath+"/", h)
	} else {
		h := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, nil)
		mux.Handle(base, h)
		mux.Handle(base+"/", h)
	}

	// Order matters: access log wraps everything (so it sees the final
	// status set by auth/CORS too), then bearer auth (gates before CORS
	// is reached), then CORS innermost so OPTIONS are answered with the
	// proper headers. Wrapping order in code is reversed because each
	// step composes the *next* handler.
	var handler http.Handler = mux
	handler = httputil.WithCORS(cfg.CORSOrigins, handler)
	handler = httputil.WithBearerAuth(cfg.AuthToken, handler)
	handler = httputil.WithAccessLog(logger, "mcp.http", handler)

	httpSrv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Bind first, then announce: the "listening" line fires only once the
	// socket is actually up, so a bind failure surfaces as an error and
	// never a misleading "listening" log.
	ln, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("mcp: http listen %s: %w", cfg.Address, err)
	}

	transportName, endpoint := "http", base
	if sse {
		transportName, endpoint = "sse", base+"/sse"
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "mcp.listening",
		slog.String("transport", transportName),
		slog.String("address", ln.Addr().String()),
		slog.String("endpoint", endpoint),
		slog.Bool("auth", strings.TrimSpace(cfg.AuthToken) != ""),
		slog.Int("tool_count", len(enabled)),
		slog.Any("tools", enabledToolNames(enabled)),
	)

	// Shutdown bridge: when ctx is cancelled, drain in-flight requests with
	// a grace window then close the listener.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), httpShutdownGrace)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("mcp: http serve %s: %w", cfg.Address, err)
	}
	return nil
}

// HTTPHandler builds the MCP server per cfg and returns an http.Handler
// that serves the streamable HTTP transport and/or SSE on one mux, with
// routes rooted at cfg.BasePath. Unlike Run it opens no listener and adds
// no auth / CORS / access-log middleware: the `huetension serve` command
// mounts this beside the web handler and applies one shared middleware
// stack.
//
// cfg.Transports selects what mounts — "http" → streamable handler at
// BasePath, "sse" → SSE handler at BasePath+"/sse". Empty defaults to
// "http". "stdio" is rejected: a child-process transport has no meaning
// on a shared HTTP listener.
func HTTPHandler(cfg Config) (http.Handler, error) {
	srv, _, _, err := buildWithLogger(cfg)
	if err != nil {
		return nil, err
	}

	tlist := normaliseTransports(cfg.Transports)
	if len(tlist) == 0 {
		tlist = []string{"http"}
	}

	base := normaliseBasePath(cfg.BasePath)
	mux := http.NewServeMux()
	var httpUp, sseUp bool
	for _, t := range tlist {
		switch t {
		case "http":
			if httpUp {
				continue
			}
			// Share ONE handler instance across the trailing-slash
			// variants — see runHTTP for the session-table rationale.
			h := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, nil)
			mux.Handle(base, h)
			mux.Handle(base+"/", h)
			httpUp = true
		case "sse":
			if sseUp {
				continue
			}
			ssePath := base + "/sse"
			h := sdk.NewSSEHandler(func(*http.Request) *sdk.Server { return srv }, nil)
			mux.Handle(ssePath, h)
			mux.Handle(ssePath+"/", h)
			sseUp = true
		case "stdio":
			return nil, errors.New("mcp: stdio transport cannot run on a shared HTTP listener; use http and/or sse")
		default:
			return nil, fmt.Errorf("mcp: unknown transport %q", t)
		}
	}
	if !httpUp && !sseUp {
		return nil, errors.New("mcp: HTTPHandler needs at least one http/sse transport")
	}
	return mux, nil
}

// normaliseBasePath returns the canonical prefix for HTTP routes. Defaults
// to "/mcp"; ensures a leading "/", trims trailing "/".
func normaliseBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/mcp"
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

