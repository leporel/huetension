package tools

import (
	"context"
	"testing"
)

// TestColorSort sorts white→black by luminance reversed → ascending
// luminance is dark→light, reverse=true gives light→dark.
func TestColorSort(t *testing.T) {
	_, out, err := handleColorSort(context.Background(), nil, ColorSortParams{
		Colors:  []string{"black", "white", "red"},
		By:      "luminance",
		Reverse: true,
	})
	if err != nil {
		t.Fatalf("handleColorSort: %v", err)
	}
	if got := out.Result.Colors[0].Hex; got != "#ffffff" {
		t.Errorf("first = %q, want #ffffff (light first when reverse)", got)
	}
	if got := out.Result.Colors[2].Hex; got != "#000000" {
		t.Errorf("last = %q, want #000000", got)
	}
	if out.Schema != schemaVersion {
		t.Errorf("schema = %q", out.Schema)
	}
}

func TestColorSortEmptyInput(t *testing.T) {
	_, _, err := handleColorSort(context.Background(), nil, ColorSortParams{})
	if err == nil {
		t.Errorf("expected error for empty input")
	}
}

func TestHarmonyGenerateComplementary(t *testing.T) {
	_, out, err := handleHarmonyGenerate(context.Background(), nil, HarmonyGenerateParams{
		Type: "complementary",
		Base: "red",
	})
	if err != nil {
		t.Fatalf("handleHarmonyGenerate: %v", err)
	}
	if out.Result.Size != 2 {
		t.Errorf("size = %d, want 2", out.Result.Size)
	}
	if out.Result.Colors[0].Hex != "#ff0000" {
		t.Errorf("first = %q, want #ff0000", out.Result.Colors[0].Hex)
	}
	// On the RYB artist wheel red's complement is green, not cyan.
	if out.Result.Colors[1].Hex != "#00ff4d" {
		t.Errorf("second = %q, want #00ff4d (RYB green complement)", out.Result.Colors[1].Hex)
	}
}

func TestHarmonyGenerateCountExpansion(t *testing.T) {
	_, out, err := handleHarmonyGenerate(context.Background(), nil, HarmonyGenerateParams{
		Type:  "complementary",
		Base:  "red",
		Count: 5,
	})
	if err != nil {
		t.Fatalf("handleHarmonyGenerate: %v", err)
	}
	if out.Result.Size != 5 {
		t.Errorf("size = %d, want 5 (extra-slot expansion)", out.Result.Size)
	}
}

func TestHarmonyGenerateUnknownType(t *testing.T) {
	_, _, err := handleHarmonyGenerate(context.Background(), nil, HarmonyGenerateParams{
		Type: "fictional",
		Base: "red",
	})
	if err == nil {
		t.Errorf("expected error for unknown type")
	}
}

func TestGradientGenerateFromTo(t *testing.T) {
	_, out, err := handleGradientGenerate(context.Background(), nil, GradientGenerateParams{
		From:  "red",
		To:    "blue",
		Steps: 5,
	})
	if err != nil {
		t.Fatalf("handleGradientGenerate: %v", err)
	}
	if out.Result.Size != 5 {
		t.Errorf("size = %d, want 5", out.Result.Size)
	}
	// First and last should be bit-exact endpoints.
	if out.Result.Colors[0].Hex != "#ff0000" {
		t.Errorf("first = %q, want #ff0000", out.Result.Colors[0].Hex)
	}
	if out.Result.Colors[4].Hex != "#0000ff" {
		t.Errorf("last = %q, want #0000ff", out.Result.Colors[4].Hex)
	}
}

func TestGradientGenerateMultiStop(t *testing.T) {
	_, out, err := handleGradientGenerate(context.Background(), nil, GradientGenerateParams{
		Stops: []string{"red", "lime", "blue"},
		Steps: 5,
	})
	if err != nil {
		t.Fatalf("handleGradientGenerate: %v", err)
	}
	if out.Result.Size != 5 {
		t.Errorf("size = %d, want 5", out.Result.Size)
	}
}

func TestGradientGenerateMissingInputs(t *testing.T) {
	_, _, err := handleGradientGenerate(context.Background(), nil, GradientGenerateParams{Steps: 5})
	if err == nil {
		t.Errorf("expected error when neither from/to nor stops are set")
	}
}

func TestContrastCheckWCAG(t *testing.T) {
	_, out, err := handleContrastCheck(context.Background(), nil, ContrastCheckParams{
		FG:   "black",
		BG:   "white",
		Algo: "wcag21",
	})
	if err != nil {
		t.Fatalf("handleContrastCheck: %v", err)
	}
	if out.Result.WCAG21 == nil {
		t.Fatalf("WCAG21 result missing")
	}
	if out.Result.WCAG21.Ratio < 20 {
		t.Errorf("ratio = %.2f, want ≈ 21", out.Result.WCAG21.Ratio)
	}
	if !out.Result.WCAG21.AAA {
		t.Errorf("black-on-white should pass AAA")
	}
}

func TestContrastCheckBoth(t *testing.T) {
	_, out, err := handleContrastCheck(context.Background(), nil, ContrastCheckParams{
		FG:   "black",
		BG:   "white",
		Algo: "both",
	})
	if err != nil {
		t.Fatalf("handleContrastCheck: %v", err)
	}
	if out.Result.WCAG21 == nil || out.Result.APCA == nil {
		t.Errorf("both algos should be present")
	}
}

