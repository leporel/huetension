package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRandomCmdDeterministicWithSeed(t *testing.T) {
	run := func() string {
		stdout := withStdoutBuffer(t)
		cmd := newRandomCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--seed", "42", "--count", "5", "--format", "txt"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		return stdout.String()
	}

	first := run()
	second := run()
	if first != second {
		t.Errorf("--seed should produce identical output:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestRandomCmdRespectsCount(t *testing.T) {
	stdout := withStdoutBuffer(t)
	cmd := newRandomCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--seed", "1", "--count", "8", "--format", "txt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 8 {
		t.Errorf("got %d lines, want 8", len(lines))
	}
}

func TestRandomCmdHarmony(t *testing.T) {
	// Default --count=5 with triadic (3 anchors) → Kuler-style expansion to 5.
	stdout := withStdoutBuffer(t)
	cmd := newRandomCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--seed", "100", "--harmony", "triadic", "--format", "txt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("got %d lines, want 5 (default --count=5 expanded)", len(lines))
	}
}

func TestRandomCmdHarmonyNaturalCount(t *testing.T) {
	// Explicit --count=3 with triadic → exactly the 3 natural anchors.
	stdout := withStdoutBuffer(t)
	cmd := newRandomCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--seed", "100", "--harmony", "triadic", "--count", "3", "--format", "txt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3 for triadic", len(lines))
	}
}

func TestRandomCmdHarmonyCountTooSmall(t *testing.T) {
	// --count below the natural anchor count must error.
	cmd := newRandomCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--seed", "100", "--harmony", "triadic", "--count", "2", "--format", "txt"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for triadic --count=2")
	}
}

func TestRandomCmdRejectsUnknownHarmony(t *testing.T) {
	cmd := newRandomCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--harmony", "fictional"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown harmony")
	}
}
