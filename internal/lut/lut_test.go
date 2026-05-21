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

func TestGenerateRBFIdentity(t *testing.T) {
	p := palette.New([]color.Color{
		color.New(255, 0, 0),
		color.New(0, 255, 0),
		color.New(0, 0, 255),
	})

	opts := Options{
		Size:              5,
		Method:            MethodRBF,
		Reach:             0.20,
		Sharpness:         2.0,
		Strength:          0,
		IncludeSaturation: true,
	}

	lut, err := Generate(p, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Strength=0 must short-circuit to the exact identity grid — same
	// invariant as the K-NN Intensity=0 path.
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

func TestGenerateRBFSharpnessDiffer(t *testing.T) {
	p := palette.New([]color.Color{
		color.New(255, 0, 0),
		color.New(0, 0, 255),
		color.New(0, 255, 0),
	})

	common := Options{
		Size:              5,
		Method:            MethodRBF,
		Reach:             0.25,
		Strength:          0.8,
		IncludeSaturation: true,
	}

	soft := common
	soft.Sharpness = 1.0
	sharp := common
	sharp.Sharpness = 6.0

	lutSoft, err := Generate(p, soft)
	if err != nil {
		t.Fatalf("Generate (sharpness 1): %v", err)
	}
	lutSharp, err := Generate(p, sharp)
	if err != nil {
		t.Fatalf("Generate (sharpness 6): %v", err)
	}

	diff := 0
	for i := range lutSoft.Nodes {
		a, b := lutSoft.Nodes[i], lutSharp.Nodes[i]
		if a.R != b.R || a.G != b.G || a.B != b.B {
			diff++
		}
	}
	if diff == 0 {
		t.Error("Sharpness=1 and Sharpness=6 produced identical LUTs")
	}
}

func TestGenerateRBFSaturationToggle(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})

	common := Options{
		Size:      5,
		Method:    MethodRBF,
		Reach:     0.20,
		Sharpness: 2.0,
		Strength:  0.8,
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

	// IncludeSaturation toggles whether (newA, newB) gets rescaled to
	// preserve the node's original chroma magnitude — outputs must
	// differ in at least one node.
	diff := 0
	for i := range lutWith.Nodes {
		a, b := lutWith.Nodes[i], lutNo.Nodes[i]
		if a.R != b.R || a.G != b.G || a.B != b.B {
			diff++
		}
	}
	if diff == 0 {
		t.Error("RBF IncludeSaturation toggle produced identical LUTs")
	}
}

func TestGenerateRBFDeterminism(t *testing.T) {
	p := palette.New([]color.Color{
		color.New(255, 0, 0),
		color.New(0, 255, 0),
	})

	opts := Options{
		Size:              5,
		Method:            MethodRBF,
		Reach:             0.20,
		Sharpness:         2.0,
		Strength:          0.8,
		IncludeSaturation: true,
	}

	lut1, _ := Generate(p, opts)
	lut2, _ := Generate(p, opts)

	if !bytes.Equal(EncodeCube(lut1, "t"), EncodeCube(lut2, "t")) {
		t.Error("two RBF Generate runs produced different cubes")
	}
}

func TestGenerateRBFValidation(t *testing.T) {
	p := palette.New([]color.Color{color.New(255, 0, 0)})

	base := Options{Size: 5, Method: MethodRBF, Reach: 0.20, Sharpness: 2.0, Strength: 0.8}

	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{"valid", base, false},
		{"reach 0", func() Options { o := base; o.Reach = 0; return o }(), true},
		{"negative reach", func() Options { o := base; o.Reach = -0.1; return o }(), true},
		{"sharpness 0", func() Options { o := base; o.Sharpness = 0; return o }(), true},
		{"negative sharpness", func() Options { o := base; o.Sharpness = -1; return o }(), true},
		{"negative strength", func() Options { o := base; o.Strength = -0.1; return o }(), true},
		{"strength > 1", func() Options { o := base; o.Strength = 1.5; return o }(), true},
		{"unknown method", func() Options { o := base; o.Method = "bogus"; return o }(), true},
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

// TestGenerateRBFShiftsPaletteRegion is a smoke test for the RBF path
// on an antipodal palette (blue + red — the case where the K-NN
// circular hue mean used to collapse). It just confirms the algorithm
// produces a non-identity cube with finite, in-gamut output. The
// visual smoothness claim against the K-NN path is verified by eye in
// the SPA, not asserted here — comparing the two output cubes pixel-
// wise depends on knob choice and would be brittle.
func TestGenerateRBFShiftsPaletteRegion(t *testing.T) {
	p := palette.New([]color.Color{
		color.New(20, 60, 220), // blue
		color.New(230, 40, 50), // red — near-antipodal hue in OkLab
	})

	opts := Options{
		Size:      9,
		Method:    MethodRBF,
		Reach:     0.20,
		Sharpness: 2.0,
		Strength:  0.90,
	}

	generated, err := Generate(p, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// At least one mid-cube node must have moved off the identity grid.
	moved := 0
	for idx, n := range generated.Nodes {
		r := idx % opts.Size
		g := (idx / opts.Size) % opts.Size
		b := (idx / (opts.Size * opts.Size))

		expR := uint8(math.Round(float64(r) / float64(opts.Size-1) * 255))
		expG := uint8(math.Round(float64(g) / float64(opts.Size-1) * 255))
		expB := uint8(math.Round(float64(b) / float64(opts.Size-1) * 255))

		if n.R != expR || n.G != expG || n.B != expB {
			moved++
		}
	}
	if moved == 0 {
		t.Error("RBF with Strength=0.9 produced an identity cube — algorithm did nothing")
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
