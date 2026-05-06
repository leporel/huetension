package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCSSCmdVarsDialect(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newCSSCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "#00ff00", "--name", "brand"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{":root {", "--brand-1: #ff0000;", "--brand-2: #00ff00;", "}"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

func TestCSSCmdSCSSDialect(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newCSSCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#3366cc", "--kind", "scss", "--name", "palette"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"$palette-1: #3366cc;", "$palette-list:"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

func TestCSSCmdLESSDialect(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newCSSCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#3366cc", "--kind", "less", "--name", "palette"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"@palette-1: #3366cc;", "@palette-list:"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

func TestCSSCmdRejectsUnknownKind(t *testing.T) {
	cmd := newCSSCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "--kind", "stylus"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown --kind")
	}
}

func TestCSSCmdReadsStdin(t *testing.T) {
	withStdinReader(t, "#ff0000\n#00ff00\n")
	stdout := withStdoutBuffer(t)

	cmd := newCSSCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "--color-1: #ff0000;") {
		t.Errorf("stdin colors not rendered:\n%s", got)
	}
}
