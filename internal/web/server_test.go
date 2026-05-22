package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSPAFallback verifies that:
//   - "/" serves the placeholder index.html.
//   - unknown SPA-style paths fall back to index.html so vue-router
//     deep links resolve client-side.
//   - REST routes that DO exist are not shadowed by the fallback.
func TestSPAFallback(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	for _, path := range []string{"/", "/library/aurora", "/contrast", "/anything-the-spa-may-route"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, resp.StatusCode)
		}
		if !strings.Contains(string(body), "huetension") {
			t.Errorf("%s body missing placeholder marker: %s", path, body)
		}
	}

	// REST routes mounted before the SPA fallback must still answer.
	resp, err := http.Get(base + "/api/v1/color/convert?color=%23000000")
	if err != nil {
		t.Fatalf("API GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("API status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("API Content-Type = %q, want application/json", ct)
	}
}

// TestBearerAuthGate confirms the middleware shape: with a non-empty
// AuthToken, requests without the matching Bearer header receive 401
// and the WWW-Authenticate header. Loopback bind is fine — Run's
// non-loopback safety check is exercised separately.
func TestBearerAuthGate(t *testing.T) {
	cfg := Config{
		Version:   "test",
		AuthToken: "secret",
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()

	// No token → 401.
	resp, err := http.Get(ts.URL + "/api/v1/color/convert?color=%23000000")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-token status = %d, want 401", resp.StatusCode)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Errorf("missing WWW-Authenticate header")
	}

	// Correct token → 200.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/color/convert?color=%23000000", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("auth GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("with-token status = %d, want 200", resp.StatusCode)
	}
}

// TestCORSPreflight asserts the OPTIONS short-circuit returns 204 with
// the configured Access-Control-Allow-Origin echoed back. Browsers
// require this before making the real cross-origin request.
func TestCORSPreflight(t *testing.T) {
	cfg := Config{
		Version:     "test",
		CORSOrigins: []string{"https://example.com"},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/color/convert", nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("ACAO = %q, want https://example.com", got)
	}
}

// TestRunRejectsNonLoopbackWithoutToken locks in the safety net: a
// non-loopback bind without an auth token must refuse to start.
//
// We exercise Run directly rather than spinning up a listener, because
// the refusal happens before ListenAndServe is reached.
func TestRunRejectsNonLoopbackWithoutToken(t *testing.T) {
	cases := []string{
		"0.0.0.0:9999",
		":9999", // "any interface" — empty host falls under IsLoopbackAddr's
		// conservative "assume non-loopback" branch, so the safety
		// net must fire here too. Locks in the rationale behind the
		// CLI's 127.0.0.1:8080 default.
	}
	for _, addr := range cases {
		cfg := Config{
			Address: addr,
			Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
		err := Run(t.Context(), cfg)
		if err == nil {
			t.Errorf("%s: expected refusal, got nil", addr)
			continue
		}
		if !strings.Contains(err.Error(), "non-loopback") {
			t.Errorf("%s: error = %v, want a non-loopback message", addr, err)
		}
	}
}

// TestDevProxyForwardsNonAPI verifies that when Config.DevProxy points at
// an upstream (the Vite dev server in real use), every non-API path is
// reverse-proxied while /api/v1/* still resolves to the local Go
// handlers. The marker body from the upstream confirms the proxy round-
// trip; the JSON content-type on the API path confirms the local API
// was not shadowed by the root proxy.
func TestDevProxyForwardsNonAPI(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("vite:" + r.URL.Path))
	}))
	defer upstream.Close()

	cfg := Config{
		Version:  "test",
		DevProxy: upstream.URL,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()

	// SPA-shaped paths reverse-proxy to upstream.
	for _, path := range []string{"/", "/library/aurora", "/src/main.ts"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		want := "vite:" + path
		if string(body) != want {
			t.Errorf("%s body = %q, want %q", path, body, want)
		}
	}

	// API path stays local.
	resp, err := http.Get(ts.URL + "/api/v1/color/convert?color=%23000000")
	if err != nil {
		t.Fatalf("API GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("API status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("API Content-Type = %q, want application/json", ct)
	}
}

// TestDevProxyRejectsBadUpstream catches typos / missing scheme at
// BuildHandler time so operators see the failure at start, not on the
// first request.
func TestDevProxyRejectsBadUpstream(t *testing.T) {
	for _, bad := range []string{"localhost:5173", "ftp://x", "http://"} {
		cfg := Config{DevProxy: bad, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		if _, err := BuildHandler(cfg); err == nil {
			t.Errorf("BuildHandler(DevProxy=%q): expected error, got nil", bad)
		}
	}
}

// TestNormaliseBasePath documents the canonicalisation rules so future
// changes surface as test diffs rather than silent breakage.
func TestNormaliseBasePath(t *testing.T) {
	cases := map[string]string{
		"":         "/api/v1",
		"/api/v1":  "/api/v1",
		"api/v1":   "/api/v1",
		"/api/v1/": "/api/v1",
		"/":        "/",
		"/custom/": "/custom",
	}
	for in, want := range cases {
		if got := normaliseBasePath(in); got != want {
			t.Errorf("normaliseBasePath(%q) = %q, want %q", in, got, want)
		}
	}
}
