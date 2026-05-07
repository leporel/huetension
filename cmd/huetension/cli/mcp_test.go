package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestMCPListToolsText verifies --list-tools prints the catalogue in text
// mode without spinning up a transport. Slice A's only registered tool is
// color.convert.
func TestMCPListToolsText(t *testing.T) {
	stdout := withStdoutBuffer(t)
	cmd := newMCPCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--list-tools"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "color.convert") {
		t.Errorf("expected color.convert in --list-tools output, got:\n%s", out)
	}
}

func TestMCPListToolsJSON(t *testing.T) {
	stdout := withStdoutBuffer(t)
	cmd := newMCPCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--list-tools", "--list-format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"name": "color.convert"`) {
		t.Errorf("expected JSON entry for color.convert, got:\n%s", out)
	}
}

func TestMCPHTTPNonLoopbackRequiresAuth(t *testing.T) {
	// Non-loopback bind without --auth-token must refuse to start, even
	// before opening the listener. Use --address 0.0.0.0:0 so even on
	// systems where 0.0.0.0:N would otherwise be available, the refusal
	// happens early.
	cmd := newMCPCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--transport", "http", "--address", "0.0.0.0:0"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected refusal for non-loopback bind without --auth-token")
	} else if !strings.Contains(err.Error(), "non-loopback") {
		t.Errorf("error %q should mention non-loopback", err)
	}
}

func TestMCPRejectsUnknownTransport(t *testing.T) {
	cmd := newMCPCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--transport", "fictional"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown transport")
	}
}

func TestMCPRejectsUnknownToolInEnable(t *testing.T) {
	cmd := newMCPCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--list-tools", "--enable", "fictional.tool"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown tool in --enable")
	}
}

// TestMCPRejectsLooseRootOnNetworkTransport pins the foot-gun guard:
// running HTTP/SSE with the operator clearing --read-only=false but
// leaving --root at the default "." should refuse to start. The auto-
// defaults would have flipped ReadOnly back on, so this only triggers
// when the operator explicitly takes the safety off.
func TestMCPRejectsLooseRootOnNetworkTransport(t *testing.T) {
	cmd := newMCPCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--transport", "http",
		"--address", "127.0.0.1:0",
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

// TestMCPAcceptsLooseRootWhenPinned verifies the same combination is
// accepted once the operator pins --root explicitly. We use --list-tools
// to short-circuit before the listener opens — this test is about
// pre-flight validation order, not transport behaviour.
func TestMCPAcceptsLooseRootWhenPinned(t *testing.T) {
	stdout := withStdoutBuffer(t)
	cmd := newMCPCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--transport", "http",
		"--address", "127.0.0.1:0",
		"--read-only=false",
		"--root", t.TempDir(),
		"--list-tools",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if stdout.Len() == 0 {
		t.Errorf("expected --list-tools output, got empty")
	}
}