func TestContrastCheckUnknownAlgo(t *testing.T) {
	_, _, err := handleContrastCheck(context.Background(), nil, ContrastCheckParams{
		FG:   "black",
		BG:   "white",
		Algo: "fictional",
	})
	if err == nil {
		t.Errorf("expected error for unknown algo")
	}
}

func TestContrastCheckSuggestSingleAlgo(t *testing.T) {
	_, out, err := handleContrastCheck(context.Background(), nil, ContrastCheckParams{
		FG:      "#888888",
		BG:      "white",
		Algo:    "wcag21",
		Suggest: true,
	})
	if err != nil {
		t.Fatalf("handleContrastCheck: %v", err)
	}
	if out.Result.Suggest == nil {
		t.Fatalf("expected Suggest result")
	}
	if out.Result.Suggest.Target != 4.5 {
		t.Errorf("default wcag21 target = %v, want 4.5", out.Result.Suggest.Target)
	}
	if out.Result.Suggest.Suggested == nil {
		t.Fatalf("mid-grey on white should have a passing suggestion")
	}
}

func TestContrastCheckSuggestRejectsBoth(t *testing.T) {
	_, _, err := handleContrastCheck(context.Background(), nil, ContrastCheckParams{
		FG:      "black",
		BG:      "white",
		Algo:    "both",
		Suggest: true,
	})
	if err == nil {
		t.Errorf("suggest=true with algo=both should error")
	}
}

func TestContrastCheckSuggestRespectsTarget(t *testing.T) {
	_, out, err := handleContrastCheck(context.Background(), nil, ContrastCheckParams{
		FG:      "#000000",
		BG:      "#ffffff",
		Algo:    "apca",
		Suggest: true,
		Target:  75,
	})
	if err != nil {
		t.Fatalf("handleContrastCheck: %v", err)
	}
	if out.Result.Suggest == nil || out.Result.Suggest.Target != 75 {
		t.Errorf("expected target=75 echoed back, got %+v", out.Result.Suggest)
	}
}

func TestBlindnessSimulateAll(t *testing.T) {
	_, out, err := handleBlindnessSimulate(context.Background(), nil, BlindnessSimulateParams{
		Colors: []string{"red", "green", "blue"},
		Kind:   "all",
	})
	if err != nil {
		t.Fatalf("handleBlindnessSimulate: %v", err)
	}
	if len(out.Result.Variants) != 4 {
		t.Errorf("variants = %d, want 4 (protan/deutan/tritan/achroma)", len(out.Result.Variants))
	}
	for _, v := range out.Result.Variants {
		if len(v.Colors) != 3 {
			t.Errorf("kind %s: got %d colors, want 3", v.Kind, len(v.Colors))
		}
	}
}

func TestBlindnessSimulateSingleKind(t *testing.T) {
	_, out, err := handleBlindnessSimulate(context.Background(), nil, BlindnessSimulateParams{
		Colors: []string{"red"},
		Kind:   "protan",
	})
	if err != nil {
		t.Fatalf("handleBlindnessSimulate: %v", err)
	}
	if len(out.Result.Variants) != 1 || out.Result.Variants[0].Kind != "protan" {
		t.Errorf("expected single protan variant, got %+v", out.Result.Variants)
	}
}

func TestBlindnessSimulateUnknownKind(t *testing.T) {
	_, _, err := handleBlindnessSimulate(context.Background(), nil, BlindnessSimulateParams{
		Colors: []string{"red"},
		Kind:   "fictional",
	})
	if err == nil {
		t.Errorf("expected error for unknown kind")
	}
}

func TestPaletteRandomSeeded(t *testing.T) {
	a, _, err := handlePaletteRandom(context.Background(), nil, PaletteRandomParams{Count: 5, Seed: 42})
	_ = a
	if err != nil {
		t.Fatalf("handlePaletteRandom: %v", err)
	}
	_, out1, _ := handlePaletteRandom(context.Background(), nil, PaletteRandomParams{Count: 5, Seed: 42})
	_, out2, _ := handlePaletteRandom(context.Background(), nil, PaletteRandomParams{Count: 5, Seed: 42})
	if len(out1.Result.Colors) != 5 {
		t.Errorf("size = %d, want 5", len(out1.Result.Colors))
	}
	for i := range out1.Result.Colors {
		if out1.Result.Colors[i].Hex != out2.Result.Colors[i].Hex {
			t.Errorf("seed=42 not deterministic at %d: %s vs %s",
				i, out1.Result.Colors[i].Hex, out2.Result.Colors[i].Hex)
		}
	}
}

func TestPaletteRandomHarmony(t *testing.T) {
	_, out, err := handlePaletteRandom(context.Background(), nil, PaletteRandomParams{
		Count:   5,
		Seed:    100,
		Harmony: "triadic",
	})
	if err != nil {
		t.Fatalf("handlePaletteRandom: %v", err)
	}
	if out.Result.Size != 5 {
		t.Errorf("size = %d, want 5 (extra-slot expansion of 3-anchor triadic)", out.Result.Size)
	}
}

func TestPaletteRandomUnknownHarmony(t *testing.T) {
	_, _, err := handlePaletteRandom(context.Background(), nil, PaletteRandomParams{
		Count:   5,
		Harmony: "fictional",
	})
	if err == nil {
		t.Errorf("expected error for unknown harmony")
	}
}
