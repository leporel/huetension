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
	stdout := withStdoutBuffer(t)
	cmd := newRandomCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--seed", "100", "--harmony", "triadic", "--format", "txt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	// triadic = 3 colors regardless of --count.
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3 for triadic", len(lines))
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
