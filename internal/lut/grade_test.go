package lut

import (
	"math"
	"testing"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// gradeTestPalette is a teal + orange pair — the classic two-spoke grade,
// with hues far enough apart that every gap has a wide transition band.
func gradeTestPalette() *palette.Palette {
	return palette.New([]color.Color{
		color.New(20, 140, 150),
		color.New(230, 120, 40),
	})
}

// assertIdentityCube checks every node sits exactly on the RGB grid.
func assertIdentityCube(t *testing.T, l *LUT) {
	t.Helper()
	for idx, got := range l.Nodes {
		r := idx % l.Size
		g := (idx / l.Size) % l.Size
		b := idx / (l.Size * l.Size)

		expR := uint8(math.Round(float64(r) / float64(l.Size-1) * 255))
		expG := uint8(math.Round(float64(g) / float64(l.Size-1) * 255))
		expB := uint8(math.Round(float64(b) / float64(l.Size-1) * 255))

		if got.R != expR || got.G != expG || got.B != expB {
			t.Fatalf("node %d: expected (%d,%d,%d), got (%d,%d,%d)",
				idx, expR, expG, expB, got.R, got.G, got.B)
		}
	}
}

func TestGenerateGradeIdentity(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		pal  *palette.Palette
	}{
		{"zero knobs", Options{Size: 5, Method: MethodGrade}, gradeTestPalette()},
		{"zero knobs with saturation", Options{Size: 5, Method: MethodGrade, IncludeSaturation: true}, gradeTestPalette()},
		{"neutral palette hue-only", Options{Size: 5, Method: MethodGrade, Compression: 0.8, Mute: 0.5},
			palette.New([]color.Color{color.New(30, 30, 30), color.New(200, 200, 200)})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := Generate(tt.pal, tt.opts)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			assertIdentityCube(t, l)
		})
	}
}

