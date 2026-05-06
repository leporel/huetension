package extract

import (
	"context"
	"image"
	stdcolor "image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/palette"
)

// fixturePath returns the path to a file under testdata/.
func fixturePath(name string) string {
	return filepath.Join("testdata", name)
}

// solidImage returns an N×N image filled with a single color, useful for
// algorithms-can-detect-monochrome tests.
func solidImage(n int, c stdcolor.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			img.Set(x, y, c)
		}
	}
	return img
}

// stripeImage returns a checkerboard of two colors so every algorithm can
// confidently produce ≥ 2 distinct cluster centres.
func stripeImage(n int, a, b stdcolor.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			if (x+y)%2 == 0 {
				img.Set(x, y, a)
			} else {
				img.Set(x, y, b)
			}
		}
	}
	return img
}

func freqsSum(p *palette.Palette) float64 {
	s := 0.0
	for _, c := range p.Colors {
		s += c.Freq
	}
	return s
}

func TestExtractDefaults(t *testing.T) {
	img := stripeImage(32, stdcolor.NRGBA{R: 255, A: 255}, stdcolor.NRGBA{B: 255, A: 255})
	p, err := Extract(context.Background(), img, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || len(p.Colors) == 0 {
		t.Fatalf("nil/empty palette")
	}
	if p.Metadata.Method != string(MethodSoft) {
		t.Errorf("default method = %q, want soft", p.Metadata.Method)
	}
	// Frequencies should sum to roughly 1.
	if s := freqsSum(p); math.Abs(s-1.0) > 0.05 {
		t.Errorf("freqs sum = %.4f, want ≈ 1.0", s)
	}
}

func TestExtractAllMethodsProducePalettes(t *testing.T) {
	img := stripeImage(32, stdcolor.NRGBA{R: 255, A: 255}, stdcolor.NRGBA{B: 255, A: 255})
	for _, m := range AllMethods {
		p, err := Extract(context.Background(), img, Options{Method: m, PaletteSize: 4})
		if err != nil {
			t.Errorf("%s: %v", m, err)
			continue
		}
		if len(p.Colors) == 0 {
			t.Errorf("%s: empty palette", m)
		}
	}
}

func TestExtractRespectsPaletteSize(t *testing.T) {
	img := stripeImage(64, stdcolor.NRGBA{R: 200, G: 100, B: 50, A: 255}, stdcolor.NRGBA{B: 200, A: 255})
	p, err := Extract(context.Background(), img, Options{Method: MethodMedianCut, PaletteSize: 6})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Colors) != 6 {
		t.Errorf("got %d colors, want 6", len(p.Colors))
	}
}

func TestExtractMonochromeImage(t *testing.T) {
	img := solidImage(16, stdcolor.NRGBA{R: 60, G: 90, B: 200, A: 255})
	for _, m := range AllMethods {
		p, err := Extract(context.Background(), img, Options{Method: m, PaletteSize: 5})
		if err != nil {
			t.Errorf("%s: %v", m, err)
			continue
		}
		if len(p.Colors) == 0 {
			t.Errorf("%s: empty palette", m)
		}
		// At least the dominant color should be near the source.
		// (Soft mode might filter for sat/light, but #3C5AC8 has decent sat.)
		if !looksBlueish(p.Colors[0]) {
			t.Errorf("%s: dominant color = %s, expected blue-ish", m, p.Colors[0].Hex())
		}
	}
}

// looksBlueish is a lenient assertion — Blue ≥ Red and Blue ≥ Green.
func looksBlueish(c color.Color) bool {
	return c.B >= c.R && c.B >= c.G
}

