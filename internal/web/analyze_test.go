package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/sandbox"
)

// analyzeResponse is the slice of the envelope these tests care about:
// the strips array carrying base64 PNG content per metric.
type analyzeResponse struct {
	Schema string `json:"schema"`
	Tool   string `json:"tool"`
	Result struct {
		Strips []struct {
			Metric   string `json:"metric"`
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		} `json:"strips"`
	} `json:"result"`
}

func decodeAnalyze(t *testing.T, resp *http.Response) analyzeResponse {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env analyzeResponse
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode (body=%s): %v", body, err)
	}
	return env
}

// assertFourPNGStrips checks the wire-side strip list contains the four
// metrics in canonical order and each `content` field decodes to a valid
// PNG. Used by every happy-path test below.
func assertFourPNGStrips(t *testing.T, env analyzeResponse) {
	t.Helper()
	want := []string{"hue", "luminance", "saturation", "distance"}
	if len(env.Result.Strips) != 4 {
		t.Fatalf("strips len = %d, want 4", len(env.Result.Strips))
	}
	for i, s := range env.Result.Strips {
		if s.Metric != want[i] {
			t.Errorf("strip[%d].metric = %q, want %q", i, s.Metric, want[i])
		}
		if s.Encoding != "base64" {
			t.Errorf("strip[%d].encoding = %q, want base64", i, s.Encoding)
		}
		raw, err := base64.StdEncoding.DecodeString(s.Content)
		if err != nil {
			t.Errorf("strip[%d]: base64 decode: %v", i, err)
			continue
		}
		if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
			t.Errorf("strip[%d]: png decode: %v", i, err)
		}
	}
}

func TestAnalyzeMultipartHappyPath(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	body, ct := buildMultipartBody(t, makeTinyPNG(t), map[string]string{
		"distance_target": "red",
	})
	resp, err := http.Post(base+"/api/v1/analyze", ct, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}

	env := decodeAnalyze(t, resp)
	if env.Schema != "huetension/v1" {
		t.Errorf("schema = %q", env.Schema)
	}
	if env.Tool != "image.analyze" {
		t.Errorf("tool = %q", env.Tool)
	}
	assertFourPNGStrips(t, env)
}

func TestAnalyzeJSONDataURI(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(makeTinyPNG(t))
	body, _ := json.Marshal(map[string]any{"data": dataURI})

	resp, err := http.Post(base+"/api/v1/analyze", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}
	assertFourPNGStrips(t, decodeAnalyze(t, resp))
}

func TestAnalyzeBadDistanceTarget(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(makeTinyPNG(t))
	body, _ := json.Marshal(map[string]any{
		"data":            dataURI,
		"distance_target": "purple",
	})
	resp, err := http.Post(base+"/api/v1/analyze", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (want 400), body = %s", resp.StatusCode, raw)
	}
}

func TestAnalyzeEmptyJSONBody(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	resp, err := http.Post(base+"/api/v1/analyze", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAnalyzeBadContentType(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/analyze", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", resp.StatusCode)
	}
}

// TestAnalyzeSpaceTakesEffectMultipart locks in that the multipart code
// path actually forwards `space` to analyze.Strips — a regression here
// would silently default to OkLCH and the UI selector would do nothing.
func TestAnalyzeSpaceTakesEffectMultipart(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	pngBytes := makeTinyPNG(t)

	post := func(space string) [][]byte {
		body, ct := buildMultipartBody(t, pngBytes, map[string]string{
			"space": space,
		})
		resp, err := http.Post(base+"/api/v1/analyze", ct, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
		}
		env := decodeAnalyze(t, resp)
		out := make([][]byte, len(env.Result.Strips))
		for i, s := range env.Result.Strips {
			raw, err := base64.StdEncoding.DecodeString(s.Content)
			if err != nil {
				t.Fatalf("base64 decode strip %d: %v", i, err)
			}
			out[i] = raw
		}
		return out
	}

	oklch := post("oklch")
	hsl := post("hsl")

	// At least one of hue/luminance/saturation must differ; the distance
	// strip is space-independent and must stay byte-identical.
	diff := false
	for i := 0; i < 3; i++ {
		if !bytes.Equal(oklch[i], hsl[i]) {
			diff = true
			break
		}
	}
	if !diff {
		t.Errorf("space=hsl produced byte-identical h/l/s strips to space=oklch — wire field not propagated")
	}
	if !bytes.Equal(oklch[3], hsl[3]) {
		t.Errorf("distance strip differs across spaces — should not depend on space")
	}
}

func TestAnalyzeMissingMultipartFile(t *testing.T) {
	base, teardown := newExtractTestServer(t, sandbox.ImageSandbox{})
	defer teardown()

	var buf bytes.Buffer
	resp, err := http.Post(base+"/api/v1/analyze", "multipart/form-data; boundary=xxx", &buf)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
