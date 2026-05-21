package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os/exec"
	"testing"
)

type applyLUTEnvelope struct {
	Result struct {
		Results []struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		} `json:"results"`
	} `json:"result"`
}

// makeRedPNG returns a tiny 2×2 red PNG — used as the input image.
func makeRedPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return buf.Bytes()
}

// defaultApplyParams is the minimal valid params payload for the apply
// path — bare-bones LUT (red-only palette, mild grade) plus a small
// cube size so test runs stay fast.
func defaultApplyParams() map[string]any {
	return map[string]any{
		"colors":             []string{"#ff0000"},
		"radius":             0.15,
		"distribution":       0.5,
		"intensity":          0.8,
		"blend_neighbors":    1,
		"include_saturation": true,
		"size":               5,
	}
}

// buildApplyLUTBody builds a multipart body with optional params + a
// list of image parts. Pass nil/empty to omit. Returns body + Content-
// Type header value carrying the boundary.
func buildApplyLUTBody(t *testing.T, params any, images [][]byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	if params != nil {
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		if err := mw.WriteField("params", string(paramsJSON)); err != nil {
			t.Fatalf("write params: %v", err)
		}
	}
	for i, img := range images {
		w, err := mw.CreateFormFile("image", fmt.Sprintf("image_%d.png", i))
		if err != nil {
			t.Fatalf("CreateFormFile(image_%d): %v", i, err)
		}
		if _, err := w.Write(img); err != nil {
			t.Fatalf("write image %d: %v", i, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	return body, mw.FormDataContentType()
}

func doApplyLUT(t *testing.T, base string, params any, images [][]byte) *http.Response {
	t.Helper()
	body, contentType := buildApplyLUTBody(t, params, images)
	resp, err := http.Post(base+"/api/v1/apply-lut", contentType, body)
	if err != nil {
		t.Fatalf("POST /apply-lut: %v", err)
	}
	return resp
}

func TestApplyLUTMissingImage(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	resp := doApplyLUT(t, base, defaultApplyParams(), nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestApplyLUTMissingParams(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	img := makeRedPNG(t)
	resp := doApplyLUT(t, base, nil, [][]byte{img})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestApplyLUTValidationErrors(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()
	img := makeRedPNG(t)

	cases := []struct {
		name   string
		params map[string]any
	}{
		{"no colors", map[string]any{
			"colors": []string{}, "radius": 0.15, "distribution": 0.5, "intensity": 0.8,
		}},
		{"radius negative", map[string]any{
			"colors": []string{"#ff0000"}, "radius": -0.1, "distribution": 0.5, "intensity": 0.8,
		}},
		{"distribution > 1", map[string]any{
			"colors": []string{"#ff0000"}, "radius": 0.15, "distribution": 1.5, "intensity": 0.8,
		}},
		{"bad color", map[string]any{
			"colors": []string{"not-a-color"}, "radius": 0.15, "distribution": 0.5, "intensity": 0.8,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doApplyLUT(t, base, tc.params, [][]byte{img})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want 400; body=%s", resp.StatusCode, body)
			}
		})
	}
}

// TestApplyLUTSuccessBatch runs end-to-end through ffmpeg's lut3d filter
// for two images in a single request. Skipped when ffmpeg is absent.
func TestApplyLUTSuccessBatch(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available; skipping integration test")
	}

	base, teardown := newTestServer(t)
	defer teardown()

	img1 := makeRedPNG(t)
	img2 := makeRedPNG(t)
	resp := doApplyLUT(t, base, defaultApplyParams(), [][]byte{img1, img2})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var env applyLUTEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%s)", err, body)
	}
	if got := len(env.Result.Results); got != 2 {
		t.Fatalf("results length = %d, want 2", got)
	}
	for i, r := range env.Result.Results {
		if r.Encoding != "base64" {
			t.Fatalf("result[%d].encoding = %q, want base64", i, r.Encoding)
		}
		raw, err := base64.StdEncoding.DecodeString(r.Content)
		if err != nil {
			t.Fatalf("result[%d] base64 decode: %v", i, err)
		}
		if !bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
			t.Errorf("result[%d] is not a PNG (magic = % x)", i, raw[:min(8, len(raw))])
		}
	}
}
