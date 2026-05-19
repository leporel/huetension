package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	stdcolor "image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/sandbox"
)

// newExtractTestServer builds the web handler with a sandbox config of
// the caller's choosing. Returns the started test server (URL +
// teardown) so handlers and middleware run end-to-end, the same way
// they would in production.
func newExtractTestServer(t *testing.T, sb sandbox.ImageSandbox) (string, func()) {
	t.Helper()
	cfg := Config{
		Version: "test",
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Sandbox: sb,
	}
	h, err := BuildHandler(cfg)
	if err != nil {
		t.Fatalf("BuildHandler: %v", err)
	}
	ts := httptest.NewServer(h)
	return ts.URL, ts.Close
}

// makeTinyPNG returns a deterministic 4×4 checkerboard PNG (red / blue).
// Bytes alone are enough — extract decodes from raw — but tests that
// want a URL source serve the same bytes through an httptest server.
func makeTinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := stdcolor.RGBA{R: 255, A: 255}
	blue := stdcolor.RGBA{B: 255, A: 255}
	for y := range 4 {
		for x := range 4 {
			if (x+y)%2 == 0 {
				img.Set(x, y, red)
			} else {
				img.Set(x, y, blue)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// buildMultipartBody packs body bytes as a `image` file part plus the
// option form fields, returning the encoded body and its Content-Type
// header (with the multipart boundary).
func buildMultipartBody(t *testing.T, body []byte, fields map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(extractFileFieldName, "tiny.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatalf("write part: %v", err)
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %s: %v", k, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	return buf.Bytes(), mw.FormDataContentType()
}

// extractResponse is the slice of the envelope these tests care about:
// schema/tool plus the palette colors (with source pin coords).
type extractResponse struct {
	Schema string `json:"schema"`
	Tool   string `json:"tool"`
	Result struct {
		Palette struct {
			Size   int                  `json:"size"`
			Colors []exporter.ColorJSON `json:"colors"`
		} `json:"palette"`
	} `json:"result"`
}

func decodeExtract(t *testing.T, resp *http.Response) extractResponse {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env extractResponse
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode (body=%s): %v", body, err)
	}
	return env
}

// TestExtractMultipartHappyPath drives the SPA's expected upload path:
// multipart/form-data with an `image` file field and option form
// fields. Asserts a successful 200 + populated palette + every color
// carries a source pin (the pin-overlay contract).
func TestExtractMultipartHappyPath(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	body, ct := buildMultipartBody(t, makeTinyPNG(t), map[string]string{
		"size":   "2",
		"resize": "0", // tiny enough; skip resize
	})
	resp, err := http.Post(base+"/api/v1/extract", ct, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}

	env := decodeExtract(t, resp)
	if env.Schema != "huetension/v1" {
		t.Errorf("schema = %q", env.Schema)
	}
	if env.Tool != "image.extract" {
		t.Errorf("tool = %q", env.Tool)
	}
	if env.Result.Palette.Size == 0 {
		t.Fatalf("empty palette")
	}
	for i, c := range env.Result.Palette.Colors {
		if c.Source == nil {
			t.Errorf("color[%d] %s: missing source pin", i, c.Hex)
			continue
		}
		if c.Source.X < 0 || c.Source.X > 1 || c.Source.Y < 0 || c.Source.Y > 1 {
			t.Errorf("color[%d] source out of range: %+v", i, c.Source)
		}
	}
}

// TestExtractJSONDataURI exercises the JSON-body path with a base64
// data URI — what the Web SPA will fall back to for "paste this base64
// image" UX, and what MCP clients use.
func TestExtractJSONDataURI(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	pngBytes := makeTinyPNG(t)
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	body, _ := json.Marshal(map[string]any{
		"data":   dataURI,
		"size":   2,
		"resize": 0,
	})
	resp, err := http.Post(base+"/api/v1/extract", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}
	env := decodeExtract(t, resp)
	if env.Result.Palette.Size == 0 {
		t.Errorf("empty palette")
	}
}

// TestExtractJSONURL spins up an httptest server hosting the tiny PNG,
// then asks /extract to fetch it. Exercises the URL → sandbox-aware
// loader → extract path end-to-end.
func TestExtractJSONURL(t *testing.T) {
	pngBytes := makeTinyPNG(t)
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer imgSrv.Close()

	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	body, _ := json.Marshal(map[string]any{
		"url":    imgSrv.URL + "/tiny.png",
		"size":   2,
		"resize": 0,
	})
	resp, err := http.Post(base+"/api/v1/extract", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}
	env := decodeExtract(t, resp)
	if env.Result.Palette.Size == 0 {
		t.Errorf("empty palette")
	}
}

// TestExtractSandboxRejectsHost confirms AllowHosts cuts off URL inputs
// to non-listed hosts before any fetch happens — the security boundary
// that lets operators expose /extract publicly without becoming a
// generic image proxy.
func TestExtractSandboxRejectsHost(t *testing.T) {
	pngBytes := makeTinyPNG(t)
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(pngBytes)
	}))
	defer imgSrv.Close()

	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{
		AllowHosts: []string{"only.example.com"},
	})
	defer teardown()

	body, _ := json.Marshal(map[string]any{"url": imgSrv.URL + "/tiny.png"})
	resp, err := http.Post(base+"/api/v1/extract", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (want 400), body = %s", resp.StatusCode, raw)
	}
}

// TestExtractSandboxRejectsOversizedDataURI verifies MaxImageBytes is
// enforced on decoded base64 payloads — a small cap rejects even a
// tiny PNG because PNG bytes (~70+) exceed the cap.
func TestExtractSandboxRejectsOversizedDataURI(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{
		MaxImageBytes: 10,
	})
	defer teardown()

	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(makeTinyPNG(t))
	body, _ := json.Marshal(map[string]any{"data": dataURI})
	resp, err := http.Post(base+"/api/v1/extract", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (want 400), body = %s", resp.StatusCode, raw)
	}
}

// TestExtractBadContentType makes sure we reject unsupported request
// shapes with 415 instead of trying to parse them as JSON / multipart.
func TestExtractBadContentType(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/extract", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", resp.StatusCode)
	}
}

// TestExtractEmptyJSONBody locks in the validation: a JSON body with
// neither url nor data must come back as 400 with a clear message,
// not a 500 / panic.
func TestExtractEmptyJSONBody(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	resp, err := http.Post(base+"/api/v1/extract", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestExtractMissingMultipartFile sanity-checks the multipart path:
// a request with the right Content-Type but no `image` file part
// returns 400, not 500.
func TestExtractMissingMultipartFile(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("size", "5")
	_ = mw.Close()

	resp, err := http.Post(base+"/api/v1/extract", mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
