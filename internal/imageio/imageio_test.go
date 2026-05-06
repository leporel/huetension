package imageio

import (
	"bytes"
	"encoding/base64"
	"image"
	stdcolor "image/color"
	"image/png"
	"path/filepath"
	"testing"
)

// makePNG returns a minimal PNG byte stream of the given size, useful for
// fixtures without committing binary blobs.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestLoadBytes(t *testing.T) {
	data := makePNG(t, 4, 4)
	got, err := LoadBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != "png" {
		t.Errorf("format = %q, want png", got.Format)
	}
	if got.Image.Bounds().Dx() != 4 {
		t.Errorf("dx = %d, want 4", got.Image.Bounds().Dx())
	}
}

func TestLoadFromTestdata(t *testing.T) {
	// Fixtures live in extract/testdata; we reach across since imageio has
	// no fixtures of its own and these are the canonical samples.
	path, err := filepath.Abs(filepath.Join("..", "extract", "testdata", "img2.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != "jpeg" {
		t.Errorf("format = %q, want jpeg", got.Format)
	}
	if got.SourceType != SourceFile {
		t.Errorf("source type = %q, want %q", got.SourceType, SourceFile)
	}
	if got.Image.Bounds().Empty() {
		t.Errorf("image bounds empty")
	}
}

func TestLoadDataURI(t *testing.T) {
	pngBytes := makePNG(t, 2, 2)
	uri := "data:image/png;base64," + base64Encode(pngBytes)
	got, err := Load(uri, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceType != SourceData {
		t.Errorf("source type = %q, want %q", got.SourceType, SourceData)
	}
	if got.Image.Bounds().Dx() != 2 {
		t.Errorf("dx = %d, want 2", got.Image.Bounds().Dx())
	}
}

func TestLoadFileNotExist(t *testing.T) {
	if _, err := Load("does-not-exist.png", LoadOptions{}); err == nil {
		t.Errorf("expected error")
	}
}

func TestLoadDataURINonBase64Rejected(t *testing.T) {
	// RFC 2397 also allows URL-encoded data, which we deliberately don't
	// support — confirm the rejection is explicit.
	if _, err := Load("data:image/png,some-payload", LoadOptions{}); err == nil {
		t.Errorf("expected error for non-base64 data URI")
	}
}

func TestLoadOversizeRejected(t *testing.T) {
	pngBytes := makePNG(t, 16, 16)
	// MaxBytes lower than payload size.
	uri := "data:image/png;base64," + base64Encode(pngBytes)
	_, err := Load(uri, LoadOptions{MaxBytes: 10})
	if err == nil {
		t.Errorf("expected size limit error")
	}
}

func TestResizeShrinksLongestSide(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 400))
	out := Resize(img, 256)
	if out.Bounds().Dx() != 256 {
		t.Errorf("dx = %d, want 256", out.Bounds().Dx())
	}
	if out.Bounds().Dy() != 128 {
		t.Errorf("dy = %d, want 128 (preserve 2:1 aspect)", out.Bounds().Dy())
	}
}

func TestResizeNoUpscale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	out := Resize(img, 256)
	if out.Bounds().Dx() != 50 {
		t.Errorf("Resize should not upscale: dx = %d", out.Bounds().Dx())
	}
}

func TestResizeZeroIsNoOp(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	out := Resize(img, 0)
	if out != image.Image(img) {
		t.Errorf("Resize(_, 0) should return original image")
	}
}

func TestPixelsCount(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	pix := Pixels(img, 0)
	if len(pix) != 12 {
		t.Errorf("len = %d, want 12", len(pix))
	}
}

func TestPixelsAlphaMask(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, stdcolor.NRGBA{R: 255, A: 255}) // opaque red
	img.Set(1, 0, stdcolor.NRGBA{G: 255, A: 0})   // fully transparent
	pix := Pixels(img, 1)
	if len(pix) != 1 {
		t.Errorf("len = %d, want 1 (transparent pixel masked)", len(pix))
	}
}

func TestCheckHostAllowList(t *testing.T) {
	allow := []string{"*.unsplash.com", "raw.githubusercontent.com"}
	cases := map[string]bool{
		"https://images.unsplash.com/photo.jpg":          true,
		"https://raw.githubusercontent.com/foo/bar.png":  true,
		"https://evil.com/photo.jpg":                     false,
		"https://unsplash.com/photo.jpg":                 false, // bare host doesn't match *.unsplash.com
	}
	for u, want := range cases {
		err := checkHost(u, allow)
		got := err == nil
		if got != want {
			t.Errorf("checkHost(%q) ok=%v, want %v (err=%v)", u, got, want, err)
		}
	}
}

func base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
