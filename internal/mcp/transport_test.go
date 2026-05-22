package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/httputil"
)

// httpTestSetup boots a streamable HTTP handler in front of an isolated
// MCP server for the duration of the test. Returns the server URL and a
// teardown handle.
func httpTestSetup(t *testing.T, cfg Config) (string, func()) {
	t.Helper()
	srv, _, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	mux := http.NewServeMux()
	base := normaliseBasePath(cfg.BasePath)
	h := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, nil)
	mux.Handle(base, h)
	mux.Handle(base+"/", h)

	var handler http.Handler = mux
	handler = httputil.WithCORS(cfg.CORSOrigins, handler)
	handler = httputil.WithBearerAuth(cfg.AuthToken, handler)

	ts := httptest.NewServer(handler)
	return ts.URL + base, ts.Close
}

func TestHTTPEndToEnd(t *testing.T) {
	endpoint, teardown := httpTestSetup(t, Config{Version: "test"})
	defer teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Sanity: tools/list works.
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(list.Tools) == 0 {
		t.Fatalf("no tools registered")
	}

	// tools/call color.convert hex round-trip via the HTTP handler.
	res, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "color.convert",
		Arguments: map[string]any{"color": "royalblue", "to": "hex"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned IsError: %+v", res.Content)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(raw), `"value":"#4169e1"`) {
		t.Errorf("unexpected structured content: %s", raw)
	}
}

func TestHTTPRejectsMissingBearer(t *testing.T) {
	endpoint, teardown := httpTestSetup(t, Config{Version: "test", AuthToken: "secret"})
	defer teardown()

	resp, err := http.Get(endpoint)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); !strings.Contains(got, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, want Bearer challenge", got)
	}
}

func TestHTTPAcceptsValidBearer(t *testing.T) {
	endpoint, teardown := httpTestSetup(t, Config{Version: "test", AuthToken: "secret"})
	defer teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := sdk.NewClient(&sdk.Implementation{Name: "auth-client", Version: "test"}, nil)
	transport := &sdk.StreamableClientTransport{
		Endpoint: endpoint,
		HTTPClient: &http.Client{
			Transport: bearerInjectingTransport{token: "secret", base: http.DefaultTransport},
		},
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Connect with valid bearer: %v", err)
	}
	defer func() { _ = session.Close() }()

	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatalf("ListTools: %v", err)
	}
}

// bearerInjectingTransport stamps Authorization on every outbound request.
type bearerInjectingTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerInjectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(req2)
}

func TestHTTPCORSPreflight(t *testing.T) {
	endpoint, teardown := httpTestSetup(t, Config{Version: "test", CORSOrigins: []string{"*"}})
	defer teardown()

	req, _ := http.NewRequest(http.MethodOptions, endpoint, nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q, want *", got)
	}
}

func TestRunHTTPRefusesNonLoopbackWithoutAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := Run(ctx, Config{
		Version:    "test",
		Transports: []string{"http"},
		Address:    "0.0.0.0:0",
		// AuthToken intentionally empty
	})
	if err == nil {
		t.Fatalf("expected refusal for non-loopback without auth")
	}
	if !strings.Contains(err.Error(), "non-loopback") {
		t.Errorf("error %q should mention non-loopback", err)
	}
}

func TestRunHTTPLoopbackWithoutAuthIsAllowed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Config{
			Version:    "test",
			Transports: []string{"http"},
			// 127.0.0.1:0 → loopback, kernel picks port; auth not required.
			Address: "127.0.0.1:0",
		})
	}()

	// Give the listener a beat to come up, then kill it.
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		// err should be nil (graceful shutdown) — http.ErrServerClosed is
		// already mapped to nil inside runHTTP.
		if err != nil {
			t.Errorf("Run error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not return after context cancel")
	}
}

func TestNormaliseTransportsAll(t *testing.T) {
	got := normaliseTransports([]string{"all"})
	want := []string{"stdio", "http", "sse"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("at %d: got %q, want %q", i, got[i], n)
		}
	}
}

func TestNormaliseTransportsDedup(t *testing.T) {
	got := normaliseTransports([]string{"stdio", "stdio", " STDIO ", "http"})
	want := []string{"stdio", "http"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("at %d: got %q, want %q", i, got[i], n)
		}
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr     string
		loopback bool
	}{
		{"127.0.0.1:7337", true},
		{"localhost:7337", true},
		{"[::1]:7337", true},
		{":7337", false},
		{"0.0.0.0:7337", false},
		{"192.168.1.1:7337", false},
		{"example.com:7337", false},
	}
	for _, c := range cases {
		if got := httputil.IsLoopbackAddr(c.addr); got != c.loopback {
			t.Errorf("%q: got %v, want %v", c.addr, got, c.loopback)
		}
	}
}
