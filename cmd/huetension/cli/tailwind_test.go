package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestTailwindCmdFlat(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newTailwindCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "#00ff00", "--name", "brand"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{`module.exports = {`, `"brand-1": "#ff0000",`, `"brand-2": "#00ff00",`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

func TestTailwindCmdShades5(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newTailwindCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#3366cc", "--shades", "5", "--name", "brand"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{`"brand-1":`, `"100":`, `"300":`, `"500":`, `"700":`, `"900":`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
	// Should NOT contain shade keys outside the 5-stop list.
	for _, unwanted := range []string{`"50":`, `"200":`, `"400":`} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q in 5-shade output\n%s", unwanted, got)
		}
	}
}

func TestTailwindCmdShadesAuto(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newTailwindCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#3366cc", "--shades", "auto"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	// auto = 10 shades = 50/100/200/.../900.
	for _, want := range []string{`"50":`, `"100":`, `"500":`, `"900":`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

func TestTailwindCmdRejectsBadShades(t *testing.T) {
	cmd := newTailwindCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#ff0000", "--shades", "purple"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for non-numeric --shades")
	}
}

func TestTailwindCmdReadsStdin(t *testing.T) {
	withStdinReader(t, "#ff0000\n")
	stdout := withStdoutBuffer(t)

	cmd := newTailwindCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), `"color-1": "#ff0000"`) {
		t.Errorf("stdin not rendered:\n%s", stdout.String())
	}
}
