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
	"time"

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
// as a visual snapshot of how each algorithm currently behaves. For the
// soft and softk methods, an additional sidecar per preset is rendered so
// the mood comparison table in testdata/README.md stays in sync.
func TestExtractWritesPaletteSidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	fixtures := []string{"img1.png", "img2.jpg", "img3.jpg"}
	for _, name := range fixtures {
		for _, m := range AllMethods {
			out := paletteSidecarPath(name, m)
			start := time.Now()
			p, err := FromSource(context.Background(), fixturePath(name), Options{
				Method:      m,
				PaletteSize: 5,
				SortBy:      palette.SortByOkL, // dark → light makes the swatch row read like a gradient
			}, imageio.LoadOptions{})
			elapsed := time.Since(start)
			if err != nil {
				t.Errorf("%s/%s: %v", name, m, err)
				continue
			}
			t.Logf("%-9s | %-26s | %5d ms | size=%d", name, string(m), elapsed.Milliseconds(), len(p.Colors))
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
		// Per-preset sidecars for the soft / softk methods. SoftPresetDefault
		// is skipped — the base imgN_palette_{soft,softk}.jpg above already
		// renders the default preset (applyDefaults normalises empty preset
		// to default for the soft pipeline).
		for _, m := range []Method{MethodSoft, MethodSoftK} {
			for _, preset := range AllSoftPresets {
				if preset == SoftPresetDefault {
					continue
				}
				out := presetSidecarPath(name, m, preset)
				start := time.Now()
				p, err := FromSource(context.Background(), fixturePath(name), Options{
					Method:      m,
					PaletteSize: 5,
					SortBy:      palette.SortByOkL,
					SoftPreset:  preset,
				}, imageio.LoadOptions{})
				elapsed := time.Since(start)
				if err != nil {
					t.Errorf("%s/%s/%s: %v", name, m, preset, err)
					continue
				}
				label := string(m) + " - " + string(preset)
				effective, _ := p.Metadata.Params["preset_effective"].(bool)
				t.Logf("%-9s | %-26s | %5d ms | size=%d effective=%v", name, label, elapsed.Milliseconds(), len(p.Colors), effective)
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

// presetSidecarPath returns
// "testdata/<basename>_palette_<method>_<preset>.jpg" — the per-preset
// variant of paletteSidecarPath used by the mood comparison table.
func presetSidecarPath(name string, m Method, preset SoftPreset) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return filepath.Join("testdata", base+"_palette_"+string(m)+"_"+string(preset)+".jpg")
}

func TestSoftFallbackWhenFiltersTooStrict(t *testing.T) {
	// Force a chroma floor the input can't satisfy. Soft mode must fall
	// back to the raw pixel set rather than returning empty.
	img := solidImage(32, stdcolor.NRGBA{R: 128, G: 128, B: 128, A: 255})
	p, err := Extract(context.Background(), img, Options{
		Method:      MethodSoft,
		PaletteSize: 3,
		MinChroma:   0.99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Colors) == 0 {
		t.Errorf("soft mode returned empty palette under aggressive filter")
	}
}

func TestApplyDefaultsRespectsPresetPrecedence(t *testing.T) {
	t.Run("preset fills only zero fields", func(t *testing.T) {
		got := applyDefaults(Options{
			Method:     MethodSoft,
			SoftPreset: SoftPresetColorful,
		})
		want := softPresetMapping(SoftPresetColorful)
		if got.MinOkL != want.MinOkL || got.MaxOkL != want.MaxOkL {
			t.Errorf("OkL bounds: got (%.2f, %.2f), want (%.2f, %.2f)",
				got.MinOkL, got.MaxOkL, want.MinOkL, want.MaxOkL)
		}
		if got.MinChroma != want.MinChroma {
			t.Errorf("MinChroma: got %.2f, want %.2f", got.MinChroma, want.MinChroma)
		}
		if got.RankSaturationExponent != want.SaturationExponent {
			t.Errorf("exponent: got %.2f, want %.2f",
				got.RankSaturationExponent, want.SaturationExponent)
		}
	})

	t.Run("explicit overrides preserved", func(t *testing.T) {
		got := applyDefaults(Options{
			Method:                 MethodSoftK,
			SoftPreset:             SoftPresetBright,
			MinOkL:                 0.42,
			RankSaturationExponent: 3.14,
		})
		if got.MinOkL != 0.42 {
			t.Errorf("explicit MinOkL was overwritten: %.2f", got.MinOkL)
		}
		if got.RankSaturationExponent != 3.14 {
			t.Errorf("explicit exponent was overwritten: %.2f", got.RankSaturationExponent)
		}
		// Untouched fields still come from the preset.
		want := softPresetMapping(SoftPresetBright)
		if got.MaxOkL != want.MaxOkL {
			t.Errorf("MaxOkL should be from preset: got %.2f, want %.2f",
				got.MaxOkL, want.MaxOkL)
		}
	})

	t.Run("empty preset is normalised to default", func(t *testing.T) {
		got := applyDefaults(Options{Method: MethodSoft})
		if got.SoftPreset != SoftPresetDefault {
			t.Errorf("SoftPreset: got %q, want %q", got.SoftPreset, SoftPresetDefault)
		}
		want := softPresetMapping(SoftPresetDefault)
		if got.MinOkL != want.MinOkL || got.MaxOkL != want.MaxOkL {
			t.Errorf("OkL bounds: got (%.2f, %.2f), want (%.2f, %.2f)",
				got.MinOkL, got.MaxOkL, want.MinOkL, want.MaxOkL)
		}
		if got.RankSaturationExponent != want.SaturationExponent {
			t.Errorf("RankSaturationExponent: got %.2f, want %.2f",
				got.RankSaturationExponent, want.SaturationExponent)
		}
	})

	t.Run("non-soft method leaves preset fields untouched", func(t *testing.T) {
		got := applyDefaults(Options{Method: MethodKMeans})
		if got.SoftPreset != "" {
			t.Errorf("non-soft method should not auto-default SoftPreset, got %q", got.SoftPreset)
		}
		if got.MinOkL != 0 || got.RankSaturationExponent != 0 {
			t.Errorf("non-soft method should not populate preset fields, got MinOkL=%.2f exp=%.2f",
				got.MinOkL, got.RankSaturationExponent)
		}
	})
}

func TestSoftPresetProducesPalette(t *testing.T) {
	fixtures := []string{"img1.png", "img2.jpg", "img3.jpg"}
	for _, name := range fixtures {
		for _, preset := range AllSoftPresets {
			for _, m := range []Method{MethodSoft, MethodSoftK} {
				t.Run(name+"/"+string(m)+"/"+string(preset), func(t *testing.T) {
					p, err := FromSource(context.Background(), fixturePath(name), Options{
						Method:      m,
						PaletteSize: 5,
						SoftPreset:  preset,
					}, imageio.LoadOptions{})
					if err != nil {
						t.Fatalf("extract: %v", err)
					}
					if len(p.Colors) == 0 {
						t.Fatalf("empty palette")
					}
					if len(p.Colors) > 5 {
						t.Errorf("got %d colors, want ≤ 5", len(p.Colors))
					}
					if got := p.Metadata.Params["soft_preset"]; got != string(preset) {
						t.Errorf("metadata.soft_preset = %v, want %s", got, preset)
					}
				})
			}
		}
	}
}

func TestSoftPresetDeterministic(t *testing.T) {
	for _, preset := range AllSoftPresets {
		t.Run(string(preset), func(t *testing.T) {
			opts := Options{
				Method:      MethodSoftK,
				PaletteSize: 6,
				SoftPreset:  preset,
			}
			a, err := FromSource(context.Background(), fixturePath("img2.jpg"), opts, imageio.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			b, err := FromSource(context.Background(), fixturePath("img2.jpg"), opts, imageio.LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Colors) != len(b.Colors) {
				t.Fatalf("color count drifted: %d vs %d", len(a.Colors), len(b.Colors))
			}
			for i := range a.Colors {
				if a.Colors[i].R != b.Colors[i].R ||
					a.Colors[i].G != b.Colors[i].G ||
					a.Colors[i].B != b.Colors[i].B {
					t.Errorf("color %d drifted: %v vs %v", i, a.Colors[i], b.Colors[i])
				}
			}
		})
	}
}

func TestSoftPresetFallbackMetadata(t *testing.T) {
	// Solid mid-grey: zero chroma everywhere. SoftPresetDeep requires
	// MinChroma 0.10, so the pre-filter will knock out every pixel and the
	// pipeline must fall back to the raw set, marking the preset ineffective.
	img := solidImage(32, stdcolor.NRGBA{R: 128, G: 128, B: 128, A: 255})
	p, err := Extract(context.Background(), img, Options{
		Method:      MethodSoftK,
		PaletteSize: 3,
		SoftPreset:  SoftPresetDeep,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Colors) == 0 {
		t.Fatal("preset path returned empty palette under fallback")
	}
	if v, ok := p.Metadata.Params["preset_effective"].(bool); !ok || v {
		t.Errorf("preset_effective = %v (want false)", p.Metadata.Params["preset_effective"])
	}
	if v, ok := p.Metadata.Params["preset_fallback"].(string); !ok || v != "insufficient_pixels" {
		t.Errorf("preset_fallback = %v (want 'insufficient_pixels')", p.Metadata.Params["preset_fallback"])
	}
}

func TestSoftPresetEffectiveOnNormalImage(t *testing.T) {
	// On a real photo with lots of chroma, the preset should NOT fall back.
	p, err := FromSource(context.Background(), fixturePath("img2.jpg"), Options{
		Method:      MethodSoftK,
		PaletteSize: 5,
		SoftPreset:  SoftPresetColorful,
	}, imageio.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := p.Metadata.Params["preset_effective"].(bool); !ok || !v {
		t.Errorf("preset_effective = %v (want true)", p.Metadata.Params["preset_effective"])
	}
	if _, has := p.Metadata.Params["preset_fallback"]; has {
		t.Errorf("preset_fallback should be absent when preset is effective")
	}
}
