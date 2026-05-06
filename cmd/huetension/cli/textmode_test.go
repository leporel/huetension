package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestExtractCmdTextDefault verifies the default output for palette-producing
// commands is now the human-friendly text rendering, not JSON. Regression
// guard against accidentally flipping the default back.
func TestExtractCmdTextDefault(t *testing.T) {
	t.Setenv("NO_COLOR", "1") // strip ANSI to keep the assertion grep-friendly

	stdout := withStdoutBuffer(t)
	cmd := newExtractCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		fixturePath(t, "img1.png"),
		"--method", "kmeans",
		"--size", "3",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Errorf("default output looks like JSON, expected text-mode swatches:\n%s", got)
	}
	// Text output should mention hex codes for the 3 extracted colors.
	if !strings.Contains(got, "#") {
		t.Errorf("text output missing hex marker:\n%s", got)
	}
}

func TestHarmonyCmdTextDefault(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	stdout := withStdoutBuffer(t)
	cmd := newHarmonyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"triadic", "#3366cc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	// Text mode should NOT contain the CSS variable syntax.
	if strings.Contains(got, "--color-1:") {
		t.Errorf("default mode emitted CSS, want text:\n%s", got)
	}
	if !strings.Contains(got, "#3366cc") {
		t.Errorf("text output missing base color:\n%s", got)
	}
}

// TestErrPartialFailureSentinel guards the contract used by Execute()'s
// exit-code mapping: convert returns errPartialFailure when some inputs
// parsed and others didn't.
func TestErrPartialFailureSentinel(t *testing.T) {
	stdout := withStdoutBuffer(t)
	cmd := newConvertCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "not-a-color", "--format", "json"})

	err := cmd.Execute()
	if !errors.Is(err, errPartialFailure) {
		t.Errorf("got %v, want errPartialFailure (drives exit code 2)", err)
	}
	// Partial output should still have been written (one good, one error).
	if !strings.Contains(stdout.String(), "#ff0000") {
		t.Errorf("partial output missing the parseable input")
	}
}
