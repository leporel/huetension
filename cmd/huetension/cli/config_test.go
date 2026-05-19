package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestConfigFileDrivesMCPToolSelection proves config.yaml is actually
// consumed: a `mcp.disable` entry in the file removes that tool from the
// registered set, observed via `huetension mcp --list-tools`. The same
// `mcp:` section feeds `huetension serve` (which has no --disable flag).
//
// viper is a process-global singleton — the test resets it (and the
// --data-dir global) on entry and exit so it neither inherits nor leaks
// config state across the suite.
func TestConfigFileDrivesMCPToolSelection(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Cleanup(func() { dataDir = "" })

	dir := t.TempDir()
	const cfg = "mcp:\n  disable:\n    - color.convert\n"
	if err := os.WriteFile(filepath.Join(dir, configFilename), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write %s: %v", configFilename, err)
	}

	stdout := withStdoutBuffer(t)
	root := newRootCmd()
	root.SetArgs([]string{"mcp", "--data-dir", dir, "--list-tools"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, "color.convert") {
		t.Errorf("color.convert disabled in %s but still listed:\n%s", configFilename, out)
	}
	if !strings.Contains(out, "harmony.generate") {
		t.Errorf("non-disabled tools should still appear:\n%s", out)
	}
}

// TestConfigFileFlagOverride confirms the precedence order: an explicit
// --enable on the command line wins over the config file's mcp.enable.
func TestConfigFileFlagOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Cleanup(func() { dataDir = "" })

	dir := t.TempDir()
	// The file would whitelist only color.convert …
	const cfg = "mcp:\n  enable:\n    - color.convert\n"
	if err := os.WriteFile(filepath.Join(dir, configFilename), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write %s: %v", configFilename, err)
	}

	stdout := withStdoutBuffer(t)
	root := newRootCmd()
	// … but an explicit --enable flag overrides it to harmony.generate.
	root.SetArgs([]string{"mcp", "--data-dir", dir, "--enable", "harmony.generate", "--list-tools"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "harmony.generate") {
		t.Errorf("--enable harmony.generate should win over config:\n%s", out)
	}
	if strings.Contains(out, "color.convert") {
		t.Errorf("config mcp.enable should be overridden by the flag:\n%s", out)
	}
}
