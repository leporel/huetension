package web

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// lutEnvelope decodes the /lut response — Encoding is set only for the
// HALD PNG binary format.
type lutEnvelope struct {
	Tool   string `json:"tool"`
	Result struct {
		Format   string `json:"format"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		Filename string `json:"filename"`
	} `json:"result"`
}

func TestLUTEndpointCube(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	var env lutEnvelope
	resp := doPOST(t, base, "/api/v1/lut", map[string]any{
		"format":             "cube",
		"colors":             []string{"#ff8800", "#0044aa"},
		"radius":             0.15,
		"distribution":       0.5,
		"intensity":          0.8,
		"blend_neighbors":    1,
		"include_saturation": true,
		"size":               5,
	}, &env)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if env.Tool != "lut.generate" {
		t.Errorf("tool = %q, want lut.generate", env.Tool)
	}
	if env.Result.Format != "cube" {
		t.Errorf("format = %q", env.Result.Format)
	}
	if env.Result.Encoding != "" {
		t.Errorf("text format should not carry an encoding, got %q", env.Result.Encoding)
	}
	if !strings.Contains(env.Result.Content, "LUT_3D_SIZE 5") {
		t.Errorf("missing LUT_3D_SIZE in content: %s", env.Result.Content)
	}
	if !strings.HasSuffix(env.Result.Filename, ".cube") {
		t.Errorf("filename = %q, want *.cube", env.Result.Filename)
	}
}

func TestLUTEndpointHaldPNG(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	var env lutEnvelope
	resp := doPOST(t, base, "/api/v1/lut", map[string]any{
		"format":             "png",
		"colors":             []string{"#ff8800", "#0044aa"},
		"radius":             0.15,
		"distribution":       0.5,
		"intensity":          0.8,
		"blend_neighbors":    1,
		"include_saturation": true,
		"size":               16, // 16 → 4×4 sections of 16×16, 64×64 image
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
	if !strings.Contains(env.Result.Filename, "64x64") {
		t.Errorf("filename = %q, want side dimensions in name", env.Result.Filename)
	}
}

func TestLUTEndpointErrors(t *testing.T) {
	base, teardown := newTestServer(t)
	defer teardown()

	cases := []struct {
		name    string
		payload map[string]any
	}{
		{"no colors", map[string]any{"format": "cube", "colors": []string{}}},
		{"unknown format", map[string]any{"format": "xml", "colors": []string{"#ff0000"}}},
		{"radius negative", map[string]any{
			"format": "cube", "colors": []string{"#ff0000"},
			"radius": -0.1, "distribution": 0.5, "intensity": 0.8,
		}},
		{"distribution > 1", map[string]any{
			"format": "cube", "colors": []string{"#ff0000"},
			"radius": 0.15, "distribution": 1.5, "intensity": 0.8,
		}},
		{"intensity > 1", map[string]any{
			"format": "cube", "colors": []string{"#ff0000"},
			"radius": 0.15, "distribution": 0.5, "intensity": 1.5,
		}},
		{"png with non-perfect-square size", map[string]any{
			"format": "png", "colors": []string{"#ff0000"},
			"radius": 0.15, "distribution": 0.5, "intensity": 0.8,
			"size": 33,
		}},
		{"bad color", map[string]any{
			"format": "cube", "colors": []string{"not-a-color"},
			"radius": 0.15, "distribution": 0.5, "intensity": 0.8,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doPOST(t, base, "/api/v1/lut", tc.payload, nil)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}
