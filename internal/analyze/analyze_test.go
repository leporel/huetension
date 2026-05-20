package analyze

import (
	"bytes"
	"image"
	stdcolor "image/color"
	"image/png"
	"testing"

	"github.com/leporel/huetension/internal/color"
)

// makeTestImage builds a small RGB image with a known hue/luminance sweep
// so the monotonicity assertions can rely on the input distribution.
func makeTestImage(t *testing.T) image.Image {
	t.Helper()
	const w, h = 64, 64
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// A diagonal sweep across the full sRGB cube so we have a
			// rich distribution of luminance and hue.
			r := uint8(x * 255 / (w - 1))
			g := uint8(y * 255 / (h - 1))
			b := uint8((x + y) * 255 / (w + h - 2))
			img.Set(x, y, stdcolor.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func TestStrips_ReturnsFourStripsInOrder(t *testing.T) {
	img := makeTestImage(t)
	r, err := Strips(img, Options{Width: 256, Height: 16})
	if err != nil {
		t.Fatalf("Strips: %v", err)
	}
	if got := len(r.Strips); got != 4 {
		t.Fatalf("len(Strips) = %d, want 4", got)
	}
	want := []Metric{MetricHue, MetricLuminance, MetricSaturation, MetricDistance}
	for i, s := range r.Strips {
		if s.Metric != want[i] {
			t.Errorf("Strips[%d].Metric = %q, want %q", i, s.Metric, want[i])
		}
		if len(s.PNG) == 0 {
			t.Errorf("Strips[%d].PNG is empty", i)
		}
	}
}

func TestStrips_PNGDimensionsMatchOptions(t *testing.T) {
	img := makeTestImage(t)
	const W, H = 300, 24
	r, err := Strips(img, Options{Width: W, Height: H})
	if err != nil {
		t.Fatalf("Strips: %v", err)
	}
	for _, s := range r.Strips {
		decoded, err := png.Decode(bytes.NewReader(s.PNG))
		if err != nil {
			t.Fatalf("%s: png decode: %v", s.Metric, err)
		}
		b := decoded.Bounds()
		if b.Dx() != W || b.Dy() != H {
			t.Errorf("%s: dims = %dx%d, want %dx%d", s.Metric, b.Dx(), b.Dy(), W, H)
		}
	}
}

func TestStrips_Deterministic(t *testing.T) {
	img := makeTestImage(t)
	opts := Options{Width: 128, Height: 16}

	a, err := Strips(img, opts)
	if err != nil {
		t.Fatalf("Strips a: %v", err)
	}
	b, err := Strips(img, opts)
	if err != nil {
		t.Fatalf("Strips b: %v", err)
	}
	for i := range a.Strips {
		if !bytes.Equal(a.Strips[i].PNG, b.Strips[i].PNG) {
			t.Errorf("strip %d (%s) bytes differ across runs", i, a.Strips[i].Metric)
		}
	}
}

// columnColors decodes a single-row sample of the strip into a slice of
// per-column colours so monotonicity assertions can iterate over them.
func columnColors(t *testing.T, s Strip) []color.Color {
	t.Helper()
	decoded, err := png.Decode(bytes.NewReader(s.PNG))
	if err != nil {
		t.Fatalf("%s: png decode: %v", s.Metric, err)
	}
	b := decoded.Bounds()
	out := make([]color.Color, b.Dx())
	y := b.Min.Y // sample the top row — every row is identical
	for x := b.Min.X; x < b.Max.X; x++ {
		out[x-b.Min.X] = color.FromImageColor(decoded.At(x, y))
	}
	return out
}

func TestStrips_LuminanceMonotonic(t *testing.T) {
	img := makeTestImage(t)
	r, err := Strips(img, Options{Width: 64, Height: 8})
	if err != nil {
		t.Fatalf("Strips: %v", err)
	}
	cols := columnColors(t, r.Strips[1]) // luminance
	prev := -1.0
	for i, c := range cols {
		L, _, _ := c.ToOkLCH()
		if L+1e-9 < prev {
			t.Errorf("luminance non-monotonic at col %d: %.4f then %.4f", i, prev, L)
		}
		prev = L
	}
}

// In HSL space the luminance strip sorts by HSL L; the bucket painting
// averages in OkLab so the rendered HSL L isn't strictly monotonic per
// column, but the overall trend across quarters must rise.
func TestStrips_HSLSpaceLuminanceTrendsUp(t *testing.T) {
	img := makeTestImage(t)
	r, err := Strips(img, Options{Width: 64, Height: 8, Space: SpaceHSL})
	if err != nil {
		t.Fatalf("Strips: %v", err)
	}
	cols := columnColors(t, r.Strips[1])
	q := len(cols) / 4
	var first, last float64
	for i := 0; i < q; i++ {
		_, _, l1 := cols[i].ToHSL()
		_, _, l2 := cols[len(cols)-1-i].ToHSL()
		first += l1
		last += l2
	}
	if last <= first {
		t.Errorf("HSL luminance trend not increasing: first-quarter avg %.4f, last-quarter avg %.4f",
			first/float64(q), last/float64(q))
	}
}

func TestStrips_HSLAndOkLCHDiffer(t *testing.T) {
	img := makeTestImage(t)
	a, err := Strips(img, Options{Width: 128, Height: 8, Space: SpaceOkLCH})
	if err != nil {
		t.Fatalf("oklch: %v", err)
	}
	b, err := Strips(img, Options{Width: 128, Height: 8, Space: SpaceHSL})
	if err != nil {
		t.Fatalf("hsl: %v", err)
	}
	// At least one of the three space-dependent strips should differ
	// across the two spaces — they sort against different keys.
	diff := false
	for i := 0; i < 3; i++ {
		if !bytes.Equal(a.Strips[i].PNG, b.Strips[i].PNG) {
			diff = true
			break
		}
	}
	if !diff {
		t.Errorf("OkLCH and HSL strips are byte-identical — sort keys are not actually space-dependent")
	}
	// The Distance strip is space-independent.
	if !bytes.Equal(a.Strips[3].PNG, b.Strips[3].PNG) {
		t.Errorf("Distance strip differs across spaces — should not depend on Space")
	}
}

func TestStrips_BadSpace(t *testing.T) {
	img := makeTestImage(t)
	_, err := Strips(img, Options{Width: 64, Height: 8, Space: "lab"})
	if err == nil {
		t.Fatalf("expected error for unknown space")
	}
}

func TestParseSpace(t *testing.T) {
	cases := []struct {
		in   string
		want Space
		ok   bool
	}{
		{"", DefaultSpace, true},
		{"oklch", SpaceOkLCH, true},
		{"hsl", SpaceHSL, true},
		{"lab", "", false},
	}
	for _, tc := range cases {
		got, ok := ParseSpace(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseSpace(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestStrips_DistanceTargetValidation(t *testing.T) {
	img := makeTestImage(t)
	_, err := Strips(img, Options{Width: 64, Height: 8, DistanceTarget: "purple"})
	if err == nil {
		t.Fatalf("expected error for unknown distance target")
	}
}

func TestStrips_DistanceTargetDefaults(t *testing.T) {
	img := makeTestImage(t)
	defaulted, err := Strips(img, Options{Width: 64, Height: 8})
	if err != nil {
		t.Fatalf("default target: %v", err)
	}
	explicit, err := Strips(img, Options{Width: 64, Height: 8, DistanceTarget: DistanceBlue})
	if err != nil {
		t.Fatalf("explicit blue: %v", err)
	}
	if !bytes.Equal(defaulted.Strips[3].PNG, explicit.Strips[3].PNG) {
		t.Errorf("blue default should match explicit blue")
	}
}

func TestStrips_NilImage(t *testing.T) {
	if _, err := Strips(nil, Options{}); err == nil {
		t.Fatalf("expected error for nil image")
	}
}

func TestStrips_FullyTransparentImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8)) // all zero alpha
	_, err := Strips(img, Options{Width: 64, Height: 8})
	if err == nil {
		t.Fatalf("expected error for fully transparent image")
	}
}

func TestParseDistanceTarget(t *testing.T) {
	cases := []struct {
		in   string
		want DistanceTarget
		ok   bool
	}{
		{"", DefaultDistanceTarget, true},
		{"red", DistanceRed, true},
		{"green", DistanceGreen, true},
		{"blue", DistanceBlue, true},
		{"orange", "", false},
	}
	for _, tc := range cases {
		got, ok := ParseDistanceTarget(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseDistanceTarget(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseMetric(t *testing.T) {
	for _, m := range AllMetrics {
		if got, ok := ParseMetric(string(m)); !ok || got != m {
			t.Errorf("ParseMetric(%q) = (%q, %v)", m, got, ok)
		}
	}
	if _, ok := ParseMetric("bogus"); ok {
		t.Errorf("ParseMetric bogus should fail")
	}
}
