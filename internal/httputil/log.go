package httputil

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// NewLogger builds the operational *slog.Logger shared by huetension's
// long-running servers (web / mcp / serve).
//
// format selects the handler: "text" (default — human-readable
// "key=value" lines) or "json" (one object per line, for log shippers).
// level is "debug" | "info" | "warn" | "error"; empty defaults to "info".
// Both are matched case-insensitively; an unknown value is an error.
//
// w is the sink — the servers pass os.Stderr: the MCP stdio transport
// owns stdout for JSON-RPC, so any log byte there would corrupt the wire,
// and stderr keeps every transport consistent.
func NewLogger(w io.Writer, format, level string) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("unknown log format %q (want text|json)", format)
	}
}

// parseLevel maps a CLI-friendly level name to a slog.Level. Empty or
// unset defaults to info — the safe middle ground for an unattended
// server.
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q (want debug|info|warn|error)", s)
	}
}

// WithAccessLog logs every HTTP request at info level. Captured fields:
// method, path, status, duration, remote address. msg is the slog event
// message — pass a transport-specific tag like "mcp.http" or "web.http"
// so logs from one process running both transports can be filtered.
//
// A nil logger disables the middleware (returns next unchanged) — useful
// for tests that want a quieter handler chain.
func WithAccessLog(logger *slog.Logger, msg string, next http.Handler) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		logger.LogAttrs(r.Context(), slog.LevelInfo, msg,
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

// Unwrap exposes the wrapped ResponseWriter so http.ResponseController
// (and any middleware that traverses wrappers) can reach optional
// interfaces like http.Flusher. The MCP streamable transport flushes its
// SSE stream through a ResponseController; without this it would no-op
// silently behind the access log, stalling server→client events.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
