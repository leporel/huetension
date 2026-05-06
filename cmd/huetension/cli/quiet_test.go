package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestExtractTextHeader verifies palette commands prepend a one-line
// summary in text mode ("Extracted N colors from photo.jpg").
func TestExtractTextHeader(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	stdout := withStdoutBuffer(t)
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"extract", fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--size", "3",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	first := strings.SplitN(stdout.String(), "\n", 2)[0]
	if !strings.HasPrefix(first, "Extracted 3 colors from") {
		t.Errorf("first line should be the header; got %q", first)
	}
}

// TestQuietSuppressesHeader verifies --quiet drops the header line while
// keeping the swatch table intact.
func TestQuietSuppressesHeader(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	stdout := withStdoutBuffer(t)
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--quiet",
		"extract", fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--size", "3",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, "Extracted") {
		t.Errorf("--quiet should suppress the header; got:\n%s", out)
	}
	if !strings.Contains(out, "#") {
		t.Errorf("body still expected (swatch table); got:\n%s", out)
	}
}

// TestQuietHasNoEffectOnJSON verifies --quiet leaves the JSON envelope
// untouched. The envelope's `tool` field already serves the role of the
// text header, so suppressing it would drop information.
func TestQuietHasNoEffectOnJSON(t *testing.T) {
	stdout := withStdoutBuffer(t)
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--quiet",
		"harmony", "triadic", "#3366cc",
		"--format", "json", "--pretty",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"tool": "harmony"`) {
		t.Errorf("--quiet should not strip JSON tool field:\n%s", out)
	}
}

// TestHarmonyTextHeader checks harmony's verb/source pair.
func TestHarmonyTextHeader(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	stdout := withStdoutBuffer(t)
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"harmony", "triadic", "#3366cc"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	first := strings.SplitN(stdout.String(), "\n", 2)[0]
	if !strings.Contains(first, "Generated") || !strings.Contains(first, "triadic") {
		t.Errorf("harmony header missing verb/type; got %q", first)
	}
}
