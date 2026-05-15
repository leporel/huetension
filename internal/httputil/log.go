package httputil

import (
	"log/slog"
	"net/http"
	"time"
)

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
