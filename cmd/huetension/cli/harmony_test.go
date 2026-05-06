package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHarmonyCmdTriadicCSS(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newHarmonyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"triadic", "#3366cc", "--format", "css"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "--color-1: #3366cc;") {
		t.Errorf("expected base color preserved as --color-1, got:\n%s", out)
	}
	// Triadic = 3 colors → --color-1, --color-2, --color-3.
	for _, want := range []string{"--color-1", "--color-2", "--color-3"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestHarmonyCmdAnalogousCount(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newHarmonyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"analogous", "royalblue", "--count", "5", "--format", "txt"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("got %d lines, want 5\n%s", len(lines), stdout.String())
	}
}

func TestHarmonyCmdRejectsUnknownType(t *testing.T) {
	cmd := newHarmonyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"sextuple", "#3366cc"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown harmony type")
	}
}

func TestHarmonyCmdRejectsBadColor(t *testing.T) {
	cmd := newHarmonyCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"triadic", "not-a-color"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unparseable color")
	}
}
