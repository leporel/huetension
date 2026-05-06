package cli

import (
	"bytes"
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
