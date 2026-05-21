package lut

import (
	"bytes"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

func TestGenerateIdentity(t *testing.T) {
	p := palette.New([]color.Color{
		color.New(255, 0, 0),
		color.New(0, 255, 0),
		color.New(0, 0, 255),
	})

	opts := Options{
		Size:              5,
		Radius:            0.15,
		Distribution:      0.5,
		Intensity:         0,
		BlendNeighbors:    1,
		IncludeSaturation: true,
	}

	lut, err := Generate(p, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// With Intensity=0 the short-circuit emits exact identity colours.
	for idx := range opts.Size * opts.Size * opts.Size {
		r := idx % opts.Size
		g := (idx / opts.Size) % opts.Size
		b := (idx / (opts.Size * opts.Size))

		expR := uint8(math.Round(float64(r) / float64(opts.Size-1) * 255))
		expG := uint8(math.Round(float64(g) / float64(opts.Size-1) * 255))
		expB := uint8(math.Round(float64(b) / float64(opts.Size-1) * 255))

		got := lut.Nodes[idx]
		if got.R != expR || got.G != expG || got.B != expB {
			t.Fatalf("node %d: expected (%d,%d,%d), got (%d,%d,%d)",
				idx, expR, expG, expB, got.R, got.G, got.B)
		}
	}
}

func TestGenerateBlendNeighborsDiffer(t *testing.T) {
	p := palette.New([]color.Color{
		color.New(255, 0, 0),
		color.New(0, 255, 0),
		color.New(0, 0, 255),
		color.New(255, 255, 0),
	})

	common := Options{
		Size:              5,
		Radius:            0.15,
		Distribution:      0.5,
		Intensity:         0.8,
		IncludeSaturation: true,
	}

	o1 := common
	o1.BlendNeighbors = 1
	o3 := common
	o3.BlendNeighbors = 3

	lut1, err := Generate(p, o1)
	if err != nil {
		t.Fatalf("Generate (blend 1): %v", err)
	}
	lut3, err := Generate(p, o3)
	if err != nil {
		t.Fatalf("Generate (blend 3): %v", err)
	}

	diff := 0
	for i := range lut1.Nodes {
		a, b := lut1.Nodes[i], lut3.Nodes[i]
		if a.R != b.R || a.G != b.G || a.B != b.B {
			diff++
		}
	}
	if diff == 0 {
		t.Error("BlendNeighbors=1 and BlendNeighbors=3 produced identical LUTs")
	}
}

func TestGenerateSaturationToggle(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})

	common := Options{
		Size:           5,
		Radius:         0.15,
		Distribution:   0.5,
		Intensity:      0.8,
		BlendNeighbors: 1,
	}

	withSat := common
	withSat.IncludeSaturation = true
	noSat := common
	noSat.IncludeSaturation = false

	lutWith, err := Generate(p, withSat)
	if err != nil {
		t.Fatalf("Generate (with sat): %v", err)
	}
	lutNo, err := Generate(p, noSat)
	if err != nil {
		t.Fatalf("Generate (no sat): %v", err)
	}

	diff := 0
	for i := range lutWith.Nodes {
		a, b := lutWith.Nodes[i], lutNo.Nodes[i]
		if a.R != b.R || a.G != b.G || a.B != b.B {
			diff++
		}
	}
	if diff == 0 {
		t.Error("IncludeSaturation toggle produced identical LUTs")
	}
}

func TestGenerateDeterminism(t *testing.T) {
	p := palette.New([]color.Color{
		color.New(255, 0, 0),
		color.New(0, 255, 0),
	})

	opts := Options{
		Size:              5,
		Radius:            0.15,
		Distribution:      0.5,
		Intensity:         0.8,
		BlendNeighbors:    1,
		IncludeSaturation: true,
	}

	lut1, _ := Generate(p, opts)
	lut2, _ := Generate(p, opts)

	if !bytes.Equal(EncodeCube(lut1, "t"), EncodeCube(lut2, "t")) {
		t.Error("two Generate runs produced different cubes")
	}
}

