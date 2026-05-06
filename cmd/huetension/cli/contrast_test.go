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
