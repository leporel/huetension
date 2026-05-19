package serve

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// quietLogger keeps test output clean — BuildHandler would otherwise
// build a JSON logger writing to stderr.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newServeHandler(t *testing.T, cfg Config) http.Handler {
	t.Helper()
	if cfg.Logger == nil {
		cfg.Logger = quietLogger()
	}
	if cfg.Version == "" {
		cfg.Version = "test"
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	return h
}

// TestComposesThreeSurfaces is the headline S9 check: one handler, and
// the SPA, the REST API, and the MCP transport are all reachable on it
// at their default prefixes without shadowing each other.
func TestComposesThreeSurfaces(t *testing.T) {
	ts := httptest.NewServer(newServeHandler(t, Config{}))
	defer ts.Close()

	// SPA at "/" — the embedded placeholder index.html.
	assertSPA(t, ts.URL+"/")
	// SPA deep link is not shadowed by the API / MCP mounts.
	assertSPA(t, ts.URL+"/library/aurora")

	// REST under /api/v1.
	resp, err := http.Get(ts.URL + "/api/v1/color/convert?color=%23000000")
	if err != nil {
		t.Fatalf("API GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("API status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("API Content-Type = %q, want application/json", ct)
	}

	// MCP under /mcp via a real SDK client over the streamable transport.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "serve-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("MCP Connect: %v", err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("MCP ListTools: %v", err)
	}
	if len(list.Tools) == 0 {
		t.Errorf("MCP exposed no tools")
	}
}

// TestDisablesMCPTools confirms MCPDisable reaches the hosted MCP server:
// a disabled tool is absent from tools/list while the rest still register.
// This is the path config.yaml's `mcp:` section drives through serve.
func TestDisablesMCPTools(t *testing.T) {
	ts := httptest.NewServer(newServeHandler(t, Config{MCPDisable: []string{"color.convert"}}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "disable-test", Version: "test"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("MCP Connect: %v", err)
	}
	defer session.Close()

	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("MCP ListTools: %v", err)
	}
	if len(list.Tools) == 0 {
		t.Fatalf("expected the non-disabled tools to still register")
	}
	for _, tool := range list.Tools {
		if tool.Name == "color.convert" {
			t.Errorf("color.convert was disabled but still appears in tools/list")
		}
	}
}

// TestRejectsUnknownDisabledTool confirms a typo in the disable list fails
// BuildHandler loudly rather than silently disabling nothing.
func TestRejectsUnknownDisabledTool(t *testing.T) {
	_, err := BuildHandler(Config{Logger: quietLogger(), MCPDisable: []string{"no.such.tool"}})
	if err == nil {
		t.Fatalf("expected an unknown disabled tool to be rejected")
	}
}

func assertSPA(t *testing.T, url string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%s status = %d, want 200", url, resp.StatusCode)
	}
	if !strings.Contains(string(body), "huetension") {
		t.Errorf("%s body missing SPA marker: %s", url, body)
	}
}

// TestSharedAuthGatesEverySurface confirms one --auth-token covers the
// whole process: every surface 401s without it and answers with it.
func TestSharedAuthGatesEverySurface(t *testing.T) {
	ts := httptest.NewServer(newServeHandler(t, Config{AuthToken: "secret"}))
	defer ts.Close()

	for _, path := range []string{"/", "/api/v1/color/convert?color=%23000000", "/mcp"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s without token: status = %d, want 401", path, resp.StatusCode)
		}
	}

	// REST with the token.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/color/convert?color=%23000000", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authed API GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("authed API status = %d, want 200", resp.StatusCode)
	}

	// MCP with the token injected on every request.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "auth-serve-test", Version: "test"}, nil)
	transport := &sdk.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: bearerInjectingTransport{token: "secret", base: http.DefaultTransport},
		},
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("authed MCP Connect: %v", err)
	}
	defer session.Close()
	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatalf("authed MCP ListTools: %v", err)
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

// TestBuildHandlerRejectsBaseCollision locks in the foot-gun guard: the
// API and MCP prefixes must not equal or nest, or one shadows the other.
func TestBuildHandlerRejectsBaseCollision(t *testing.T) {
	cases := [][2]string{
		{"/api/v1", "/api/v1"}, // equal
		{"/api", "/api/v1"},    // mcp nests under api
		{"/x/y", "/x"},         // api nests under mcp
	}
	for _, c := range cases {
		_, err := BuildHandler(Config{
			Logger:      quietLogger(),
			APIBasePath: c[0],
			MCPBasePath: c[1],
		})
		if err == nil {
			t.Errorf("api=%q mcp=%q: expected collision error, got nil", c[0], c[1])
		}
	}
}

