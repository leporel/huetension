package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	colorlib "image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/sandbox"
)

// makeTinyPNG generates a deterministic 4x4 PNG with two distinct colors.
// Returns both the encoded bytes and a temp-file path inside dir.
func makeTinyPNG(t *testing.T, dir string) (string, []byte) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := colorlib.RGBA{R: 255, A: 255}
	blue := colorlib.RGBA{B: 255, A: 255}
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
	if dir == "" {
		return "", buf.Bytes()
	}
	path := filepath.Join(dir, "tiny.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	return path, buf.Bytes()
}

func TestImageExtractFromData(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	encoded := base64.StdEncoding.EncodeToString(data)
	_, out, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data:   encoded,
		Method: "kmeans",
		Size:   2,
	}, ImageSandbox{})
	if err != nil {
		t.Fatalf("handleImageExtract: %v", err)
	}
	if out.Result.Size == 0 {
		t.Fatalf("empty palette")
	}
	if out.Tool != "image.extract" {
		t.Errorf("tool = %q", out.Tool)
	}
	if out.Schema != schemaVersion {
		t.Errorf("schema = %q", out.Schema)
	}
}

func TestImageExtractFromDataURI(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data:   uri,
		Method: "kmeans",
		Size:   2,
	}, ImageSandbox{})
	if err != nil {
		t.Fatalf("handleImageExtract: %v", err)
	}
}

func TestImageExtractFromPathSandboxed(t *testing.T) {
	dir := t.TempDir()
	path, _ := makeTinyPNG(t, dir)

	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Path:   path,
		Method: "kmeans",
		Size:   2,
	}, ImageSandbox{Root: dir})
	if err != nil {
		t.Fatalf("handleImageExtract: %v", err)
	}
}

func TestImageExtractRejectsPathOutsideRoot(t *testing.T) {
	outsideDir := t.TempDir()
	insideDir := t.TempDir()
	path, _ := makeTinyPNG(t, outsideDir)

	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Path: path,
	}, ImageSandbox{Root: insideDir})
	if err == nil {
		t.Fatalf("expected sandbox rejection")
	}
	if !strings.Contains(err.Error(), "escapes root") {
		t.Errorf("error %q should mention escape", err)
	}
}

func TestImageExtractRejectsPathInReadOnlyMode(t *testing.T) {
	dir := t.TempDir()
	path, _ := makeTinyPNG(t, dir)

	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Path: path,
	}, ImageSandbox{ReadOnly: true, Root: dir})
	if err == nil {
		t.Fatalf("expected read-only rejection")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error %q should mention read-only", err)
	}
}

func TestImageExtractRejectsPathWhenRootEmpty(t *testing.T) {
	dir := t.TempDir()
	path, _ := makeTinyPNG(t, dir)

	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Path: path,
	}, ImageSandbox{Root: ""})
	if err == nil {
		t.Fatalf("expected fs-disabled rejection")
	}
	if !strings.Contains(err.Error(), "filesystem access disabled") {
		t.Errorf("error %q should mention disabled fs", err)
	}
}

func TestImageExtractRejectsMultipleSources(t *testing.T) {
	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Path: "x.png",
		URL:  "https://example.com/y.png",
	}, ImageSandbox{Root: "."})
	if err == nil {
		t.Fatalf("expected mutual-exclusion rejection")
	}
}

func TestImageExtractRejectsNoSource(t *testing.T) {
	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{}, ImageSandbox{})
	if err == nil {
		t.Fatalf("expected missing-source rejection")
	}
}

func TestImageExtractRejectsBadBase64(t *testing.T) {
	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data: "not-base64!!!",
	}, ImageSandbox{})
	if err == nil {
		t.Fatalf("expected base64 decode error")
	}
}

func TestImageExtractDataExceedsLimit(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	encoded := base64.StdEncoding.EncodeToString(data)
	// Set MaxImageBytes lower than the actual PNG size.
	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data: encoded,
	}, ImageSandbox{MaxImageBytes: 10})
	if err == nil {
		t.Fatalf("expected size-limit rejection")
	}
}

