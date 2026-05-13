package cli

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixturePath resolves a path under internal/extract/testdata/ from the
// cli/ test directory. The fixture set lives there because that's where
// extraction images are kept; cli tests reuse them rather than ship a
// duplicate copy.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", "internal", "extract", "testdata", name))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	return abs
}

// withStdoutBuffer swaps stdoutWriter for a fresh bytes.Buffer for the
// duration of the test, restoring the original on cleanup. Tests use this
// to capture what the command would have written to os.Stdout.
func withStdoutBuffer(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := stdoutWriter
	buf := &bytes.Buffer{}
	stdoutWriter = buf
	t.Cleanup(func() { stdoutWriter = prev })
	return buf
}

func TestExtractCmdJSONToStdout(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--size", "4",
		"--format", "json",
		"--pretty",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"schema": "huetension/v1"`) {
		t.Errorf("stdout missing JSON envelope schema; got: %s", out)
	}
	if !strings.Contains(out, `"tool": "extract"`) {
		t.Errorf("stdout missing tool identity; got: %s", out)
	}
	if !strings.Contains(out, `"result"`) {
		t.Errorf("stdout missing result wrapper; got: %s", out)
	}
}

func TestExtractCmdWritesPNGFromExtension(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "swatch.png")

	cmd := newExtractCmd()
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--size", "4",
		"--output", out,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoded swatch is not PNG: %v", err)
	}
	// 4 colors × default 96 px wide = 384 px wide.
	if got := img.Bounds().Dx(); got != 384 {
		t.Errorf("width = %d, want 384 (4 colors × 96 px)", got)
	}
	if got := img.Bounds().Dy(); got != 96 {
		t.Errorf("height = %d, want 96", got)
	}
}

func TestExtractCmdExplicitFormatBeatsExtension(t *testing.T) {
	// User wrote `--format css -o out.png` — the CSS format must win.
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out.png")

	cmd := newExtractCmd()
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--size", "3",
		"--format", "css",
		"--output", out,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), ":root {") {
		t.Errorf("file should be CSS, got prefix %q", string(data[:min(40, len(data))]))
	}
}

func TestExtractCmdSwatchSize(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "swatch.png")

	cmd := newExtractCmd()
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--size", "3",
		"--output", out,
		"--swatch-size", "40x20",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, _ := os.ReadFile(out)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != 120 {
		t.Errorf("width = %d, want 120 (3×40)", got)
	}
	if got := img.Bounds().Dy(); got != 20 {
		t.Errorf("height = %d, want 20", got)
	}
}

func TestExtractCmdRejectsUnknownFormat(t *testing.T) {
	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--format", "yaml",
	})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown format")
	}
}

func TestExtractCmdSoftPresetEmitsMetadata(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "softk",
		"--soft-preset", "colorful",
		"--size", "4",
		"--format", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"soft_preset":"colorful"`) {
		t.Errorf("stdout missing soft_preset in metadata; got: %s", out)
	}
	if !strings.Contains(out, `"preset_effective":true`) {
		t.Errorf("stdout missing preset_effective; got: %s", out)
	}
}

func TestExtractCmdSoftPresetCaseInsensitive(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "soft",
		"--soft-preset", "BRIGHT",
		"--size", "3",
		"--format", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), `"soft_preset":"bright"`) {
		t.Errorf("case-insensitive preset not normalised to 'bright'; got: %s", stdout.String())
	}
}

func TestExtractCmdSoftPresetUnknown(t *testing.T) {
	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--soft-preset", "bogus",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
	if !strings.Contains(err.Error(), "unknown soft preset") {
		t.Errorf("error %q should mention 'unknown soft preset'", err.Error())
	}
}

func TestExtractCmdSoftPresetWrongMethod(t *testing.T) {
	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--soft-preset", "bright",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for soft-preset with non-soft method")
	}
	if !strings.Contains(err.Error(), "requires --method soft or softk") {
		t.Errorf("error %q should explain method requirement", err.Error())
	}
}

func TestExtractCmdSoftPresetExplicitOverride(t *testing.T) {
	// Preset sets MinChroma; explicit --min-saturation is HSL and lives on
	// a separate axis — both should appear, but the preset path uses
	// chroma fields. Verify metadata reflects the preset's effective
	// chroma value, not zero.
	stdout := withStdoutBuffer(t)

	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img2.jpg"),
		"--method", "softk",
		"--soft-preset", "deep",
		"--size", "4",
		"--format", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"soft_preset":"deep"`) {
		t.Errorf("missing deep preset in metadata: %s", out)
	}
	// metadata writes min_chroma; deep preset sets it to 0.10.
	if !strings.Contains(out, `"min_chroma":0.1`) {
		t.Errorf("metadata should include min_chroma from preset; got: %s", out)
	}
}

func TestParseSwatchSize(t *testing.T) {
	cases := map[string]struct {
		w, h    int
		wantErr bool
	}{
		"":         {0, 0, false},
		"40x20":    {40, 20, false},
		"40X20":    {40, 20, false},
		"100x100":  {100, 100, false},
		"badinput": {0, 0, true},
		"40x":      {0, 0, true},
		"-5x10":    {0, 0, true},
		"0x10":     {0, 0, true},
	}
	for in, want := range cases {
		w, h, err := parseSwatchSize(in)
		if want.wantErr {
			if err == nil {
				t.Errorf("parseSwatchSize(%q) = (%d,%d,nil), want error", in, w, h)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSwatchSize(%q): %v", in, err)
			continue
		}
		if w != want.w || h != want.h {
			t.Errorf("parseSwatchSize(%q) = (%d,%d), want (%d,%d)", in, w, h, want.w, want.h)
		}
	}
}
