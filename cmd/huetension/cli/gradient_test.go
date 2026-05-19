package cli

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestGradientCmdTwoStops(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newGradientCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#000000", "#ffffff", "--steps", "5", "--format", "txt"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("got %d lines, want 5\n%s", len(lines), stdout.String())
	}
	if lines[0] != "#000000" || lines[len(lines)-1] != "#ffffff" {
		t.Errorf("endpoints not preserved: first=%q last=%q", lines[0], lines[len(lines)-1])
	}
}

func TestGradientCmdMultiStop(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newGradientCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"red", "green", "blue", "--steps", "7", "--format", "txt"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 7 {
		t.Errorf("got %d lines, want 7", len(lines))
	}
}

// TestGradientCmdPNGIsSmoothGradient verifies a raster target renders a
// resampled smooth gradient, not the discrete --steps swatch strip.
func TestGradientCmdPNGIsSmoothGradient(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newGradientCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	// --steps 5 would be a 480×96 (5×96) swatch strip under the old path.
	cmd.SetArgs([]string{"#000000", "#ffffff", "--steps", "5", "--format", "png"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(stdout.Bytes()))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != gradientImageWidth || b.Dy() != gradientImageHeight {
		t.Errorf("png size = %dx%d, want %dx%d (a resampled gradient, not a %d-block strip)",
			b.Dx(), b.Dy(), gradientImageWidth, gradientImageHeight, 5)
	}
	// Left edge dark, right edge light → it is the gradient, end to end.
	rL, _, _, _ := img.At(b.Min.X, b.Min.Y).RGBA()
	rR, _, _, _ := img.At(b.Max.X-1, b.Min.Y).RGBA()
	if rL >= rR {
		t.Errorf("expected a dark→light gradient, got left R=%d right R=%d", rL, rR)
	}
}

func TestGradientCmdRejectsTooFewStops(t *testing.T) {
	cmd := newGradientCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"red"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for single stop")
	}
}

func TestGradientCmdRejectsBadColor(t *testing.T) {
	cmd := newGradientCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"red", "not-a-color"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unparseable stop")
	}
}
