package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/exporter"
	huemcp "github.com/leporel/huetension/internal/mcp"
	"github.com/leporel/huetension/internal/palette/library"
	"github.com/leporel/huetension/internal/sandbox"
)

// newLibraryServer is the test-only variant of newTestServer that
// installs a real library index (the embedded defaults). Tests for the
// stateless endpoints don't need this; library.* tests do.
func newLibraryServer(t *testing.T) (string, func()) {
	t.Helper()
	idx := library.MustLoadDefaults()
	cfg := Config{
		Version: "test",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Library: idx,
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	return ts.URL, ts.Close
}

// newWritableLibraryServer is newLibraryServer with a writable
// library.json path, so the POST /library/palette save endpoint is
// enabled. Returns the base URL, the on-disk path, and a teardown func.
func newWritableLibraryServer(t *testing.T) (string, string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.json")
	cfg := Config{
		Version:     "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Library:     library.MustLoadDefaults(),
		LibraryPath: path,
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	return ts.URL, path, ts.Close
}

func TestLibraryIndexEnvelope(t *testing.T) {
	base, stop := newLibraryServer(t)
	defer stop()

	var env struct {
		Schema string `json:"schema"`
		Tool   string `json:"tool"`
		Result struct {
			Categories []struct {
				Slug  string `json:"slug"`
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"categories"`
			Palettes []json.RawMessage `json:"palettes"`
		} `json:"result"`
	}
	resp := doGET(t, base, "/api/v1/library", &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Schema != "huetension/v1" {
		t.Errorf("schema: got %q", env.Schema)
	}
	if env.Tool != "library.index" {
		t.Errorf("tool: got %q", env.Tool)
	}
	if len(env.Result.Categories) < 6 {
		t.Errorf("categories count: got %d, want >=6", len(env.Result.Categories))
	}
	if len(env.Result.Palettes) < 30 {
		t.Errorf("palettes count: got %d, want >=30", len(env.Result.Palettes))
	}
}

func TestLibraryIndexETag(t *testing.T) {
	base, stop := newLibraryServer(t)
	defer stop()

	// First request: 200 carrying an ETag.
	resp1, err := http.Get(base + "/api/v1/library")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request status: %d", resp1.StatusCode)
	}
	etag := resp1.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag header on /library response")
	}

	// Conditional request echoing the ETag: 304, empty body.
	req2, _ := http.NewRequest(http.MethodGet, base+"/api/v1/library", nil)
	req2.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("conditional GET: %v", err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Fatalf("matching If-None-Match: status %d, want 304", resp2.StatusCode)
	}
	if len(body2) != 0 {
		t.Errorf("304 response carried a %d-byte body", len(body2))
	}

	// Stale ETag: full 200 with the catalogue body.
	req3, _ := http.NewRequest(http.MethodGet, base+"/api/v1/library", nil)
	req3.Header.Set("If-None-Match", `"stale-etag"`)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("stale conditional GET: %v", err)
	}
	body3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("stale If-None-Match: status %d, want 200", resp3.StatusCode)
	}
	if len(body3) == 0 {
		t.Error("200 response had an empty body")
	}
}

func TestLibraryIndexETagChangesAfterSave(t *testing.T) {
	base, _, stop := newWritableLibraryServer(t)
	defer stop()

	resp1, err := http.Get(base + "/api/v1/library")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp1.Body.Close()
	etag1 := resp1.Header.Get("ETag")
	if etag1 == "" {
		t.Fatal("no ETag on first /library response")
	}

	// A save changes the catalogue bytes — the ETag must move with them.
	if saveResp := doPOST(t, base, "/api/v1/library/palette", map[string]any{
		"name": "Etag Probe", "colors": []string{"#abcdef"},
	}, nil); saveResp.StatusCode != http.StatusOK {
		t.Fatalf("save: status %d", saveResp.StatusCode)
	}

	// A client holding the pre-save ETag must now revalidate to a full
	// 200, not get a stale 304.
	req, _ := http.NewRequest(http.MethodGet, base+"/api/v1/library", nil)
	req.Header.Set("If-None-Match", etag1)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("conditional GET: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("after save, pre-save ETag: status %d, want 200", resp2.StatusCode)
	}
	if etag2 := resp2.Header.Get("ETag"); etag2 == etag1 {
		t.Errorf("ETag unchanged after save: %q", etag2)
	}
}

func TestLibraryByCategoryAcceptsSlugAndName(t *testing.T) {
	base, stop := newLibraryServer(t)
	defer stop()

	type listEnv struct {
		Result struct {
			Category struct {
				Slug  string `json:"slug"`
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"category"`
			Palettes []json.RawMessage `json:"palettes"`
		} `json:"result"`
	}

	var bySlug listEnv
	respSlug := doGET(t, base, "/api/v1/library/brand-tech", &bySlug)
	if respSlug.StatusCode != http.StatusOK {
		t.Fatalf("slug status: %d", respSlug.StatusCode)
	}
	if bySlug.Result.Category.Slug != "brand-tech" || bySlug.Result.Category.Name != "Brand & Tech" {
		t.Errorf("slug→category: got %+v", bySlug.Result.Category)
	}
	if len(bySlug.Result.Palettes) == 0 {
		t.Error("brand-tech: empty palette list")
	}

	var byName listEnv
	respName := doGET(t, base, "/api/v1/library/Brand%20%26%20Tech", &byName)
	if respName.StatusCode != http.StatusOK {
		t.Fatalf("name status: %d", respName.StatusCode)
	}
	if byName.Result.Category.Slug != "brand-tech" {
		t.Errorf("display-name lookup didn't normalise to slug: %+v", byName.Result.Category)
	}
	if len(byName.Result.Palettes) != len(bySlug.Result.Palettes) {
		t.Errorf("slug vs display-name returned different palette counts: %d vs %d",
			len(bySlug.Result.Palettes), len(byName.Result.Palettes))
	}
}

func TestLibraryByCategoryUnknown404(t *testing.T) {
	base, stop := newLibraryServer(t)
	defer stop()
	resp := doGET(t, base, "/api/v1/library/no-such-category", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestLibraryGetReturnsPalette(t *testing.T) {
	base, stop := newLibraryServer(t)
	defer stop()
	var env struct {
		Result struct {
			Palette struct {
				ID         string               `json:"id"`
				Name       string               `json:"name"`
				Categories []string             `json:"categories"`
				Colors     []exporter.ColorJSON `json:"colors"`
			} `json:"palette"`
		} `json:"result"`
	}
	resp := doGET(t, base, "/api/v1/library/palette/arctic-frost", &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Result.Palette.ID != "arctic-frost" || env.Result.Palette.Name != "Arctic Frost" {
		t.Errorf("palette identity: got %+v", env.Result.Palette)
	}
	if len(env.Result.Palette.Colors) != 4 {
		t.Errorf("arctic-frost color count: got %d, want 4", len(env.Result.Palette.Colors))
	}
}

func TestLibraryGetMissing404(t *testing.T) {
	base, stop := newLibraryServer(t)
	defer stop()
	resp := doGET(t, base, "/api/v1/library/palette/no-such-id", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestLibraryUnavailableWhenNotConfigured(t *testing.T) {
	cfg := Config{Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	for _, p := range []string{"/api/v1/library", "/api/v1/library/brand-tech", "/api/v1/library/palette/arctic-frost"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("%s: status %d, want 503", p, resp.StatusCode)
		}
	}
}

func TestLibrarySaveCreatesPalette(t *testing.T) {
	base, path, stop := newWritableLibraryServer(t)
	defer stop()

	var env struct {
		Tool   string `json:"tool"`
		Result struct {
			Palette struct {
				ID         string               `json:"id"`
				Name       string               `json:"name"`
				Categories []string             `json:"categories"`
				Colors     []exporter.ColorJSON `json:"colors"`
			} `json:"palette"`
		} `json:"result"`
	}
	resp := doPOST(t, base, "/api/v1/library/palette", map[string]any{
		"name":       "My Sunset",
		"colors":     []string{"#ff8800", "#cc4400"},
		"categories": []string{"Warm"},
		"tags":       []string{"demo"},
	}, &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Tool != "library.save" {
		t.Errorf("tool: got %q, want library.save", env.Tool)
	}
	p := env.Result.Palette
	if p.ID != "my-sunset" {
		t.Errorf("generated id: got %q, want my-sunset", p.ID)
	}
	// "Saved" is force-added; the client extra follows it.
	if len(p.Categories) != 2 || p.Categories[0] != "Saved" || p.Categories[1] != "Warm" {
		t.Errorf("categories: got %v, want [Saved Warm]", p.Categories)
	}
	if len(p.Colors) != 2 {
		t.Errorf("colors: got %d, want 2", len(p.Colors))
	}

	// The palette is persisted — it round-trips through a fresh Load.
	reloaded, err := library.Load(path)
	if err != nil {
		t.Fatalf("reload library.json: %v", err)
	}
	if _, ok := reloaded.Get("my-sunset"); !ok {
		t.Error("saved palette not present in persisted library.json")
	}
	// ...and is visible on the same server's GET endpoint right away.
	if got := doGET(t, base, "/api/v1/library/palette/my-sunset", nil); got.StatusCode != http.StatusOK {
		t.Errorf("GET saved palette: status %d, want 200", got.StatusCode)
	}
}

func TestLibrarySaveAssignsUniqueID(t *testing.T) {
	base, _, stop := newWritableLibraryServer(t)
	defer stop()

	idOf := func(name string) string {
		var env struct {
			Result struct {
				Palette struct {
					ID string `json:"id"`
				} `json:"palette"`
			} `json:"result"`
		}
		resp := doPOST(t, base, "/api/v1/library/palette", map[string]any{
			"name": name, "colors": []string{"#101010"},
		}, &env)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("save %q: status %d", name, resp.StatusCode)
		}
		return env.Result.Palette.ID
	}
	if first, second := idOf("Twins"), idOf("Twins"); first != "twins" || second != "twins-2" {
		t.Errorf("ids: got %q, %q; want twins, twins-2", first, second)
	}
}

func TestLibrarySaveRejectsBadInput(t *testing.T) {
	base, _, stop := newWritableLibraryServer(t)
	defer stop()

	cases := []struct {
		name    string
		payload map[string]any
	}{
		{"no name", map[string]any{"colors": []string{"#000000"}}},
		{"no colors", map[string]any{"name": "Empty"}},
		{"bad color", map[string]any{"name": "Bad", "colors": []string{"definitely-not-a-color"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := doPOST(t, base, "/api/v1/library/palette", c.payload, nil)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status: got %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestLibrarySaveUnavailableWithoutPath(t *testing.T) {
	// newLibraryServer configures a catalogue but no LibraryPath, so the
	// read endpoints work while saving is disabled.
	base, stop := newLibraryServer(t)
	defer stop()
	resp := doPOST(t, base, "/api/v1/library/palette", map[string]any{
		"name": "Nope", "colors": []string{"#000000"},
	}, nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", resp.StatusCode)
	}
}

func TestLibrarySaveForbiddenWhenReadOnly(t *testing.T) {
	cfg := Config{
		Version:     "test",
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Library:     library.MustLoadDefaults(),
		LibraryPath: filepath.Join(t.TempDir(), "library.json"),
		Sandbox:     sandbox.ImageSandbox{ReadOnly: true},
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()

	resp := doPOST(t, ts.URL, "/api/v1/library/palette", map[string]any{
		"name": "Nope", "colors": []string{"#000000"},
	}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", resp.StatusCode)
	}
}

// TestLibraryRESTvsMCPColorParity is the cross-transport guarantee
// REST `/library/palette/{id}` and MCP `library.get`
// must produce byte-identical `colors` arrays for the same id, run
// against the same Index. By construction both sides funnel through
// exporter.EncodeColors after library.Palette.ToPalette, so any drift
// here is a real regression in one of the two encoders.
func TestLibraryRESTvsMCPColorParity(t *testing.T) {
	idx := library.MustLoadDefaults()
	all := idx.All()
	if len(all) == 0 {
		t.Fatal("no palettes")
	}
	ids := []string{"arctic-frost", "tailwind-slate", "high-contrast-ink"}

	// REST side.
	cfg := Config{Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Library: idx}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()

	// MCP side. We build an MCP server with the library tools enabled
	// and drive it via in-memory transport, then JSON-decode the
	// structured content of the library.get response.
	mcpCfg := huemcp.Config{
		Version: "test",
		Library: idx,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv, _, err := huemcp.Build(mcpCfg)
	if err != nil {
		t.Fatalf("mcp Build: %v", err)
	}

	ctx := context.Background()
	clientT, serverT := sdk.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverT) }()

	client := sdk.NewClient(&sdk.Implementation{Name: "parity-test", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			// REST colors[]
			var restEnv struct {
				Result struct {
					Palette struct {
						Colors []exporter.ColorJSON `json:"colors"`
					} `json:"palette"`
				} `json:"result"`
			}
			doGET(t, ts.URL, "/api/v1/library/palette/"+id, &restEnv)
			restBytes, err := json.Marshal(restEnv.Result.Palette.Colors)
			if err != nil {
				t.Fatalf("marshal rest colors: %v", err)
			}

			// MCP colors[]
			res, err := cs.CallTool(ctx, &sdk.CallToolParams{
				Name:      "library.get",
				Arguments: map[string]any{"id": id},
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Fatalf("CallTool error: %v", res.Content)
			}
			structured, ok := res.StructuredContent.(map[string]any)
			if !ok {
				t.Fatalf("unexpected StructuredContent type %T", res.StructuredContent)
			}
			result, _ := structured["result"].(map[string]any)
			pal, _ := result["palette"].(map[string]any)
			rawColors, _ := pal["colors"].([]any)
			mcpBytes, err := json.Marshal(rawColors)
			if err != nil {
				t.Fatalf("marshal mcp colors: %v", err)
			}

			// Both sides went through the same encoder; for parity we
			// re-decode and re-marshal each side into the same Go type
			// so map-key ordering can't account for any apparent
			// difference. The decisive check is that the round-tripped
			// shapes match field-for-field.
			var restColors, mcpColors []exporter.ColorJSON
			if err := json.Unmarshal(restBytes, &restColors); err != nil {
				t.Fatalf("rest re-decode: %v", err)
			}
			if err := json.Unmarshal(mcpBytes, &mcpColors); err != nil {
				t.Fatalf("mcp re-decode: %v", err)
			}
			if len(restColors) != len(mcpColors) {
				t.Fatalf("color count mismatch: rest=%d mcp=%d", len(restColors), len(mcpColors))
			}
			for i := range restColors {
				rb, _ := json.Marshal(restColors[i])
				mb, _ := json.Marshal(mcpColors[i])
				if string(rb) != string(mb) {
					t.Errorf("color[%d] mismatch:\n  rest=%s\n   mcp=%s", i, rb, mb)
				}
			}
		})
	}
}
