package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSortCmdLuminance(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newSortCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ffffff", "#000000", "#888888"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "#000000" {
		t.Errorf("first = %q, want #000000 (darkest)", lines[0])
	}
	if lines[2] != "#ffffff" {
		t.Errorf("last = %q, want #ffffff (lightest)", lines[2])
	}
}

func TestSortCmdReverse(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newSortCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ffffff", "#000000", "--reverse"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if lines[0] != "#ffffff" {
		t.Errorf("first = %q, want #ffffff (light first when reversed)", lines[0])
	}
}

func TestSortCmdJSON(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newSortCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "#00ff00", "#0000ff", "--format", "json", "--by", "hue"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 3 {
		t.Errorf("got %d entries, want 3", len(got))
	}
}

func TestSortCmdReadsStdin(t *testing.T) {
	withStdinReader(t, "#888888\n#000000\n#ffffff\n")
	stdout := withStdoutBuffer(t)

	cmd := newSortCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "#000000" {
		t.Errorf("first = %q, want #000000", lines[0])
	}
}

func TestSortCmdNoInput(t *testing.T) {
	withStdinReader(t, "")

	cmd := newSortCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error when no inputs given")
	}
}
