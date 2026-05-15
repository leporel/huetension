package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/leporel/huetension/internal/httputil"
)

const (
	httpShutdownGrace = 5 * time.Second
	defaultBasePath   = "/api/v1"
)

// Run starts the web server: it serves the embedded SPA at "/" and the
// REST API under cfg.BasePath. Blocks until ctx is cancelled or the
// listener fails.
func Run(ctx context.Context, cfg Config) error {
	if strings.TrimSpace(cfg.Address) == "" {
		return errors.New("web: --address is required")
	}
	if !httputil.IsLoopbackAddr(cfg.Address) && strings.TrimSpace(cfg.AuthToken) == "" {
		return fmt.Errorf("web: bind address %q is non-loopback but --auth-token is empty; refusing to start", cfg.Address)
	}

	logger, err := resolveLogger(cfg)
	if err != nil {
		return err
	}

	handler, err := buildHandler(cfg, logger)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), httpShutdownGrace)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("web: listen %s: %w", cfg.Address, err)
	}
	return nil
}

// BuildHandler returns the composed http.Handler for the web server
// without starting a listener. Exposed for tests and for the future
// `huetension serve` command (S9) that mounts this handler alongside
// the MCP transports on a shared listener.
func BuildHandler(cfg Config) (http.Handler, error) {
	logger, err := resolveLogger(cfg)
	if err != nil {
		return nil, err
	}
	return buildHandler(cfg, logger)
}

// buildHandler is the shared composition step. Caller provides the
// already-resolved logger so Run and BuildHandler can both use it
// without re-resolving (and tests can inject their own).
func buildHandler(cfg Config, logger *slog.Logger) (http.Handler, error) {
	base := normaliseBasePath(cfg.BasePath)

	mux := http.NewServeMux()
	registerAPI(mux, base, apiHandlers{sandbox: cfg.Sandbox, library: cfg.Library})

	// "/" serves either the embedded SPA (default) or a reverse proxy
	// to the Vite dev server when --dev is set. API routes registered
	// above still resolve locally because ServeMux prefers longer
	// path patterns over the bare "/" fallback.
	root, err := newRootHandler(cfg)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", root)

	var h http.Handler = mux
	h = httputil.WithCORS(cfg.CORSOrigins, h)
	h = httputil.WithBearerAuth(cfg.AuthToken, h)
	h = httputil.WithAccessLog(logger, "web.http", h)
	return h, nil
}

// newRootHandler picks between the embedded SPA and the --dev reverse
// proxy. Kept separate from buildHandler so unit tests can exercise
// the selection rule directly.
func newRootHandler(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.DevProxy) != "" {
		return newDevProxyHandler(cfg.DevProxy)
	}
	return newSPAHandler(), nil
}

// IsLoopbackBind exposes httputil's loopback check so the CLI command
// can apply web-specific auto-defaults (read-only, block-private-
// networks) without importing the lower-level package directly. Same
// semantics as httputil.IsLoopbackAddr: "127.0.0.1:port", "[::1]:port",
// and "localhost:port" return true; ":port", "0.0.0.0:port", and
// any literal IP / hostname return false.
func IsLoopbackBind(addr string) bool {
	return httputil.IsLoopbackAddr(addr)
}

// normaliseBasePath returns the canonical prefix for REST routes.
// Defaults to "/api/v1"; ensures a leading "/", trims trailing "/".
func normaliseBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return defaultBasePath
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

// resolveLogger returns the *slog.Logger to use for this server.
// Mirrors internal/mcp's resolveLogger so both transports format logs
// identically — same fields, same destination (stderr by default).
func resolveLogger(cfg Config) (*slog.Logger, error) {
	if cfg.Logger != nil {
		return cfg.Logger, nil
	}
	level, err := parseLogLevel(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	return newJSONLogger(os.Stderr, level), nil
}

func newJSONLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("web: unknown log level %q (want debug|info|warn|error)", s)
}
