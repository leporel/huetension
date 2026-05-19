package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/harmony"
	pcolor "github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// newTestServer returns a started httptest.Server fronting the web
// handler chain (no auth, no CORS, logger discarded). Callers receive
// the base URL and a teardown func.
func newTestServer(t *testing.T) (string, func()) {
	t.Helper()
	cfg := Config{
		Version: "test",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	return ts.URL, ts.Close
}

// doGET issues a GET against the test server and decodes the response
// body into v. Fails the test on transport or decode errors.
func doGET(t *testing.T, base, path string, v any) *http.Response {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if v != nil {
		if err := json.Unmarshal(body, v); err != nil {
			t.Fatalf("decode %s (body=%s): %v", path, body, err)
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp
}

// doPOST issues a JSON POST against the test server.
func doPOST(t *testing.T, base, path string, payload any, v any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(base+path, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if v != nil {
		if err := json.Unmarshal(respBody, v); err != nil {
			t.Fatalf("decode %s (body=%s): %v", path, respBody, err)
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	return resp
}

// TestColorConvertEndpoint exercises the convert handler in both
// to=all (formats map) and to=<single> (value) modes.
func TestColorConvertEndpoint(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	var env struct {
		Schema string `json:"schema"`
		Tool   string `json:"tool"`
		Params struct {
			Color string `json:"color"`
			To    string `json:"to"`
		} `json:"params"`
		Result struct {
			Input   string            `json:"input"`
			Hex     string            `json:"hex"`
			Formats map[string]string `json:"formats"`
			Value   string            `json:"value"`
		} `json:"result"`
	}
	resp := doGET(t, base, "/api/v1/color/convert?color=%23336699", &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Schema != "huetension/v1" {
		t.Errorf("schema = %q", env.Schema)
	}
	if env.Tool != "color.convert" {
		t.Errorf("tool = %q", env.Tool)
	}
	if env.Result.Hex != "#336699" {
		t.Errorf("hex = %q", env.Result.Hex)
	}
	if len(env.Result.Formats) == 0 {
		t.Errorf("expected formats map populated for to=all")
	}

	// to=hex narrows the response to a single value. Fresh struct so
	// the prior decode's Formats map does not bleed into the assertion
	// (encoding/json does not zero fields absent from the new body).
	var narrow struct {
		Result struct {
			Value   string            `json:"value"`
			Formats map[string]string `json:"formats"`
		} `json:"result"`
	}
	resp = doGET(t, base, "/api/v1/color/convert?color=royalblue&to=hex", &narrow)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if narrow.Result.Value == "" {
		t.Errorf("expected value populated for to=hex")
	}
	if narrow.Result.Formats != nil {
		t.Errorf("expected formats nil for to=hex, got %v", narrow.Result.Formats)
	}
}

// TestColorSortPaletteParity asserts that the JSON the sort handler
// emits is byte-identical to exporter.Export(samePalette, FormatJSON,
// sameOpts). This is the canonical-envelope contract — if the bytes
// match, every JSON consumer sees the same shape the CLI does.
func TestColorSortPaletteParity(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	colors := []string{"#ff0000", "#00ff00", "#0000ff"}
	resp := doPOST(t, base, "/api/v1/color/sort", map[string]any{
		"colors":  colors,
		"by":      "luminance",
		"reverse": false,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	gotBytes, _ := io.ReadAll(resp.Body)

	// Reconstruct what the handler produced and call exporter.Export
	// with the same Tool/Params.
	cs, err := parseColorList(colors)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pal := palette.New(cs)
	if err := pal.Sort(palette.SortByLuminance, false); err != nil {
		t.Fatalf("sort: %v", err)
	}
	wantBytes, err := exporter.Export(pal, exporter.FormatJSON, exporter.Options{
		Tool: "color.sort",
		Params: map[string]any{
			"colors":  colors,
			"by":      "luminance",
			"reverse": false,
		},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if string(bytes.TrimSpace(gotBytes)) != string(bytes.TrimSpace(wantBytes)) {
		t.Errorf("envelope mismatch.\ngot:  %s\nwant: %s", gotBytes, wantBytes)
	}
}

// TestHarmonyEndpointShape covers the typed envelope: harmony type
// resolves through resolveHarmonyType, path params work, palette has
// expected size for complementary (2 colors).
func TestHarmonyEndpointShape(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	var env struct {
		Schema string `json:"schema"`
		Tool   string `json:"tool"`
		Result struct {
			Palette struct {
				Size   int                  `json:"size"`
				Name   string               `json:"name"`
				Colors []exporter.ColorJSON `json:"colors"`
			} `json:"palette"`
		} `json:"result"`
	}
	resp := doGET(t, base, "/api/v1/harmony/complementary/%23ff0000", &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Tool != "harmony.generate" {
		t.Errorf("tool = %q", env.Tool)
	}
	if env.Result.Palette.Size != 2 {
		t.Errorf("complementary size = %d, want 2", env.Result.Palette.Size)
	}
	if env.Result.Palette.Name != string(harmony.Complementary) {
		t.Errorf("name = %q", env.Result.Palette.Name)
	}
}

// TestHarmonyUnknownType verifies typos surface as 400, not 500 or
// silent empty palettes.
func TestHarmonyUnknownType(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	resp, err := http.Get(base + "/api/v1/harmony/no-such-type/%23ff0000")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestGradientFromTo and TestGradientStops cover the two input modes
// for /api/v1/gradient.
func TestGradientFromTo(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	var env struct {
		Tool   string `json:"tool"`
		Result struct {
			Palette struct {
				Size int `json:"size"`
			} `json:"palette"`
		} `json:"result"`
	}
	resp := doGET(t, base, "/api/v1/gradient?from=%23ff0000&to=%230000ff&steps=5", &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Tool != "gradient.generate" {
		t.Errorf("tool = %q", env.Tool)
	}
	if env.Result.Palette.Size != 5 {
		t.Errorf("size = %d, want 5", env.Result.Palette.Size)
	}
}

func TestGradientStops(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	var env struct {
		Result struct {
			Palette struct {
				Size int `json:"size"`
			} `json:"palette"`
		} `json:"result"`
	}
	resp := doGET(t, base, "/api/v1/gradient?stops=%23ff0000,%2300ff00,%230000ff&steps=7", &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Result.Palette.Size != 7 {
		t.Errorf("size = %d, want 7", env.Result.Palette.Size)
	}
}

// TestGradientPositions covers the optional positions= param: a valid
// positioned request succeeds, a count mismatch is a 400.
func TestGradientPositions(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	var env struct {
		Result struct {
			Palette struct {
				Size int `json:"size"`
			} `json:"palette"`
		} `json:"result"`
	}
	resp := doGET(t, base,
		"/api/v1/gradient?stops=%23000000,%23ff0000,%23ffffff&positions=0,0.2,1&steps=7", &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Result.Palette.Size != 7 {
		t.Errorf("size = %d, want 7", env.Result.Palette.Size)
	}

	// positions count must match stops count → 400, not 500.
	bad := doGET(t, base,
		"/api/v1/gradient?stops=%23000000,%23ffffff&positions=0,0.5,1&steps=5", nil)
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("mismatched positions: status = %d, want 400", bad.StatusCode)
	}
}

// TestContrastEndpoint exercises wcag21 (default), apca, and both modes.
func TestContrastEndpoint(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	for _, algo := range []string{"", "wcag21", "apca", "both"} {
		var env struct {
			Tool   string `json:"tool"`
			Result struct {
				WCAG21 map[string]any `json:"wcag21"`
				APCA   map[string]any `json:"apca"`
			} `json:"result"`
		}
		q := "?fg=%23ffffff&bg=%23000000"
		if algo != "" {
			q += "&algo=" + algo
		}
		resp := doGET(t, base, "/api/v1/contrast"+q, &env)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("algo=%q status: %d", algo, resp.StatusCode)
		}
		if env.Tool != "contrast.check" {
			t.Errorf("tool = %q", env.Tool)
		}
		switch algo {
		case "", "wcag21":
			if env.Result.WCAG21 == nil {
				t.Errorf("algo=%q expected WCAG21", algo)
			}
			if env.Result.APCA != nil {
				t.Errorf("algo=%q did not expect APCA", algo)
			}
		case "apca":
			if env.Result.APCA == nil {
				t.Errorf("algo=apca expected APCA")
			}
			if env.Result.WCAG21 != nil {
				t.Errorf("algo=apca did not expect WCAG21")
			}
		case "both":
			if env.Result.WCAG21 == nil || env.Result.APCA == nil {
				t.Errorf("algo=both expected both fields")
			}
		}
	}
}

// TestPaletteRandomDeterminism asserts the seed reproducibility
// contract — same seed + same count produces the same palette bytes
// across the HTTP layer and the library.
func TestPaletteRandomDeterminism(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	var first, second struct {
		Result struct {
			Palette struct {
				Colors []exporter.ColorJSON `json:"colors"`
			} `json:"palette"`
		} `json:"result"`
	}
	doGET(t, base, "/api/v1/random?count=5&seed=42", &first)
	doGET(t, base, "/api/v1/random?count=5&seed=42", &second)
	if len(first.Result.Palette.Colors) != 5 || len(second.Result.Palette.Colors) != 5 {
		t.Fatalf("unexpected palette sizes: %d, %d", len(first.Result.Palette.Colors), len(second.Result.Palette.Colors))
	}
	for i := range first.Result.Palette.Colors {
		if first.Result.Palette.Colors[i].Hex != second.Result.Palette.Colors[i].Hex {
			t.Errorf("color[%d] not reproducible: %q vs %q", i, first.Result.Palette.Colors[i].Hex, second.Result.Palette.Colors[i].Hex)
		}
	}
}

// exportEnvelope decodes the /export response — Encoding is set only for
// binary formats.
type exportEnvelope struct {
	Tool   string `json:"tool"`
	Result struct {
		Format   string `json:"format"`
		Kind     string `json:"kind"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		Filename string `json:"filename"`
	} `json:"result"`
}

// TestExportEndpointText exercises the general /export route for a text
// format, including the CSS `kind` dialect refinement.
func TestExportEndpointText(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	var env exportEnvelope
	resp := doPOST(t, base, "/api/v1/export", map[string]any{
		"format": "css",
		"colors": []string{"#ff0000", "#00ff00"},
		"name":   "brand",
		"kind":   "scss",
	}, &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Tool != "export" {
		t.Errorf("tool = %q", env.Tool)
	}
	// `kind: scss` refines `format: css` → the SCSS renderer.
	if env.Result.Format != "scss" || env.Result.Kind != "scss" {
		t.Errorf("format/kind = %q/%q, want scss/scss", env.Result.Format, env.Result.Kind)
	}
	if env.Result.Encoding != "" {
		t.Errorf("text format should not carry an encoding, got %q", env.Result.Encoding)
	}
	if !strings.Contains(env.Result.Content, "$brand-1") {
		t.Errorf("content missing $brand-1: %s", env.Result.Content)
	}
	if env.Result.Filename != "brand.scss" {
		t.Errorf("filename = %q", env.Result.Filename)
	}
}

// TestExportEndpointBinary checks that PNG output is base64-encoded and
// round-trips back to a valid PNG.
func TestExportEndpointBinary(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	var env exportEnvelope
	resp := doPOST(t, base, "/api/v1/export", map[string]any{
		"format": "png",
		"colors": []string{"#ff0000", "#00ff00", "#0000ff"},
		"name":   "swatch",
	}, &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Result.Encoding != "base64" {
		t.Fatalf("encoding = %q, want base64", env.Result.Encoding)
	}
	raw, err := base64.StdEncoding.DecodeString(env.Result.Content)
	if err != nil {
		t.Fatalf("content is not valid base64: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
		t.Errorf("decoded content is not a PNG (magic = % x)", raw[:min(8, len(raw))])
	}
	if env.Result.Filename != "swatch.png" {
		t.Errorf("filename = %q", env.Result.Filename)
	}
}

// TestExportEndpointTailwindShades confirms the shades control reaches the
// tailwind renderer.
func TestExportEndpointTailwindShades(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	var env exportEnvelope
	resp := doPOST(t, base, "/api/v1/export", map[string]any{
		"format": "tailwind",
		"colors": []string{"#ff0000"},
		"name":   "brand",
		"shades": 5,
	}, &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	// A 5-shade scale expands one input into 100/300/500/700/900.
	if !strings.Contains(env.Result.Content, "500") {
		t.Errorf("expected a shade-scale key, got: %s", env.Result.Content)
	}
}

// TestExportEndpointErrors covers the request-validation 400s.
func TestExportEndpointErrors(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	cases := []struct {
		name    string
		payload map[string]any
	}{
		{"no colors", map[string]any{"format": "css", "colors": []string{}}},
		{"missing format", map[string]any{"colors": []string{"#ff0000"}}},
		{"unknown format", map[string]any{"format": "xml", "colors": []string{"#ff0000"}}},
		{"bad color", map[string]any{"format": "css", "colors": []string{"not-a-color"}}},
		{"negative shades", map[string]any{"format": "tailwind", "colors": []string{"#ff0000"}, "shades": -1}},
		{"kind on non-css", map[string]any{"format": "png", "colors": []string{"#ff0000"}, "kind": "scss"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doPOST(t, base, "/api/v1/export", tc.payload, nil)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestBlindnessSimulateAll(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	var env struct {
		Tool   string `json:"tool"`
		Result struct {
			Variants []struct {
				Kind   string               `json:"kind"`
				Colors []exporter.ColorJSON `json:"colors"`
			} `json:"variants"`
		} `json:"result"`
	}
	resp := doPOST(t, base, "/api/v1/blindness/simulate", map[string]any{
		"colors": []string{"#ff0000", "#00ff00", "#0000ff"},
		// kind omitted → "all"
	}, &env)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Tool != "blindness.simulate" {
		t.Errorf("tool = %q", env.Tool)
	}
	// AllKinds currently lists protan, deutan, tritan, achroma.
	if len(env.Result.Variants) < 4 {
		t.Errorf("variants count = %d, want ≥ 4", len(env.Result.Variants))
	}
	for _, v := range env.Result.Variants {
		if len(v.Colors) != 3 {
			t.Errorf("variant %q has %d colors, want 3", v.Kind, len(v.Colors))
		}
	}
}

// TestBadRequests sweeps the missing-required-parameter case across
// the GET endpoints — they should all answer 400, not 500.
func TestBadRequests(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	cases := []string{
		"/api/v1/color/convert",                                // missing color
		"/api/v1/color/convert?color=not-a-color",              // bad color
		"/api/v1/gradient?steps=5",                             // no from/to/stops
		"/api/v1/gradient?from=%23ff0000&to=%2300ff00",         // missing steps
		"/api/v1/contrast?fg=%23ffffff",                        // missing bg
		"/api/v1/contrast?fg=%23ffffff&bg=%23000000&algo=xxx",  // unknown algo
		"/api/v1/harmony/complementary/not-a-color",            // bad base color
	}
	for _, path := range cases {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", path, resp.StatusCode)
		}
	}
}

// TestColorParseSmoke uses the color package directly to confirm the
// test fixtures are sane. Guards against future color-parser regressions
// from masking endpoint issues here.
func TestColorParseSmoke(t *testing.T) {
	if _, err := pcolor.Parse("#336699"); err != nil {
		t.Fatalf("parse #336699: %v", err)
	}
}
