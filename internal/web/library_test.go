package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/exporter"
	huemcp "github.com/leporel/huetension/internal/mcp"
	"github.com/leporel/huetension/internal/palette/library"
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

// TestLibraryRESTvsMCPColorParity is the cross-transport guarantee
// promised by S3 — REST `/library/palette/{id}` and MCP `library.get`
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
