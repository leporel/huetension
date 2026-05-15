package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	webdist "github.com/leporel/huetension/web"
)

// newSPAHandler returns the handler mounted at "/" that serves the
// embedded Vue SPA. Unknown paths fall back to index.html so vue-router
// deep links (/library/aurora, /contrast?fg=...) resolve client-side
// instead of returning 404 from Go.
//
// Directory requests fall back too — without that, `/library/` (trailing
// slash) would be served by http.FileServer's auto-index, which we don't
// want for the SPA.
func newSPAHandler() http.Handler {
	sub, err := fs.Sub(webdist.Assets, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "web: SPA assets unavailable", http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			info, statErr := fs.Stat(sub, p)
			if statErr != nil || info.IsDir() {
				// Unknown path or directory request → serve index.html.
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				r2.URL.RawPath = ""
				fileServer.ServeHTTP(w, r2)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

// newDevProxyHandler returns a reverse-proxy handler that forwards every
// request to upstream (the Vite dev server). Used when Config.DevProxy
// is set so contributors get HMR for the Vue tree while the Go API runs
// unmodified at /api/v1/*.
//
// httputil.NewSingleHostReverseProxy preserves WebSocket upgrades via
// its default ServeHTTP path, so Vite's HMR socket round-trips through
// Go without extra wiring.
func newDevProxyHandler(upstream string) (http.Handler, error) {
	u, err := url.Parse(strings.TrimSpace(upstream))
	if err != nil {
		return nil, fmt.Errorf("web: --dev upstream %q: %w", upstream, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("web: --dev upstream %q: want http/https scheme", upstream)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("web: --dev upstream %q: missing host", upstream)
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	return proxy, nil
}