func TestExtractMetadataPopulated(t *testing.T) {
	img := stripeImage(50, stdcolor.NRGBA{R: 255, A: 255}, stdcolor.NRGBA{G: 255, A: 255})
	p, err := Extract(context.Background(), img, Options{Method: MethodKMeans, PaletteSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	if p.Metadata.ImageInfo == nil {
		t.Fatalf("ImageInfo nil")
	}
	if p.Metadata.ImageInfo.OriginalSize != [2]int{50, 50} {
		t.Errorf("OriginalSize = %v", p.Metadata.ImageInfo.OriginalSize)
	}
	if p.Metadata.Stats == nil {
		t.Fatalf("Stats nil")
	}
	if p.Metadata.Stats.TotalPixels == 0 {
		t.Errorf("TotalPixels = 0")
	}
	if p.Metadata.Method != "kmeans" {
		t.Errorf("Method = %q", p.Metadata.Method)
	}
	if got := p.Metadata.Params["palette_size"]; got != 3 {
		t.Errorf("Params[palette_size] = %v, want 3", got)
	}
}

func TestExtractSortApplied(t *testing.T) {
	img := stripeImage(32, stdcolor.NRGBA{R: 255, A: 255}, stdcolor.NRGBA{R: 255, G: 255, B: 255, A: 255})
	p, err := Extract(context.Background(), img, Options{
		Method:      MethodKMeans,
		PaletteSize: 2,
		SortBy:      palette.SortByLuminance,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(p.Colors); i++ {
		if p.Colors[i-1].Luminance() > p.Colors[i].Luminance() {
			t.Errorf("not sorted by luminance ascending: %v", p.Colors)
			break
		}
	}
}

func TestExtractContextCancelled(t *testing.T) {
	img := stripeImage(64, stdcolor.NRGBA{R: 255, A: 255}, stdcolor.NRGBA{B: 255, A: 255})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Extract(ctx, img, Options{})
	if err == nil {
		t.Errorf("expected context error")
	}
}

func TestExtractFromFixtureFile(t *testing.T) {
	// Smoke test on the bundled JPEGs — exact colors are too volatile to
	// snapshot, but we can assert structural invariants.
	for _, name := range []string{"img2.jpg", "img3.jpg"} {
		p, err := FromSource(context.Background(), fixturePath(name), Options{Method: MethodSoft, PaletteSize: 6}, imageio.LoadOptions{})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(p.Colors) == 0 || len(p.Colors) > 6 {
			t.Errorf("%s: got %d colors", name, len(p.Colors))
		}
		if p.Metadata.Source == "" {
			t.Errorf("%s: Source empty", name)
		}
		if p.Metadata.ImageInfo.Format != "jpeg" {
			t.Errorf("%s: Format = %q", name, p.Metadata.ImageInfo.Format)
		}
		if s := freqsSum(p); math.Abs(s-1.0) > 0.05 {
			t.Errorf("%s: freqs sum = %.4f, want ≈ 1.0", name, s)
		}
	}
}

func TestBatchPreservesOrderAndReportsErrors(t *testing.T) {
	sources := []string{
		fixturePath("img2.jpg"),
		"definitely-does-not-exist.jpg",
		fixturePath("img3.jpg"),
	}
	results := Batch(context.Background(), sources, Options{Method: MethodMedianCut, PaletteSize: 4}, imageio.LoadOptions{}, 4)
	if len(results) != len(sources) {
		t.Fatalf("len = %d, want %d", len(results), len(sources))
	}
	if results[0].Err != nil {
		t.Errorf("idx 0: %v", results[0].Err)
	}
	if results[1].Err == nil {
		t.Errorf("idx 1: expected error for missing file")
	}
	if results[2].Err != nil {
		t.Errorf("idx 2: %v", results[2].Err)
	}
	for i, r := range results {
		if r.Source != sources[i] {
			t.Errorf("idx %d: source mismatch %q vs %q", i, r.Source, sources[i])
		}
	}
}

func TestExtractNilImage(t *testing.T) {
	if _, err := Extract(context.Background(), nil, Options{}); err == nil {
		t.Errorf("expected error for nil image")
	}
}

// TestExtractWritesPaletteSidecar regenerates one preview JPG per
// (fixture × method) combination, so `internal/extract/testdata/` doubles
// as a visual snapshot of how each algorithm currently behaves.
func TestExtractWritesPaletteSidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	fixtures := []string{"img1.png", "img2.jpg", "img3.jpg"}
	for _, name := range fixtures {
		for _, m := range AllMethods {
			out := paletteSidecarPath(name, m)
			p, err := FromSource(context.Background(), fixturePath(name), Options{
				Method:      m,
				PaletteSize: 6,
				SortBy:      palette.SortByOkL, // dark → light makes the swatch row read like a gradient
			}, imageio.LoadOptions{})
			if err != nil {
				t.Errorf("%s/%s: %v", name, m, err)
				continue
			}
			if err := writePaletteJPG(out, p, 96, 96); err != nil {
				t.Errorf("%s: %v", out, err)
				continue
			}
			info, err := os.Stat(out)
			if err != nil {
				t.Errorf("stat %s: %v", out, err)
				continue
			}
			if info.Size() < 100 {
				t.Errorf("output %s suspiciously small: %d bytes", out, info.Size())
			}
		}
	}
}

// writePaletteJPG renders p as a JPEG swatch strip via the exporter and
// writes it to path. Sharing the renderer with the exporter keeps the
// testdata regeneration aligned with what the CLI's `-o palette.jpg`
// produces — a single source of truth for the swatch layout.
func writePaletteJPG(path string, p *palette.Palette, swatchW, swatchH int) error {
	data, err := exporter.Export(p, exporter.FormatJPEG, exporter.Options{
		SwatchWidth:  swatchW,
		SwatchHeight: swatchH,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// paletteSidecarPath returns "testdata/<basename>_palette_<method>.jpg".
func paletteSidecarPath(name string, m Method) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return filepath.Join("testdata", base+"_palette_"+string(m)+".jpg")
}

func TestSoftFallbackWhenFiltersTooStrict(t *testing.T) {
	// Force MinSaturation to a value the input can't satisfy. Soft mode must
	// fall back to the raw pixel set rather than returning empty.
	img := solidImage(32, stdcolor.NRGBA{R: 128, G: 128, B: 128, A: 255})
	p, err := Extract(context.Background(), img, Options{
		Method:        MethodSoft,
		PaletteSize:   3,
		MinSaturation: 0.99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Colors) == 0 {
		t.Errorf("soft mode returned empty palette under aggressive filter")
	}
}
