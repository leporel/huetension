package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/httputil"
)

// TestLoggingMiddlewareEmitsCallEntry drives a tools/call through the
// in-memory transport with a captured logger, then asserts the JSON line
// carries the fields we promise (method, subject=tool name, duration).
// It also verifies tool *arguments* are not in the log line — that is
// the documented privacy contract for the default handler.
func TestLoggingMiddlewareEmitsCallEntry(t *testing.T) {
	ctx := context.Background()

	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	srv, _, err := Build(Config{Version: "test", Logger: logger})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	t1, t2 := sdk.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	const sentinel = "thisShouldNotAppearInLogs"
	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name:      "color.convert",
		Arguments: map[string]any{"color": sentinel + "-but-this-is-actually-invalid", "to": "hex"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	_ = res

	entries := decodeJSONLines(t, buf.Bytes())
	got := findEntry(entries, "method", "tools/call")
	if got == nil {
		t.Fatalf("no 'tools/call' log entry found in:\n%s", buf.String())
	}
	if got["msg"] != "mcp.call" {
		t.Errorf("msg = %v, want mcp.call", got["msg"])
	}
	if got["subject"] != "color.convert" {
		t.Errorf("subject = %v, want color.convert", got["subject"])
	}
	if _, ok := got["duration"]; !ok {
		t.Errorf("expected a duration field in the log entry")
	}
	if strings.Contains(buf.String(), sentinel) {
		t.Errorf("tool argument leaked into log: %s", buf.String())
	}
}

// TestLoggingMiddlewareLogsErrors verifies a tool that returns an error
// surfaces it at error level so monitoring catches it. We invoke
// color.convert with an unparseable color so the handler returns an
// IsError result; the SDK reports a successful method dispatch (no Go
// error), so the middleware should log at info — but if we feed an
// unknown tool, the dispatch itself fails with an error that the
// middleware must surface.
func TestLoggingMiddlewareLogsUnknownToolError(t *testing.T) {
	ctx := context.Background()

	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	srv, _, err := Build(Config{Version: "test", Logger: logger})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	t1, t2 := sdk.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	_, _ = clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name: "no.such.tool",
	})

	entries := decodeJSONLines(t, buf.Bytes())
	var sawError bool
	for _, e := range entries {
		if e["level"] == "ERROR" && e["method"] == "tools/call" {
			sawError = true
			if _, ok := e["error"]; !ok {
				t.Errorf("error-level entry missing 'error' field: %v", e)
			}
		}
	}
	if !sawError {
		t.Errorf("expected an error-level tools/call entry, got:\n%s", buf.String())
	}
}

// TestWithAccessLog drives a tiny HTTP handler through withAccessLog and
// verifies the captured log line carries method/path/status/duration.
// We do not bring up a real listener — httptest.NewRecorder is enough to
// exercise the wrapper end-to-end.
func TestWithAccessLog(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "ok")
	})
	wrapped := httputil.WithAccessLog(logger, "mcp.http", inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp/foo", nil)
	wrapped.ServeHTTP(rec, req)

	entries := decodeJSONLines(t, buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d:\n%s", len(entries), buf.String())
	}
	got := entries[0]
	if got["msg"] != "mcp.http" {
		t.Errorf("msg = %v, want mcp.http", got["msg"])
	}
	if got["method"] != "POST" {
		t.Errorf("method = %v, want POST", got["method"])
	}
	if got["path"] != "/mcp/foo" {
		t.Errorf("path = %v, want /mcp/foo", got["path"])
	}
	// JSON numbers decode as float64 even when the original Go value was an int.
	if status, _ := got["status"].(float64); int(status) != http.StatusTeapot {
		t.Errorf("status = %v, want %d", got["status"], http.StatusTeapot)
	}
}

// TestWithAccessLogNilLoggerIsNoop guards against panics when a caller
// disables logging by passing nil. The wrapper should hand the request
// straight through.
func TestWithAccessLogNilLoggerIsNoop(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	wrapped := httputil.WithAccessLog(nil, "mcp.http", inner)

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Errorf("inner handler was not invoked")
	}
}

// decodeJSONLines splits buf at newlines and unmarshals each non-empty
// line as a JSON object. Slog's JSON handler emits one entry per line,
// so this is the natural unit of inspection.
func decodeJSONLines(t *testing.T, buf []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range bytes.SplitSeq(buf, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func findEntry(entries []map[string]any, key string, want any) map[string]any {
	for _, e := range entries {
		if e[key] == want {
			return e
		}
	}
	return nil
}
