package extract

import (
	"image"
	stdcolor "image/color"
	"testing"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// TestAnnotateSourcesNearestPixel paints a 2×1 image whose two pixels are
// pure red on the left and pure blue on the right, then asserts the
// matching palette entries get pin coordinates pointing at their cells.
// (0.25, 0.5) is the centre of the left pixel; (0.75, 0.5) is the centre
// of the right pixel.
func TestAnnotateSourcesNearestPixel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, stdcolor.RGBA{R: 255, A: 255})
	img.SetRGBA(1, 0, stdcolor.RGBA{B: 255, A: 255})

	pal := palette.New([]color.Color{
		{R: 255, A: 255}, // red
		{B: 255, A: 255}, // blue
	})

	AnnotateSources(pal, img)

	if pal.Colors[0].Source == nil || pal.Colors[1].Source == nil {
		t.Fatalf("expected both sources populated, got %+v / %+v", pal.Colors[0].Source, pal.Colors[1].Source)
	}
	if got := pal.Colors[0].Source.X; got != 0.25 {
		t.Errorf("red x = %v, want 0.25", got)
	}
	if got := pal.Colors[1].Source.X; got != 0.75 {
		t.Errorf("blue x = %v, want 0.75", got)
	}
	if got := pal.Colors[0].Source.Y; got != 0.5 {
		t.Errorf("red y = %v, want 0.5", got)
	}
}

// TestAnnotateSourcesDeterminismOnTies confirms the documented contract:
// when several pixels are equidistant to a palette entry, the first in
// scan order (top-left → bottom-right) wins. A uniform 4-pixel image of
// the same color matched against that color must pick pixel (0, 0).
func TestAnnotateSourcesDeterminismOnTies(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			img.SetRGBA(x, y, stdcolor.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	pal := palette.New([]color.Color{{R: 100, G: 150, B: 200, A: 255}})

	AnnotateSources(pal, img)

	if pal.Colors[0].Source == nil {
		t.Fatalf("source nil")
	}
	if pal.Colors[0].Source.X != 0.25 || pal.Colors[0].Source.Y != 0.25 {
		t.Errorf("tie-break pixel = (%v,%v), want (0.25,0.25)",
			pal.Colors[0].Source.X, pal.Colors[0].Source.Y)
	}
}

// TestAnnotateSourcesNilSafe makes sure the helper never panics on
// degenerate inputs. Defensive — every caller in this repo currently
// guards, but library consumers may not.
func TestAnnotateSourcesNilSafe(t *testing.T) {
	AnnotateSources(nil, nil)
	AnnotateSources(palette.New(nil), nil)
	AnnotateSources(nil, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	// 0×0 image:
	AnnotateSources(palette.New([]color.Color{{R: 1}}), image.NewRGBA(image.Rect(0, 0, 0, 0)))
}

// TestAnnotateSourcesSortStability covers the interaction with palette
// sorting: a palette sort after AnnotateSources must keep each color's
// pin attached. Implemented by sorting and verifying the pin matches
// the new index's color.
func TestAnnotateSourcesSortStability(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	img.SetRGBA(0, 0, stdcolor.RGBA{R: 255, A: 255})              // red, light
	img.SetRGBA(1, 0, stdcolor.RGBA{G: 255, A: 255})              // green, light
	img.SetRGBA(2, 0, stdcolor.RGBA{R: 20, G: 20, B: 20, A: 255}) // near-black, dark

	pal := palette.New([]color.Color{
		{R: 255, A: 255},
		{G: 255, A: 255},
		{R: 20, G: 20, B: 20, A: 255},
	})

	AnnotateSources(pal, img)

	// Capture the (hex, pin.X) pairs before sort.
	type assoc struct {
		hex string
		x   float64
	}
	before := make([]assoc, pal.Len())
	for i, c := range pal.Colors {
		before[i] = assoc{c.Hex(), c.Source.X}
	}

	if err := pal.Sort(palette.SortByLuminance, false); err != nil {
		t.Fatalf("sort: %v", err)
	}

	// After sort, find each original hex and check its pin.X is still
	// the same — Source pointer travelled with the color.
	for _, b := range before {
		found := false
		for _, c := range pal.Colors {
			if c.Hex() == b.hex {
				if c.Source == nil {
					t.Errorf("%s lost source after sort", b.hex)
					break
				}
				if c.Source.X != b.x {
					t.Errorf("%s source.X = %v after sort, want %v", b.hex, c.Source.X, b.x)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s not found after sort", b.hex)
		}
	}
}
