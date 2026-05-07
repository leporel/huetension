package mcp

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestPromptsListAndGet drives prompts/list and prompts/get over the
// in-memory transport. The list assertion verifies registered metadata
// (arguments + required flag); the get assertions verify argument
// substitution for both prompts.
func TestPromptsListAndGet(t *testing.T) {
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

	list, err := clientSession.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	gotNames := map[string]*sdk.Prompt{}
	for _, p := range list.Prompts {
		gotNames[p.Name] = p
	}
	for _, want := range []string{"extract-from-mood", "audit-contrast"} {
		if _, ok := gotNames[want]; !ok {
			t.Errorf("prompt %q missing from prompts/list (got %v)", want, mapKeys(gotNames))
		}
	}

	if p := gotNames["extract-from-mood"]; p != nil {
		if !hasRequiredArg(p, "mood") {
			t.Errorf("extract-from-mood: 'mood' should be a required argument")
		}
	}
	if p := gotNames["audit-contrast"]; p != nil {
		if !hasRequiredArg(p, "pairs") {
			t.Errorf("audit-contrast: 'pairs' should be a required argument")
		}
	}

	t.Run("extract-from-mood substitutes mood", func(t *testing.T) {
		res, err := clientSession.GetPrompt(ctx, &sdk.GetPromptParams{
			Name:      "extract-from-mood",
			Arguments: map[string]string{"mood": "foggy seaside dawn"},
		})
		if err != nil {
			t.Fatalf("GetPrompt extract-from-mood: %v", err)
		}
		text := joinPromptText(res)
		if !strings.Contains(text, "foggy seaside dawn") {
			t.Errorf("expected mood in prompt text, got:\n%s", text)
		}
		if !strings.Contains(text, "harmony.generate") {
			t.Errorf("expected the prompt to reference harmony.generate, got:\n%s", text)
		}
	})

	t.Run("extract-from-mood missing mood errors", func(t *testing.T) {
		_, err := clientSession.GetPrompt(ctx, &sdk.GetPromptParams{
			Name:      "extract-from-mood",
			Arguments: map[string]string{},
		})
		if err == nil {
			t.Errorf("expected an error when mood is missing")
		}
	})

	t.Run("audit-contrast substitutes pairs", func(t *testing.T) {
		pairs := "#fff:#222, royalblue:#fafafa"
		res, err := clientSession.GetPrompt(ctx, &sdk.GetPromptParams{
			Name:      "audit-contrast",
			Arguments: map[string]string{"pairs": pairs},
		})
		if err != nil {
			t.Fatalf("GetPrompt audit-contrast: %v", err)
		}
		text := joinPromptText(res)
		if !strings.Contains(text, pairs) {
			t.Errorf("expected pairs in prompt text, got:\n%s", text)
		}
		if !strings.Contains(text, "contrast.check") {
			t.Errorf("expected the prompt to reference contrast.check, got:\n%s", text)
		}
	})
}

func hasRequiredArg(p *sdk.Prompt, name string) bool {
	for _, a := range p.Arguments {
		if a.Name == name {
			return a.Required
		}
	}
	return false
}

// joinPromptText concatenates the TextContent bodies in res so tests can
// match against a single string rather than walking every message.
func joinPromptText(res *sdk.GetPromptResult) string {
	var b strings.Builder
	for _, m := range res.Messages {
		if tc, ok := m.Content.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