func TestImageExtractBatchMixedSandbox(t *testing.T) {
	dir := t.TempDir()
	path, data := makeTinyPNG(t, dir)
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)

	// One inside-root path + one data URI + one bogus path.
	_, out, err := handleImageExtractBatch(context.Background(), ImageExtractBatchParams{
		Sources: []string{path, uri, filepath.Join(t.TempDir(), "missing.png")},
		Method:  "kmeans",
		Size:    2,
	}, ImageSandbox{Root: dir})
	if err != nil {
		t.Fatalf("handleImageExtractBatch: %v", err)
	}
	if got := len(out.Result.Entries); got != 3 {
		t.Fatalf("entries = %d, want 3", got)
	}
	// First two should succeed, third should carry an error.
	if out.Result.Entries[0].Result == nil {
		t.Errorf("entry[0] (inside root) should have a palette: %+v", out.Result.Entries[0])
	}
	if out.Result.Entries[1].Result == nil {
		t.Errorf("entry[1] (data URI) should have a palette: %+v", out.Result.Entries[1])
	}
	if out.Result.Entries[2].Error == "" {
		t.Errorf("entry[2] (outside root) should have an error")
	}
}

func TestImageExtractBatchTotalFailureReturnsError(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	pathOutside, _ := makeTinyPNG(t, other)

	// Sandbox confined to dir; the only source escapes it.
	_, _, err := handleImageExtractBatch(context.Background(), ImageExtractBatchParams{
		Sources: []string{pathOutside},
	}, ImageSandbox{Root: dir})
	if err == nil {
		t.Fatalf("expected total-failure error when every source is rejected")
	}
}

func TestImageExtractBatchEmpty(t *testing.T) {
	_, _, err := handleImageExtractBatch(context.Background(), ImageExtractBatchParams{}, ImageSandbox{})
	if err == nil {
		t.Fatalf("expected error for empty sources")
	}
}

// TestSandboxRejectsSymlinkEscape pins the symlink-bypass fix: a symlink
// inside Root pointing outside Root must be rejected. Skip on platforms
// where the test cannot create a symlink (e.g. Windows without privileges).
func TestSandboxRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	target, _ := makeTinyPNG(t, other)
	link := filepath.Join(dir, "evil.png")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	if _, err := (ImageSandbox{Root: dir}).CheckPath(link); err == nil {
		t.Fatalf("symlink %q → %q escapes root %q but was accepted", link, target, dir)
	}
}

// TestImageExtractRedirectToDisallowedHostBlocked verifies the hardened
// HTTP client re-applies the host allowlist on redirect — without that, a
// server on an allowed host could redirect to an arbitrary host (SSRF).
func TestImageExtractRedirectToDisallowedHostBlocked(t *testing.T) {
	// 'evil' server: serves a benign PNG from a host we DO NOT allow.
	_, data := makeTinyPNG(t, "")
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	}))
	defer evil.Close()

	// 'good' server: redirects every request to the evil server.
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL, http.StatusFound)
	}))
	defer good.Close()

	// Allowlist contains only the 'good' server's host. Direct fetch on the
	// good URL would pass; the redirect to evil must be blocked.
	goodURL := good.URL // "http://127.0.0.1:NNNN"
	goodHost := strings.TrimPrefix(strings.TrimPrefix(goodURL, "https://"), "http://")
	if i := strings.IndexByte(goodHost, '/'); i >= 0 {
		goodHost = goodHost[:i]
	}

	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		URL: goodURL,
	}, ImageSandbox{AllowHosts: []string{goodHost}})
	if err == nil {
		t.Fatalf("expected redirect to disallowed host to be blocked")
	}
	if !strings.Contains(err.Error(), "redirect rejected") && !strings.Contains(err.Error(), "not in allowed list") {
		t.Errorf("error %q should mention the redirect/allowlist rejection", err)
	}
}

// TestImageExtractRedirectCap pins the redirect-loop cap. Two servers
// redirect to each other — without a cap the client would loop forever.
func TestImageExtractRedirectCap(t *testing.T) {
	var srv1, srv2 *httptest.Server
	srv1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv2.URL, http.StatusFound)
	}))
	defer srv1.Close()
	srv2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv1.URL, http.StatusFound)
	}))
	defer srv2.Close()

	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		URL: srv1.URL,
	}, ImageSandbox{}) // no AllowHosts → host check off, only the cap applies
	if err == nil {
		t.Fatalf("expected redirect cap to abort the loop")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d redirects", sandbox.HTTPMaxRedirects)) {
		t.Errorf("error %q should mention the redirect cap", err)
	}
}

