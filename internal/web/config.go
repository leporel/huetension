// Package web hosts huetension's REST API and embedded Vue SPA. It is the
// transport-layer sibling of internal/mcp — both wrap the same Phase-1
// internal libraries (color, palette, harmony, ...) and emit the same
// huetension/v1 JSON envelope, just through different wire protocols.
//
// Entry points: Run (start a listener) and BuildHandler (compose the
// http.Handler chain without listening — used by tests and by the
// future `huetension serve` command that hosts SPA + REST + MCP under
// one process).
package web

import (
	"log/slog"

	"github.com/leporel/huetension/internal/palette/library"
	"github.com/leporel/huetension/internal/sandbox"
)

// Config controls the web server's listener, security, and logging.
type Config struct {
	// Version is reported back to clients (currently as the User-Agent
	// fallback and in future SPA bundle versioning). The CLI passes the
	// binary's --version-stamped value.
	Version string

	// Address is the listen address (e.g. ":8080" or "127.0.0.1:8080").
	// Required. Binding to a non-loopback address without AuthToken is
	// refused at start time — same safety net MCP uses.
	Address string

	// BasePath is the URL prefix for the REST API (default "/api/v1").
	// The SPA is served at the root, regardless of BasePath; only the
	// REST handlers mount under the prefix.
	BasePath string

	// AuthToken, when non-empty, is required as a Bearer token on every
	// request. Loopback binds may leave it empty (single-user dev
	// scenario); non-loopback binds refuse to start without one.
	AuthToken string

	// CORSOrigins, when non-empty, sets Access-Control-Allow-Origin
	// values ("*" for allow-any). Empty disables CORS entirely.
	CORSOrigins []string

	// LogLevel selects verbosity for the default logger built when Logger
	// is nil. Accepts "debug", "info", "warn", "error". Empty defaults to
	// "info".
	LogLevel string

	// LogFormat selects the default logger's handler: "text" (default,
	// human-readable) or "json". Ignored when Logger is set explicitly.
	LogFormat string

	// Logger overrides the default logger. When nil, BuildHandler / Run
	// construct a logger writing to stderr in LogFormat at LogLevel.
	Logger *slog.Logger

	// Sandbox controls /api/v1/extract: which path inputs are allowed
	// (ReadOnly / Root), which hosts URL inputs can resolve to (AllowHosts /
	// BlockPrivateNetworks), and the cap on decoded payload size
	// (MaxImageBytes). Mirrored from MCP — the same struct gates both
	// transports, so a single security posture covers any way the user
	// reaches the server.
	Sandbox sandbox.ImageSandbox

	// Library is the curated palette catalogue served by /api/v1/library*.
	// nil makes those endpoints respond 503 (Service Unavailable). The
	// CLI loads this via library.Load(externalPath) and threads it in;
	// see cmd/huetension/cli/web.go.
	Library *library.Index

	// LibraryPath is the on-disk library.json a saved palette is
	// persisted to (POST /api/v1/library/palette). It may name a file
	// that does not exist yet — the first save bootstraps it with the
	// embedded defaults plus the new entry. Empty disables saving: the
	// save endpoint then answers 503 while the read endpoints still work.
	LibraryPath string

	// LibraryStore, when set, is used instead of building a store from
	// Library + LibraryPath. The combined `serve` command passes one store
	// to both the REST and MCP surfaces so a save on either side is
	// visible to, and never overwritten by, the other.
	LibraryStore *library.Store

	// DevProxy, when non-empty, makes the server reverse-proxy every non-API
	// request to this URL (e.g. "http://localhost:5173" for the Vite dev
	// server) instead of serving the embedded SPA. API routes under BasePath
	// still resolve to the local Go handlers, so contributors get HMR for
	// the Vue tree while the backend runs unmodified.
	//
	// WebSocket upgrades (Vite HMR) pass through via httputil's default
	// reverse-proxy behaviour.
	DevProxy string
}

// libraryStore returns the shared store when one was injected, otherwise
// a fresh store over the configured index and path.
func (c Config) libraryStore() *library.Store {
	if c.LibraryStore != nil {
		return c.LibraryStore
	}
	return library.NewStore(c.Library, c.LibraryPath)
}
