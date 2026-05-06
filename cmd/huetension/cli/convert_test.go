package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// withStdinReader swaps stdinReader for a strings.Reader so stdin-fed
// commands can be tested without touching os.Stdin.
func withStdinReader(t *testing.T, content string) {
	t.Helper()
	prev := stdinReader
	stdinReader = strings.NewReader(content)
	t.Cleanup(func() { stdinReader = prev })
}

func TestConvertCmdAllFormatsText(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newConvertCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#3366cc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"#3366cc", "rgb(", "hsl(", "lab(", "oklch("} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestConvertCmdSingleFormat(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newConvertCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "--to", "hex"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "#ff0000" {
		t.Errorf("got %q, want %q", stdout.String(), "#ff0000")
	}
}

func TestConvertCmdJSONOutput(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newConvertCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#3366cc", "--format", "json", "--to", "rgb"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0]["input"] != "#3366cc" {
		t.Errorf("input = %v", got[0]["input"])
	}
	if got[0]["value"] == nil {
		t.Errorf("value field empty")
	}
}

func TestConvertCmdReadsStdin(t *testing.T) {
	withStdinReader(t, "#ff0000\n#00ff00\n# this is a comment\n\n#0000ff\n")
	stdout := withStdoutBuffer(t)

	cmd := newConvertCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--to", "hex"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3 (the # comment line should be skipped):\n%s", len(lines), stdout.String())
	}
}

func TestConvertCmdPartialFailure(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newConvertCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "definitely-not-a-color", "--format", "json"})

	err := cmd.Execute()
	if err == nil {
		t.Errorf("expected partial-failure error")
	}
	// Output should still contain valid JSON for the parseable input
	// plus an error entry for the bad one.
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 2 {
		t.Errorf("got %d entries, want 2", len(got))
	}
}
