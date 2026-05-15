package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmbeddedDefaultsParse asserts the shipped data/defaults.json
// satisfies every validation rule. This is the closest thing to a
// "data file lints" gate — adding a malformed palette crashes the
// build via tests rather than producing a 500 in production.
func TestEmbeddedDefaultsParse(t *testing.T) {
	idx, err := Load("")
	if err != nil {
		t.Fatalf("load embedded defaults: %v", err)
	}
	if got := idx.Len(); got < 30 {
		t.Errorf("embedded defaults must contain ≥30 palettes (acceptance criterion §13); got %d", got)
	}
	cats := idx.Categories()
	if len(cats) < 6 {
		t.Errorf("embedded defaults must cover ≥6 categories (acceptance criterion §13); got %d (%v)", len(cats), cats)
	}
	for _, c := range cats {
		if c.Slug == "" || c.Name == "" {
			t.Errorf("category with missing slug/name: %+v", c)
		}
		if c.Count == 0 {
			t.Errorf("category %q has zero palettes", c.Name)
		}
	}
}

func TestSlugifyCategory(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Brand & Tech", "brand-tech"},
		{"Editorial / Publishing", "editorial-publishing"},
		{"Theme-Factory", "theme-factory"},
		{"Vintage / Retro", "vintage-retro"},
		{"Nature", "nature"},
		{"  Leading and trailing  ", "leading-and-trailing"},
		{"--multiple---separators--", "multiple-separators"},
	}
	for _, tc := range cases {
		if got := SlugifyCategory(tc.in); got != tc.want {
			t.Errorf("SlugifyCategory(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestLoadExternalReplacesEmbedded verifies the user's stated rule:
// when an external library file is present it replaces the embedded
// defaults entirely (it carries defaults+user adds combined).
func TestLoadExternalReplacesEmbedded(t *testing.T) {
	dir := t.TempDir()
	external := filepath.Join(dir, "lib.json")
	body := `{
		"palettes": [
			{
				"id": "user-only",
				"name": "User Only",
				"categories": ["Custom"],
				"colors": ["#112233", "#445566"]
			}
		]
	}`
	if err := os.WriteFile(external, []byte(body), 0o644); err != nil {
		t.Fatalf("write external: %v", err)
	}
	idx, err := Load(external)
	if err != nil {
		t.Fatalf("load external: %v", err)
	}
	if idx.Len() != 1 {
		t.Errorf("expected only the user palette (1), got %d", idx.Len())
	}
	if _, ok := idx.Get("user-only"); !ok {
		t.Error("user-only not found in external index")
	}
	if _, ok := idx.Get("arctic-frost"); ok {
		t.Error("embedded defaults leaked into external load — should be replaced wholesale")
	}
}

// TestLoadExternalMissingFallsBack covers the common case before any
// user customisation: the configured external path simply doesn't
// exist, so we serve embedded defaults rather than erroring.
func TestLoadExternalMissingFallsBack(t *testing.T) {
	idx, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("load missing external: %v", err)
	}
	if idx.Len() == 0 {
		t.Fatal("expected embedded defaults after missing-external fallback")
	}
	if _, ok := idx.Get("arctic-frost"); !ok {
		t.Error("expected embedded arctic-frost palette after fallback")
	}
}

// TestLoadExternalMalformedHardFails checks the deliberate hard-fail:
// silent fallback to embedded defaults would mask user corruption, so
// any parse / validate error on an existing external file aborts.
func TestLoadExternalMalformedHardFails(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"trailing garbage", `{"palettes": []} not json`, "decode"},
		{"empty palette list", `{"palettes": []}`, "no palettes"},
		{"invalid hex", `{"palettes":[{"id":"x","name":"X","categories":["C"],"colors":["not-a-color"]}]}`, "colors[0]"},
		{"duplicate id", `{"palettes":[
			{"id":"x","name":"X","categories":["C"],"colors":["#000000"]},
			{"id":"x","name":"X2","categories":["C"],"colors":["#FFFFFF"]}
		]}`, "duplicate id"},
		{"bad slug id", `{"palettes":[{"id":"NotASlug","name":"X","categories":["C"],"colors":["#000000"]}]}`, "must be lowercase"},
		{"unknown top field", `{"paletteslist": []}`, "unknown field"},
		{"missing category", `{"palettes":[{"id":"x","name":"X","categories":[],"colors":["#000000"]}]}`, "at least one category"},
		{"duplicate color", `{"palettes":[{"id":"x","name":"X","categories":["C"],"colors":["#FFFFFF","#FFFFFF"]}]}`, "duplicates an earlier entry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			external := filepath.Join(t.TempDir(), "lib.json")
			if err := os.WriteFile(external, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, err := Load(external)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v does not contain %q", err, tc.want)
			}
		})
	}
}

func TestByCategoryReturnsKnownAndUnknown(t *testing.T) {
	idx := MustLoadDefaults()
	cats := idx.Categories()
	if len(cats) == 0 {
		t.Fatal("no categories")
	}
	for _, c := range cats {
		got, ok := idx.ByCategory(c.Slug)
		if !ok {
			t.Errorf("category slug %q not found", c.Slug)
		}
		if len(got) != c.Count {
			t.Errorf("category %q reports count %d, ByCategory returned %d", c.Slug, c.Count, len(got))
		}
	}
	if _, ok := idx.ByCategory("definitely-not-a-category"); ok {
		t.Error("ByCategory returned ok=true for an unknown slug")
	}
}

func TestPaletteToPalette(t *testing.T) {
	idx := MustLoadDefaults()
	src, ok := idx.Get("arctic-frost")
	if !ok {
		t.Fatal("arctic-frost missing from defaults")
	}
	pal, err := src.ToPalette()
	if err != nil {
		t.Fatalf("ToPalette: %v", err)
	}
	if pal.Len() != len(src.Colors) {
		t.Errorf("color count: got %d, want %d", pal.Len(), len(src.Colors))
	}
	if pal.Name != src.Name {
		t.Errorf("name: got %q, want %q", pal.Name, src.Name)
	}
	if pal.Metadata.Source != "library:arctic-frost" {
		t.Errorf("metadata.source: got %q, want library:arctic-frost", pal.Metadata.Source)
	}
}
