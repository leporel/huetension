package mcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// resolveLogger returns the *slog.Logger to use for this server. When the
// caller pre-set cfg.Logger (tests, embedding hosts), that instance is
// returned verbatim — same handler, same destination. Otherwise we build
// a JSON logger writing to stderr at the level named by cfg.LogLevel.
//
// Stderr is deliberate: the stdio transport speaks JSON-RPC on stdout, so
// any log byte on stdout would corrupt the wire. Anything we own writes
// to stderr.
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

// parseLogLevel maps a CLI-friendly level name to a slog.Level. Empty or
// unset defaults to info — the safest middle ground for an LLM-driven
// server that may run unattended.
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
	return 0, fmt.Errorf("mcp: unknown log level %q (want debug|info|warn|error)", s)
}

// loggingMiddleware logs every receiving method dispatch as a single info
// entry: method, duration, error status, plus a method-specific subject
// (tool name, resource URI, prompt name) when one is naturally available.
//
// Tool *arguments* are intentionally not logged — they may contain user
// data (image paths, color strings tagged with internal codenames, etc.).
// At debug level we log argument types only, never their values. If a
// future caller needs full argument logging, they should set their own
// cfg.Logger with redaction in place.
func loggingMiddleware(logger *slog.Logger) sdk.Middleware {
	return func(next sdk.MethodHandler) sdk.MethodHandler {
		return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
			start := time.Now()
			result, err := next(ctx, method, req)
			attrs := []any{
				slog.String("method", method),
				slog.Duration("duration", time.Since(start)),
			}
			if sess := req.GetSession(); sess != nil {
				if id := sess.ID(); id != "" {
					attrs = append(attrs, slog.String("session_id", id))
				}
			}
			if subject, ok := methodSubject(method, req); ok {
				attrs = append(attrs, slog.String("subject", subject))
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				logger.LogAttrs(ctx, slog.LevelError, "mcp.call", toAttrs(attrs)...)
				return result, err
			}
			logger.LogAttrs(ctx, slog.LevelInfo, "mcp.call", toAttrs(attrs)...)
			return result, nil
		}
	}
}

// methodSubject pulls a stable, safe-to-log identifier out of req.Params
// for the methods where one exists. Returns ("", false) for methods that
// don't carry a meaningful subject (initialise, list, ping, …).
func methodSubject(method string, req sdk.Request) (string, bool) {
	params := req.GetParams()
	if params == nil {
		return "", false
	}
	switch method {
	case "tools/call":
		if p, ok := params.(*sdk.CallToolParamsRaw); ok && p != nil {
			return p.Name, p.Name != ""
		}
		if p, ok := params.(*sdk.CallToolParams); ok && p != nil {
			return p.Name, p.Name != ""
		}
	case "resources/read":
		if p, ok := params.(*sdk.ReadResourceParams); ok && p != nil {
			return p.URI, p.URI != ""
		}
	case "prompts/get":
		if p, ok := params.(*sdk.GetPromptParams); ok && p != nil {
			return p.Name, p.Name != ""
		}
	}
	return "", false
}

// toAttrs converts the loose []any slog argument list into []slog.Attr.
// Required because LogAttrs accepts []slog.Attr, not the variadic []any
// slog.Logger.Info uses. Centralising the conversion keeps the call sites
// readable without a wrapper helper per call.
func toAttrs(in []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(in))
	for _, v := range in {
		if a, ok := v.(slog.Attr); ok {
			out = append(out, a)
		}
	}
	return out
}

// withAccessLog logs HTTP transport requests at info level. Captured
// fields: method, path, status, duration, remote address. Mirrors the
// loggingMiddleware shape so both wire layers produce comparable output.
func withAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		logger.LogAttrs(r.Context(), slog.LevelInfo, "mcp.http",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
			slog.String("remote", r.RemoteAddr),
		)
	})
}

// statusRecorder wraps http.ResponseWriter to remember the status code
// the inner handler wrote. Without it we could not surface it in the
// access log — the stdlib ResponseWriter does not expose the status.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}
