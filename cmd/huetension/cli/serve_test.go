package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestServeRejectsLooseRootOnNonLoopback pins the shared sandbox guard as
// reached through the `serve` command: a non-loopback bind with the
// operator clearing --read-only=false but leaving --root at the default
// "." must refuse to start, before any listener opens. Mirrors the same
// guard on `huetension mcp`.
func TestServeRejectsLooseRootOnNonLoopback(t *testing.T) {
	cmd := newServeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--address", "0.0.0.0:0",
		"--auth-token", "secret",
		"--read-only=false",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected refusal when --read-only=false without explicit --root")
	}
	if !strings.Contains(err.Error(), "--root") {
		t.Errorf("error %q should mention --root", err)
	}
}

// TestServeRejectsStdioTransport confirms the serve command surfaces the
// invalid-transport error: stdio cannot run on a shared HTTP listener.
// A loopback bind keeps the sandbox guard quiet so the transport check is
// what fails.
func TestServeRejectsStdioTransport(t *testing.T) {
	cmd := newServeCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--address", "127.0.0.1:0",
		"--mcp-transport", "stdio",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected refusal for --mcp-transport stdio")
	}
	if !strings.Contains(err.Error(), "stdio") {
		t.Errorf("error %q should mention stdio", err)
	}
}