// TestSSRFBlockRejectsLoopback pins the BlockPrivateNetworks dial control:
// even a perfectly fine httptest server on 127.0.0.1 must be refused when
// the sandbox is asked to refuse private networks.
func TestSSRFBlockRejectsLoopback(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		URL: srv.URL,
	}, ImageSandbox{BlockPrivateNetworks: true})
	if err == nil {
		t.Fatalf("expected loopback refusal under BlockPrivateNetworks")
	}
	if !strings.Contains(err.Error(), "ssrf") {
		t.Errorf("error %q should mention ssrf", err)
	}
}

func TestSandboxCheckPathRelative(t *testing.T) {
	dir := t.TempDir()
	// Create a child file the sandbox should accept.
	child := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(child, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	abs, err := ImageSandbox{Root: dir}.CheckPath(child)
	if err != nil {
		t.Fatalf("CheckPath: %v", err)
	}
	if !filepath.IsAbs(abs) {
		t.Errorf("returned path %q is not absolute", abs)
	}

	// `..` traversal must fail.
	traversal := filepath.Join(dir, "..", "evil.txt")
	if _, err := (ImageSandbox{Root: dir}).CheckPath(traversal); err == nil {
		t.Errorf("traversal path should have been rejected")
	}
}

func TestImageExtractSoftPreset(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	encoded := base64.StdEncoding.EncodeToString(data)
	_, out, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data:       encoded,
		Method:     "softk",
		Size:       2,
		SoftPreset: "colorful",
	}, ImageSandbox{})
	if err != nil {
		t.Fatalf("handleImageExtract: %v", err)
	}
	if out.Result.Size == 0 {
		t.Fatalf("empty palette")
	}
	params := out.Result.Metadata.Params
	if got := params["soft_preset"]; got != "colorful" {
		t.Errorf("metadata.soft_preset = %v, want 'colorful'", got)
	}
}

func TestImageExtractSoftPresetCaseInsensitive(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	encoded := base64.StdEncoding.EncodeToString(data)
	_, out, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data:       encoded,
		Method:     "soft",
		Size:       2,
		SoftPreset: "BRIGHT",
	}, ImageSandbox{})
	if err != nil {
		t.Fatalf("handleImageExtract: %v", err)
	}
	if got := out.Result.Metadata.Params["soft_preset"]; got != "bright" {
		t.Errorf("soft_preset = %v, want 'bright' (normalised)", got)
	}
}

func TestImageExtractSoftPresetUnknown(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	encoded := base64.StdEncoding.EncodeToString(data)
	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data:       encoded,
		Method:     "softk",
		SoftPreset: "bogus",
	}, ImageSandbox{})
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
	if !strings.Contains(err.Error(), "unknown soft preset") {
		t.Errorf("error %q should mention 'unknown soft preset'", err.Error())
	}
}

func TestImageExtractSoftPresetWrongMethod(t *testing.T) {
	_, data := makeTinyPNG(t, "")
	encoded := base64.StdEncoding.EncodeToString(data)
	_, _, err := handleImageExtract(context.Background(), ImageExtractParams{
		Data:       encoded,
		Method:     "kmeans",
		SoftPreset: "bright",
	}, ImageSandbox{})
	if err == nil {
		t.Fatal("expected error for soft_preset + non-soft method")
	}
	if !strings.Contains(err.Error(), "requires method") {
		t.Errorf("error %q should mention method requirement", err.Error())
	}
}

func TestImageExtractBatchSoftPresetUnknown(t *testing.T) {
	// Bad preset should short-circuit before any source loading.
	_, _, err := handleImageExtractBatch(context.Background(), ImageExtractBatchParams{
		Sources:    []string{"does-not-matter"},
		Method:     "softk",
		SoftPreset: "bogus",
	}, ImageSandbox{})
	if err == nil {
		t.Fatal("expected error for unknown preset in batch")
	}
	if !strings.Contains(err.Error(), "unknown soft preset") {
		t.Errorf("error %q should mention 'unknown soft preset'", err.Error())
	}
}
