package tools

import (
	"context"
	"testing"
)

// TestHandleColorConvertAll exercises the all-formats branch end-to-end
// without going through the SDK — handler is a plain typed function so we
// can call it directly. Catches regressions in the envelope shape and the
// formats map population.
func TestHandleColorConvertAll(t *testing.T) {
	_, out, err := handleColorConvert(context.Background(), nil, ColorConvertParams{Color: "red"})
	if err != nil {
		t.Fatalf("handleColorConvert: %v", err)
	}
	if out.Schema != schemaVersion {
		t.Errorf("schema = %q, want %q", out.Schema, schemaVersion)
	}
	if out.Tool != "color.convert" {
		t.Errorf("tool = %q, want color.convert", out.Tool)
	}
	if out.Params.To != "all" {
		t.Errorf("params.to = %q, want all (default)", out.Params.To)
	}
	if out.Result.Hex != "#ff0000" {
		t.Errorf("hex = %q, want #ff0000", out.Result.Hex)
	}
	if got := out.Result.Formats["hex"]; got != "#ff0000" {
		t.Errorf("formats[hex] = %q, want #ff0000", got)
	}
	if _, ok := out.Result.Formats["oklch"]; !ok {
		t.Errorf("formats missing oklch")
	}
	if out.Result.Value != "" {
		t.Errorf("value should be empty in all-mode, got %q", out.Result.Value)
	}
}

func TestHandleColorConvertSingle(t *testing.T) {
	_, out, err := handleColorConvert(context.Background(), nil, ColorConvertParams{Color: "royalblue", To: "hex"})
	if err != nil {
		t.Fatalf("handleColorConvert: %v", err)
	}
	if out.Result.Value != "#4169e1" {
		t.Errorf("value = %q, want #4169e1", out.Result.Value)
	}
	if out.Result.Formats != nil {
		t.Errorf("formats should be nil in single-format mode, got %v", out.Result.Formats)
	}
}

func TestHandleColorConvertParseError(t *testing.T) {
	_, _, err := handleColorConvert(context.Background(), nil, ColorConvertParams{Color: "not-a-color"})
	if err == nil {
		t.Errorf("expected parse error")
	}
}

func TestHandleColorConvertUnknownFormat(t *testing.T) {
	_, _, err := handleColorConvert(context.Background(), nil, ColorConvertParams{Color: "red", To: "fictional"})
	if err == nil {
		t.Errorf("expected unknown-format error")
	}
}
