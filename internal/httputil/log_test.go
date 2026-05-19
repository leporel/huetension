package httputil

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestNewLoggerFormats pins the default and the two handler choices: an
// empty/`text` format yields human-readable "key=value" lines, `json`
// yields one object per line.
func TestNewLoggerFormats(t *testing.T) {
	for _, format := range []string{"", "text", "TEXT"} {
		buf := &bytes.Buffer{}
		lg, err := NewLogger(buf, format, "info")
		if err != nil {
			t.Fatalf("NewLogger(%q): %v", format, err)
		}
		lg.Info("hello", "k", "v")
		out := buf.String()
		if !strings.Contains(out, "msg=hello") || !strings.Contains(out, "k=v") {
			t.Errorf("format %q: want human-readable key=value, got %q", format, out)
		}
		if strings.Contains(out, `"msg"`) {
			t.Errorf("format %q should not be JSON: %q", format, out)
		}
	}

	buf := &bytes.Buffer{}
	lg, err := NewLogger(buf, "json", "info")
	if err != nil {
		t.Fatalf("NewLogger(json): %v", err)
	}
	lg.Info("hello")
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf("json format: want a JSON object, got %q", buf.String())
	}
}

// TestNewLoggerRejectsBadInput confirms an unknown format or level fails
// loudly rather than silently falling back.
func TestNewLoggerRejectsBadInput(t *testing.T) {
	if _, err := NewLogger(io.Discard, "yaml", "info"); err == nil {
		t.Errorf("expected unknown format to be rejected")
	}
	if _, err := NewLogger(io.Discard, "text", "trace"); err == nil {
		t.Errorf("expected unknown level to be rejected")
	}
}

// TestNewLoggerLevelFiltering verifies the level threshold is applied.
func TestNewLoggerLevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	lg, err := NewLogger(buf, "text", "error")
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	lg.Info("filtered-out")
	lg.Error("kept")
	out := buf.String()
	if strings.Contains(out, "filtered-out") {
		t.Errorf("info entry should be filtered at error level: %q", out)
	}
	if !strings.Contains(out, "kept") {
		t.Errorf("error entry missing: %q", out)
	}
}
