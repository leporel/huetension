package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestContrastCmdWCAG21(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newContrastCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#000000", "#ffffff"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["algo"] != "wcag21" {
		t.Errorf("algo = %v, want wcag21", got["algo"])
	}
	if ratio, _ := got["ratio"].(float64); ratio < 20 {
		t.Errorf("ratio = %v, want ~21:1 for black-on-white", got["ratio"])
	}
	if pass, _ := got["aaa"].(bool); !pass {
		t.Errorf("aaa = false for black-on-white, expected pass")
	}
}

func TestContrastCmdAPCA(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newContrastCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#000000", "#ffffff", "--algo", "apca"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["algo"] != "apca" {
		t.Errorf("algo = %v, want apca", got["algo"])
	}
}

func TestContrastCmdRejectsUnknownAlgo(t *testing.T) {
	cmd := newContrastCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#000", "#fff", "--algo", "iso-9001"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for unknown algo")
	}
}

func TestContrastCmdSuggestWrapsResult(t *testing.T) {
	stdout := withStdoutBuffer(t)

	cmd := newContrastCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	// Mid-grey on white fails AA — the suggestion should find a darker
	// passing lightness and wrap the score under "score".
	cmd.SetArgs([]string{"#888888", "#ffffff", "--suggest"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	score, ok := got["score"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'score' object, got %v", got["score"])
	}
	if score["algo"] != "wcag21" {
		t.Errorf("score.algo = %v, want wcag21", score["algo"])
	}
	suggest, ok := got["suggest"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'suggest' object, got %v", got["suggest"])
	}
	if suggest["target"] != 4.5 {
		t.Errorf("suggest.target = %v, want 4.5 (default for wcag21)", suggest["target"])
	}
	suggested, ok := suggest["suggested"].(map[string]any)
	if !ok {
		t.Fatalf("expected suggest.suggested object, got %v", suggest["suggested"])
	}
	if _, ok := suggested["hex"].(string); !ok {
		t.Errorf("suggest.suggested.hex missing or wrong type: %v", suggested["hex"])
	}
}

func TestContrastCmdSuggestRejectsBadTarget(t *testing.T) {
	cmd := newContrastCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"#888", "#fff", "--suggest", "--target", "-1"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("negative --target should error")
	}
}