func TestGenerateValidation(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})

	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{"size 0 defaults to 33", Options{Radius: 0.15, Distribution: 0.5, Intensity: 0.8, BlendNeighbors: 1}, false},
		{"negative radius", Options{Size: 5, Radius: -0.1, Distribution: 0.5, Intensity: 0.8, BlendNeighbors: 1}, true},
		{"distribution > 1", Options{Size: 5, Radius: 0.15, Distribution: 1.5, Intensity: 0.8, BlendNeighbors: 1}, true},
		{"intensity > 1", Options{Size: 5, Radius: 0.15, Distribution: 0.5, Intensity: 1.5, BlendNeighbors: 1}, true},
		{"size too large", Options{Size: 300, Radius: 0.15, Distribution: 0.5, Intensity: 0.8, BlendNeighbors: 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Generate(p, tt.opts)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestEncodeCube(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})
	opts := Options{Size: 5, Radius: 0.15, Distribution: 0.5, Intensity: 0.5, BlendNeighbors: 1, IncludeSaturation: true}
	lut, err := Generate(p, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	out := string(EncodeCube(lut, "test"))

	if !strings.Contains(out, "# huetension LUT") {
		t.Error("missing huetension comment")
	}
	if !strings.Contains(out, `TITLE "test"`) {
		t.Error("missing TITLE")
	}
	if !strings.Contains(out, "LUT_3D_SIZE 5") {
		t.Error("missing LUT_3D_SIZE")
	}

	dataLines := 0
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "TITLE") || strings.HasPrefix(line, "LUT_3D_SIZE") {
			continue
		}
		dataLines++
	}

	want := opts.Size * opts.Size * opts.Size
	if dataLines != want {
		t.Errorf("expected %d data lines, got %d", want, dataLines)
	}
}

func TestEncodeHaldPNG(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})
	opts := Options{Size: 64, Radius: 0.15, Distribution: 0.5, Intensity: 0.5, BlendNeighbors: 1, IncludeSaturation: true}
	lut, err := Generate(p, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	pngBytes, err := EncodeHaldPNG(lut)
	if err != nil {
		t.Fatalf("EncodeHaldPNG: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}

	side := opts.Size * int(math.Sqrt(float64(opts.Size)))
	if img.Bounds().Max.X != side || img.Bounds().Max.Y != side {
		t.Errorf("expected %dx%d, got %dx%d", side, side, img.Bounds().Max.X, img.Bounds().Max.Y)
	}
}

func TestEncodeHaldPNGRejectsNonPerfectSquareSize(t *testing.T) {
	lut := &LUT{Size: 5, Nodes: make([]color.Color, 125)}
	if _, err := EncodeHaldPNG(lut); err == nil {
		t.Error("expected error for non-perfect-square Size, got nil")
	}
}

// TestHaldIdentityMatchesReference verifies that an Intensity=0 level-8 HALD
// is pixel-identical to the canonical identity HALD .refs/LUT_original.png.
// This locks both the OkLab identity short-circuit and the HALD pixel layout.
func TestHaldIdentityMatchesReference(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping HALD identity test in short mode")
	}

	// Tests run in the package directory; the reference lives at repo root.
	refData, err := os.ReadFile("../../.refs/LUT_original.png")
	if err != nil {
		t.Skipf("reference file not found: %v", err)
	}
	refImg, err := png.Decode(bytes.NewReader(refData))
	if err != nil {
		t.Fatalf("decode reference PNG: %v", err)
	}

	p := palette.New([]color.Color{color.New(255, 0, 0)})
	lut, err := Generate(p, Options{
		Size:              64,
		Radius:            0.15,
		Distribution:      0.5,
		Intensity:         0,
		BlendNeighbors:    1,
		IncludeSaturation: true,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	pngBytes, err := EncodeHaldPNG(lut)
	if err != nil {
		t.Fatalf("EncodeHaldPNG: %v", err)
	}
	genImg, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode generated PNG: %v", err)
	}

	rb := refImg.Bounds()
	gb := genImg.Bounds()
	if rb != gb {
		t.Fatalf("bounds mismatch: reference %v, generated %v", rb, gb)
	}

	diff := 0
	for y := rb.Min.Y; y < rb.Max.Y; y++ {
		for x := rb.Min.X; x < rb.Max.X; x++ {
			r1, g1, b1, _ := refImg.At(x, y).RGBA()
			r2, g2, b2, _ := genImg.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 {
				diff++
			}
		}
	}
	if diff > 0 {
		t.Errorf("identity HALD differs from reference in %d pixels", diff)
	}
}
