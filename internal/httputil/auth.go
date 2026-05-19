// Package httputil holds HTTP transport helpers shared by huetension's
// network-facing internal packages (mcp, web, serve): thin http.Handler
// middleware, request-classification predicates, and the shared
// operational logger constructor. Nothing here is business-specific.
package httputil

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// WithBearerAuth gates downstream when token is non-empty. Empty token
// means no authentication (e.g. a loopback server without --auth-token).
//
// Comparison is constant-time (crypto/subtle) so a remote caller cannot
// recover the token by measuring response time across guesses. The
// CORS preflight (OPTIONS) is allowed through unauthenticated — browsers
// send it without Authorization, and the CORS layer answers them.
func WithBearerAuth(token string, next http.Handler) http.Handler {
	if strings.TrimSpace(token) == "" {
		return next
	}
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// IsLoopbackAddr returns true when addr binds only to loopback interfaces.
// Recognised forms:
//
//	"127.0.0.1:7337", "[::1]:7337", "localhost:7337" → loopback
//	":7337"                                          → all interfaces (NOT loopback)
//	anything else (a literal IP / hostname)          → assume non-loopback
//
// We avoid DNS resolution at startup — be conservative when uncertain.
func IsLoopbackAddr(addr string) bool {
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
