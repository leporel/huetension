package extract

import (
	"strings"
	"testing"
)

func TestParseSoftPreset(t *testing.T) {
	t.Run("empty returns empty without error", func(t *testing.T) {
		got, err := ParseSoftPreset("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("whitespace-only returns empty", func(t *testing.T) {
		got, err := ParseSoftPreset("   ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("all canonical names accepted", func(t *testing.T) {
		for _, p := range AllSoftPresets {
			got, err := ParseSoftPreset(string(p))
			if err != nil {
				t.Errorf("%s: %v", p, err)
				continue
			}
			if got != p {
				t.Errorf("%s: got %q", p, got)
			}
		}
	})

	t.Run("case-insensitive", func(t *testing.T) {
		cases := []struct {
			in   string
			want SoftPreset
		}{
			{"Default", SoftPresetDefault},
			{"COLORFUL", SoftPresetColorful},
			{"Bright", SoftPresetBright},
			{"  muted  ", SoftPresetMuted},
			{"DeEp", SoftPresetDeep},
			{"DARK", SoftPresetDark},
		}
		for _, c := range cases {
			got, err := ParseSoftPreset(c.in)
			if err != nil {
				t.Errorf("%q: %v", c.in, err)
				continue
			}
			if got != c.want {
				t.Errorf("%q: got %q, want %q", c.in, got, c.want)
			}
		}
	})

	t.Run("unknown rejected with name list in message", func(t *testing.T) {
		_, err := ParseSoftPreset("bogus")
		if err == nil {
			t.Fatal("expected error for unknown preset")
		}
		msg := err.Error()
		if !strings.Contains(msg, "bogus") {
			t.Errorf("error %q should mention input value", msg)
		}
		for _, p := range AllSoftPresets {
			if !strings.Contains(msg, string(p)) {
				t.Errorf("error %q should list %q", msg, p)
			}
		}
	})
}

func TestSoftPresetMappingDeterministic(t *testing.T) {
	for _, p := range AllSoftPresets {
		a := softPresetMapping(p)
		b := softPresetMapping(p)
		if a != b {
			t.Errorf("%s: mapping not deterministic: %+v vs %+v", p, a, b)
		}
	}
}

func TestSoftPresetMappingCoversAllPresets(t *testing.T) {
	// Every named preset must return a non-zero tuning. Catches the case
	// where someone adds a preset constant but forgets the mapping branch.
	for _, p := range AllSoftPresets {
		tuning := softPresetMapping(p)
		zero := softPresetTuning{}
		if tuning == zero {
			t.Errorf("%s: mapping returned zero tuning — branch missing?", p)
		}
	}
}

func TestSoftPresetMappingInvariants(t *testing.T) {
	for _, p := range AllSoftPresets {
		tuning := softPresetMapping(p)
		if tuning.MinOkL < 0 || tuning.MinOkL > 1 {
			t.Errorf("%s: MinOkL out of range: %v", p, tuning.MinOkL)
		}
		if tuning.MaxOkL <= 0 || tuning.MaxOkL > 1 {
			t.Errorf("%s: MaxOkL out of range: %v", p, tuning.MaxOkL)
		}
		if tuning.MinOkL >= tuning.MaxOkL {
			t.Errorf("%s: MinOkL %.2f >= MaxOkL %.2f", p, tuning.MinOkL, tuning.MaxOkL)
		}
		if tuning.MinChroma < 0 {
			t.Errorf("%s: MinChroma negative: %v", p, tuning.MinChroma)
		}
		if tuning.MaxChroma != 0 && tuning.MaxChroma <= tuning.MinChroma {
			t.Errorf("%s: MaxChroma %.2f <= MinChroma %.2f", p, tuning.MaxChroma, tuning.MinChroma)
		}
		if tuning.SaturationBias < 0 {
			t.Errorf("%s: SaturationBias negative: %v", p, tuning.SaturationBias)
		}
		if tuning.SaturationExponent <= 0 {
			t.Errorf("%s: SaturationExponent must be > 0: %v", p, tuning.SaturationExponent)
		}
		if tuning.OkLPreference < -1 || tuning.OkLPreference > 1 {
			t.Errorf("%s: OkLPreference out of [-1,+1]: %v", p, tuning.OkLPreference)
		}
	}
}
