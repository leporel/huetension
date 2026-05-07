package mcp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
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
func runHTTP(ctx context.Context, srv *sdk.Server, logger *slog.Logger, cfg Config, sse bool) error {
	if strings.TrimSpace(cfg.Address) == "" {
		return errors.New("mcp: http transport requires --address")
	}
	if !isLoopbackAddr(cfg.Address) && strings.TrimSpace(cfg.AuthToken) == "" {
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
	handler = withCORS(cfg.CORSOrigins, handler)
	handler = withBearerAuth(cfg.AuthToken, handler)
	handler = withAccessLog(logger, handler)

	httpSrv := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Shutdown bridge: when ctx is cancelled, drain in-flight requests with
	// a grace window then close the listener.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), httpShutdownGrace)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("mcp: http listen %s: %w", cfg.Address, err)
	}
	return nil
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

// withBearerAuth gates downstream when token is non-empty. Empty token
// means no authentication (loopback servers without --auth-token).
//
// Comparison is constant-time (crypto/subtle) so a remote caller cannot
// recover the token by measuring response time across guesses.
func withBearerAuth(token string, next http.Handler) http.Handler {
	if strings.TrimSpace(token) == "" {
		return next
	}
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always respond to CORS preflight without auth — browsers send
		// OPTIONS without Authorization, and the CORS layer answers them.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="huetension"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withCORS adds Access-Control-* headers and short-circuits OPTIONS
// preflight. When origins is empty the middleware is a no-op (no CORS).
func withCORS(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	allowAll := false
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAll = true
		}
		allowed[o] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Mcp-Session-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopbackAddr returns true when addr binds only to loopback interfaces.
// Recognised forms:
//
//	"127.0.0.1:7337", "[::1]:7337", "localhost:7337" → loopback
//	":7337"                                          → all interfaces (NOT loopback)
//	anything else (a literal IP / hostname)          → assume non-loopback
//
// We avoid DNS resolution at startup — be conservative when uncertain.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
