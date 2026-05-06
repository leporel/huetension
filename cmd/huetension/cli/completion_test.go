package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCmdGeneratesScripts(t *testing.T) {
	cases := map[string]string{
		"bash":       "_huetension",
		"zsh":        "compdef",
		"fish":       "complete",
		"powershell": "Register-ArgumentCompleter",
	}
	for shell, marker := range cases {
		stdout := withStdoutBuffer(t)

		root := newRootCmd()
		root.SetArgs([]string{"completion", shell})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		if err := root.Execute(); err != nil {
			t.Errorf("%s: %v", shell, err)
			continue
		}
		if !strings.Contains(stdout.String(), marker) {
			t.Errorf("%s output missing marker %q\n%s", shell, marker, stdout.String())
		}
	}
}

func TestCompletionCmdRejectsUnknownShell(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"completion", "tcsh"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.Execute(); err == nil {
		t.Errorf("expected error for unsupported shell")
	}
}