// TestBuildHandlerRejectsNonRootWebBasePath documents the deliberate S9
// limitation — the bundled SPA build assumes a root base.
func TestBuildHandlerRejectsNonRootWebBasePath(t *testing.T) {
	_, err := BuildHandler(Config{Logger: quietLogger(), WebBasePath: "/app"})
	if err == nil {
		t.Fatalf("expected --web-base-path /app to be rejected")
	}
	if !strings.Contains(err.Error(), "web-base-path") {
		t.Errorf("error %q should name --web-base-path", err)
	}
}

// TestBuildHandlerRejectsRootAPIorMCPBasePath: a "/" prefix would shadow
// the SPA entirely.
func TestBuildHandlerRejectsRootBasePaths(t *testing.T) {
	if _, err := BuildHandler(Config{Logger: quietLogger(), APIBasePath: "/"}); err == nil {
		t.Errorf("expected --api-base-path / to be rejected")
	}
	if _, err := BuildHandler(Config{Logger: quietLogger(), MCPBasePath: "/"}); err == nil {
		t.Errorf("expected --mcp-base-path / to be rejected")
	}
}

// TestBuildHandlerRejectsStdioTransport — stdio cannot run on a shared
// HTTP listener.
func TestBuildHandlerRejectsStdioTransport(t *testing.T) {
	_, err := BuildHandler(Config{Logger: quietLogger(), MCPTransports: []string{"stdio"}})
	if err == nil {
		t.Fatalf("expected --mcp-transport stdio to be rejected")
	}
	if !strings.Contains(err.Error(), "stdio") {
		t.Errorf("error %q should name stdio", err)
	}
}

func TestResolveMCPTransports(t *testing.T) {
	ok := []struct {
		in   []string
		want []string
	}{
		{nil, []string{"http"}},
		{[]string{}, []string{"http"}},
		{[]string{"http"}, []string{"http"}},
		{[]string{"sse"}, []string{"sse"}},
		{[]string{"http", "sse", "http"}, []string{"http", "sse"}},
		{[]string{" HTTP ", ""}, []string{"http"}},
	}
	for _, c := range ok {
		got, err := resolveMCPTransports(c.in)
		if err != nil {
			t.Errorf("resolveMCPTransports(%v): unexpected error %v", c.in, err)
			continue
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("resolveMCPTransports(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	for _, bad := range [][]string{{"stdio"}, {"all"}, {"http", "stdio"}, {"grpc"}} {
		if _, err := resolveMCPTransports(bad); err == nil {
			t.Errorf("resolveMCPTransports(%v): expected error, got nil", bad)
		}
	}
}

func TestNormaliseBase(t *testing.T) {
	cases := []struct{ in, def, want string }{
		{"", "/api/v1", "/api/v1"},
		{"  ", "/mcp", "/mcp"},
		{"/mcp", "/x", "/mcp"},
		{"mcp", "/x", "/mcp"},
		{"/mcp/", "/x", "/mcp"},
		{"/", "/x", "/"},
		{"/api/v1/", "/x", "/api/v1"},
	}
	for _, c := range cases {
		if got := normaliseBase(c.in, c.def); got != c.want {
			t.Errorf("normaliseBase(%q, %q) = %q, want %q", c.in, c.def, got, c.want)
		}
	}
}

func TestBasesCollide(t *testing.T) {
	collide := [][2]string{
		{"/api", "/api"},
		{"/api", "/api/v1"},
		{"/api/v1", "/api"},
	}
	for _, c := range collide {
		if !basesCollide(c[0], c[1]) {
			t.Errorf("basesCollide(%q, %q) = false, want true", c[0], c[1])
		}
	}
	clear := [][2]string{
		{"/api/v1", "/mcp"},
		{"/api", "/apix"}, // shared text prefix but not a path-segment prefix
		{"/a", "/b"},
	}
	for _, c := range clear {
		if basesCollide(c[0], c[1]) {
			t.Errorf("basesCollide(%q, %q) = true, want false", c[0], c[1])
		}
	}
}

// TestRunRejectsNonLoopbackWithoutToken mirrors web/mcp: a non-loopback
// bind without an auth token must refuse before opening a listener.
func TestRunRejectsNonLoopbackWithoutToken(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:0", ":0"} {
		err := Run(t.Context(), Config{Address: addr, Logger: quietLogger()})
		if err == nil {
			t.Errorf("%s: expected refusal, got nil", addr)
			continue
		}
		if !strings.Contains(err.Error(), "non-loopback") {
			t.Errorf("%s: error = %v, want a non-loopback message", addr, err)
		}
	}
}

func TestRunRejectsEmptyAddress(t *testing.T) {
	if err := Run(t.Context(), Config{Logger: quietLogger()}); err == nil {
		t.Fatalf("expected empty --address to be rejected")
	}
}
