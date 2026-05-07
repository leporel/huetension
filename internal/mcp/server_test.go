package mcp

import (
	"context"
	"encoding/json"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestEndToEndStdioOverInMemoryTransport spins the server up against an
// in-memory transport pair, then drives it through an SDK client. Verifies
// the full round-trip: registration → tools/list → tools/call → structured
// content envelope.
func TestEndToEndOverInMemoryTransport(t *testing.T) {
	ctx := context.Background()

	srv, _, err := Build(Config{Version: "test"})
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

	// tools/list should expose color.convert.
	list, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var found bool
	for _, tl := range list.Tools {
		if tl.Name == "color.convert" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("color.convert not in tools/list: %+v", list.Tools)
	}

	// tools/call color.convert with to=hex should round-trip a known color.
	res, err := clientSession.CallTool(ctx, &sdk.CallToolParams{
		Name:      "color.convert",
		Arguments: map[string]any{"color": "royalblue", "to": "hex"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned IsError: %+v", res.Content)
	}
	if res.StructuredContent == nil {
		t.Fatalf("expected StructuredContent, got nil")
	}

	// The SDK delivers StructuredContent as the original Go struct on the
	// server side; over the wire it is a json.RawMessage / map. We marshal-
	// roundtrip it to an inspectable map.
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var env struct {
		Schema string `json:"schema"`
		Tool   string `json:"tool"`
		Result struct {
			Hex   string `json:"hex"`
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\nraw: %s", err, raw)
	}
	if env.Schema != "huetension/v1" {
		t.Errorf("schema = %q, want huetension/v1", env.Schema)
	}
	if env.Tool != "color.convert" {
		t.Errorf("tool = %q, want color.convert", env.Tool)
	}
	if env.Result.Hex != "#4169e1" {
		t.Errorf("result.hex = %q, want #4169e1", env.Result.Hex)
	}
	if env.Result.Value != "#4169e1" {
		t.Errorf("result.value = %q, want #4169e1", env.Result.Value)
	}
}

func TestResolveEnabledDefault(t *testing.T) {
	enabled, err := ResolveEnabled(Config{})
	if err != nil {
		t.Fatalf("ResolveEnabled: %v", err)
	}
	if len(enabled) == 0 {
		t.Fatalf("expected default-enabled tools, got 0")
	}
}

func TestResolveEnabledWhitelist(t *testing.T) {
	enabled, err := ResolveEnabled(Config{Enable: []string{"color.convert"}})
	if err != nil {
		t.Fatalf("ResolveEnabled: %v", err)
	}
	if len(enabled) != 1 || enabled[0].Name != "color.convert" {
		t.Errorf("expected [color.convert], got %v", enabled)
	}
}

func TestResolveEnabledBlacklist(t *testing.T) {
	enabled, err := ResolveEnabled(Config{Disable: []string{"color.convert"}})
	if err != nil {
		t.Fatalf("ResolveEnabled: %v", err)
	}
	for _, d := range enabled {
		if d.Name == "color.convert" {
			t.Errorf("color.convert should be disabled")
		}
	}
}

func TestResolveEnabledUnknownTool(t *testing.T) {
	if _, err := ResolveEnabled(Config{Enable: []string{"fictional.tool"}}); err == nil {
		t.Errorf("expected error for unknown tool in --enable")
	}
	if _, err := ResolveEnabled(Config{Disable: []string{"fictional.tool"}}); err == nil {
		t.Errorf("expected error for unknown tool in --disable")
	}
}

// TestSelectorNamespacing covers the "tools:NAME / resources:URI /
// prompts:NAME / kind:* / bare-name → tool" parser. Drives all three
// Resolve* paths so back-compat (bare names still mean tools) and the
// new namespaced syntax are pinned together.
func TestSelectorNamespacing(t *testing.T) {
	t.Run("bare name is a tool", func(t *testing.T) {
		got, err := ResolveEnabled(Config{Enable: []string{"color.convert"}})
		if err != nil {
			t.Fatalf("ResolveEnabled: %v", err)
		}
		if len(got) != 1 || got[0].Name != "color.convert" {
			t.Errorf("expected only color.convert, got %v", got)
		}
		// Resources/prompts are not constrained by a tool-only --enable.
		rs, err := ResolveEnabledResources(Config{Enable: []string{"color.convert"}})
		if err != nil {
			t.Fatalf("ResolveEnabledResources: %v", err)
		}
		if len(rs) == 0 {
			t.Errorf("resources should default-enable when --enable only names tools")
		}
		ps, err := ResolveEnabledPrompts(Config{Enable: []string{"color.convert"}})
		if err != nil {
			t.Fatalf("ResolveEnabledPrompts: %v", err)
		}
		if len(ps) == 0 {
			t.Errorf("prompts should default-enable when --enable only names tools")
		}
	})

	t.Run("resources wildcard disable", func(t *testing.T) {
		rs, err := ResolveEnabledResources(Config{Disable: []string{"resources:*"}})
		if err != nil {
			t.Fatalf("ResolveEnabledResources: %v", err)
		}
		if len(rs) != 0 {
			t.Errorf("resources:* in --disable should drop all resources, got %d", len(rs))
		}
	})

	t.Run("prompts targeted disable", func(t *testing.T) {
		ps, err := ResolveEnabledPrompts(Config{Disable: []string{"prompts:audit-contrast"}})
		if err != nil {
			t.Fatalf("ResolveEnabledPrompts: %v", err)
		}
		var hasAudit, hasMood bool
		for _, p := range ps {
			switch p.Name {
			case "audit-contrast":
				hasAudit = true
			case "extract-from-mood":
				hasMood = true
			}
		}
		if hasAudit {
			t.Errorf("audit-contrast should be disabled")
		}
		if !hasMood {
			t.Errorf("extract-from-mood should remain enabled")
		}
	})

	t.Run("mixed enable", func(t *testing.T) {
		cfg := Config{Enable: []string{"color.convert", "prompts:extract-from-mood"}}
		got, err := ResolveEnabled(cfg)
		if err != nil {
			t.Fatalf("ResolveEnabled: %v", err)
		}
		if len(got) != 1 || got[0].Name != "color.convert" {
			t.Errorf("tools restricted to color.convert; got %v", got)
		}
		ps, err := ResolveEnabledPrompts(cfg)
		if err != nil {
			t.Fatalf("ResolveEnabledPrompts: %v", err)
		}
		if len(ps) != 1 || ps[0].Name != "extract-from-mood" {
			t.Errorf("prompts restricted to extract-from-mood; got %v", ps)
		}
	})

	t.Run("unknown kind errors", func(t *testing.T) {
		if _, err := ResolveEnabled(Config{Enable: []string{"bogus:thing"}}); err == nil {
			t.Errorf("expected error for unknown selector kind")
		}
	})

	t.Run("unknown resource errors", func(t *testing.T) {
		if _, err := ResolveEnabledResources(Config{Enable: []string{"resources:huetension://no-such"}}); err == nil {
			t.Errorf("expected error for unknown resource URI")
		}
	})

	t.Run("unknown prompt errors", func(t *testing.T) {
		if _, err := ResolveEnabledPrompts(Config{Enable: []string{"prompts:no-such-prompt"}}); err == nil {
			t.Errorf("expected error for unknown prompt name")
		}
	})

	t.Run("resources targeted enable", func(t *testing.T) {
		rs, err := ResolveEnabledResources(Config{Enable: []string{"resources:" + EnvelopeSchemaURI}})
		if err != nil {
			t.Fatalf("ResolveEnabledResources: %v", err)
		}
		if len(rs) != 1 || rs[0].URI != EnvelopeSchemaURI {
			t.Errorf("resources should be restricted to envelope schema; got %v", rs)
		}
	})
}