func TestGenerateGradeValidation(t *testing.T) {
	p := gradeTestPalette()
	base := Options{Size: 5, Method: MethodGrade, Compression: 0.7, Mute: 0.3}

	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{"valid", base, false},
		{"negative compression", func() Options { o := base; o.Compression = -0.1; return o }(), true},
		{"compression > 1", func() Options { o := base; o.Compression = 1.5; return o }(), true},
		{"NaN compression", func() Options { o := base; o.Compression = math.NaN(); return o }(), true},
		{"negative mute", func() Options { o := base; o.Mute = -0.1; return o }(), true},
		{"mute > 1", func() Options { o := base; o.Mute = 2; return o }(), true},
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

// TestGenerateGradePreservesLightness is the method's core promise: only
// the chromatic plane moves, so every node keeps its OkLab L (up to 8-bit
// rounding) even at full compression with the saturation pull on.
func TestGenerateGradePreservesLightness(t *testing.T) {
	opts := Options{Size: 9, Method: MethodGrade, Compression: 1, Mute: 1, IncludeSaturation: true}
	l, err := Generate(gradeTestPalette(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	const tolerance = 0.03 // 8-bit quantisation of a rotated colour
	for idx, got := range l.Nodes {
		r := idx % opts.Size
		g := (idx / opts.Size) % opts.Size
		b := idx / (opts.Size * opts.Size)
		src := color.FromRGB01(
			float64(r)/float64(opts.Size-1),
			float64(g)/float64(opts.Size-1),
			float64(b)/float64(opts.Size-1),
		)
		if diff := math.Abs(src.OkL() - got.OkL()); diff > tolerance {
			t.Fatalf("node %d: lightness moved by %.4f (src %v → %v)", idx, diff, src.Hex(), got.Hex())
		}
	}
}

// TestGenerateGradeSnapsToPaletteHues checks the vectorscope compression
// actually happens: a saturated colour whose hue lies well inside a
// spoke's plateau lands on that spoke's hue, and the palette colours
// themselves are fixed points.
func TestGenerateGradeSnapsToPaletteHues(t *testing.T) {
	p := gradeTestPalette()
	mapper := newGradeMapper(p.Colors, 1)
	if len(mapper.anchors) != 2 {
		t.Fatalf("expected 2 anchors, got %d", len(mapper.anchors))
	}

	for _, a := range mapper.anchors {
		if got := mapper.warp(a.hue).hue; math.Abs(got-a.hue) > 1e-9 {
			t.Errorf("anchor hue %.2f moved to %.2f", a.hue, got)
		}
		// 15% of the way into the neighbouring gap is deep inside the
		// plateau at full compression — it must collapse onto the spoke.
		for _, sign := range []float64{-1, 1} {
			probe := math.Mod(a.hue+sign*0.15*180+360, 360)
			got := mapper.warp(probe).hue
			if d := hueDelta(got, a.hue); d > 2 {
				t.Errorf("hue %.2f near anchor %.2f warped to %.2f (%.2f° off)", probe, a.hue, got, d)
			}
		}
	}
}

// TestGenerateGradeHueMapIsMonotone walks the whole wheel and verifies
// the warp never reverses direction — the property that keeps gradients
// from folding back on themselves.
func TestGenerateGradeHueMapIsMonotone(t *testing.T) {
	pal := palette.New([]color.Color{
		color.New(230, 40, 50),
		color.New(240, 200, 40),
		color.New(30, 160, 90),
		color.New(40, 80, 220),
	})
	for _, compression := range []float64{0.3, 0.7, 1} {
		mapper := newGradeMapper(pal.Colors, compression)
		prev := mapper.warp(0).hue
		unwrapped := prev
		for h := 0.25; h < 360; h += 0.25 {
			cur := mapper.warp(h).hue
			step := cur - prev
			if step < -180 {
				step += 360
			}
			if step < -1e-9 {
				t.Fatalf("compression %.1f: hue map reverses at %.2f° (%.4f → %.4f)", compression, h, prev, cur)
			}
			unwrapped += step
			prev = cur
		}
		// One full turn in must be one full turn out.
		if math.Abs(unwrapped-mapper.warp(0).hue-360) > 3 {
			t.Fatalf("compression %.1f: map does not cover the wheel once (total %.2f°)", compression, unwrapped-mapper.warp(0).hue)
		}
	}
}

func TestGenerateGradeAnchorsSkipNeutralsAndMergeNearDuplicates(t *testing.T) {
	colors := []color.Color{
		color.New(128, 128, 128), // neutral — no anchor
		color.New(230, 120, 40),
		color.New(232, 121, 41), // same hue within the merge window
		color.New(20, 140, 150),
	}
	mapper := newGradeMapper(colors, 0.5)
	if len(mapper.anchors) != 2 {
		t.Fatalf("expected 2 anchors (neutral dropped, duplicate merged), got %d", len(mapper.anchors))
	}
	if mapper.anchors[0].hue > mapper.anchors[1].hue {
		t.Error("anchors are not sorted by hue")
	}
}

func TestGenerateGradeMuteAndSaturationChangeOutput(t *testing.T) {
	p := gradeTestPalette()
	base := Options{Size: 7, Method: MethodGrade, Compression: 0.7}
	plain, err := Generate(p, base)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	variants := map[string]Options{
		"mute":       func() Options { o := base; o.Mute = 0.8; return o }(),
		"saturation": func() Options { o := base; o.IncludeSaturation = true; return o }(),
	}
	for name, opts := range variants {
		got, err := Generate(p, opts)
		if err != nil {
			t.Fatalf("%s: Generate: %v", name, err)
		}
		differs := false
		for i := range plain.Nodes {
			if plain.Nodes[i] != got.Nodes[i] {
				differs = true
				break
			}
		}
		if !differs {
			t.Errorf("%s: toggling the knob produced an identical cube", name)
		}
	}
}

func TestGenerateGradeDeterminism(t *testing.T) {
	opts := Options{Size: 7, Method: MethodGrade, Compression: 0.7, Mute: 0.3}
	a, err := Generate(gradeTestPalette(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := Generate(gradeTestPalette(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for i := range a.Nodes {
		if a.Nodes[i] != b.Nodes[i] {
			t.Fatalf("node %d differs between runs", i)
		}
	}
}

// hueDelta is the shortest angular distance between two hues in degrees.
func hueDelta(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}
