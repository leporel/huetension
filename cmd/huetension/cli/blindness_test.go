package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBlindnessCmdSingleKind(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newBlindnessCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "#00ff00", "#0000ff", "--kind", "deutan", "--format", "txt"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3\n%s", len(lines), stdout.String())
	}
}

func TestBlindnessCmdAllKinds(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newBlindnessCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "--kind", "all"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string][]map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	for _, kind := range []string{"protan", "deutan", "tritan", "achroma"} {
		entries, ok := got[kind]
		if !ok {
			t.Errorf("missing kind %q in output", kind)
			continue
		}
		if len(entries) != 1 {
			t.Errorf("kind %q: got %d entries, want 1", kind, len(entries))
		}
	}
}

func TestBlindnessCmdRejectsUnknownKind(t *testing.T) {
	cmd := newBlindnessCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "--kind", "marsupial"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown kind")
	}
}
