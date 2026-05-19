package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateID(t *testing.T) {
	none := func(string) bool { return false }
	cases := []struct{ name, want string }{
		{"Aurora", "aurora"},
		{"Slate Studio", "slate-studio"},
		{"  Brand & Tech  ", "brand-tech"},
		{"Café Crème", "caf-cr-me"}, // non-ASCII letters collapse to dashes
		{"", "palette"},
		{"!!!", "palette"},
		{"123", "123"},
	}
	for _, c := range cases {
		got := GenerateID(c.name, none)
		if got != c.want {
			t.Errorf("GenerateID(%q) = %q, want %q", c.name, got, c.want)
		}
		if !isSlugID(got) {
			t.Errorf("GenerateID(%q) = %q is not a valid slug id", c.name, got)
		}
	}
}

func TestGenerateIDCollision(t *testing.T) {
	taken := map[string]bool{"aurora": true, "aurora-2": true}
	got := GenerateID("Aurora", func(id string) bool { return taken[id] })
	if got != "aurora-3" {
		t.Errorf("collision suffix: got %q, want aurora-3", got)
	}
}

func TestIndexAdd(t *testing.T) {
	base := MustLoadDefaults()
	n0 := base.Len()
	next, err := base.Add(Palette{
		ID:         "my-test",
		Name:       "My Test",
		Categories: []string{"Saved"},
		Colors:     []string{"#112233", "#445566"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if base.Len() != n0 {
		t.Errorf("Add mutated the receiver: len %d, want %d", base.Len(), n0)
	}
	if _, ok := base.Get("my-test"); ok {
		t.Error("receiver index resolves the added id — Add must not mutate it")
	}
	if next.Len() != n0+1 {
		t.Errorf("new index len %d, want %d", next.Len(), n0+1)
	}
	if got, ok := next.Get("my-test"); !ok || got.Name != "My Test" {
		t.Errorf("Get(my-test) on new index: %+v ok=%v", got, ok)
	}
}

func TestIndexAddRejectsBadInput(t *testing.T) {
	base := MustLoadDefaults()
	existing := base.All()[0].ID

	if _, err := base.Add(Palette{
		ID: existing, Name: "Dup", Categories: []string{"X"}, Colors: []string{"#000000"},
	}); err == nil {
		t.Error("expected a duplicate-id error")
	}
	if _, err := base.Add(Palette{
		ID: "bad-color", Name: "Bad", Categories: []string{"X"}, Colors: []string{"not-a-color"},
	}); err == nil {
		t.Error("expected a bad-color error")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	added, err := MustLoadDefaults().Add(Palette{
		ID:         "round-trip",
		Name:       "Round Trip",
		Categories: []string{"Saved"},
		Tags:       []string{"test"},
		Colors:     []string{"#102030", "#405060"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	path := filepath.Join(t.TempDir(), "library.json")
	if err := Save(path, added); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("atomic write left a %s.tmp file behind", path)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Len() != added.Len() {
		t.Errorf("reloaded len %d, want %d", reloaded.Len(), added.Len())
	}
	// The embedded defaults survive the save (first-save bootstrap).
	if _, ok := reloaded.Get("arctic-frost"); !ok {
		t.Error("a default palette did not survive the save")
	}
	got, ok := reloaded.Get("round-trip")
	if !ok {
		t.Fatal("saved palette missing after reload")
	}
	if got.Name != "Round Trip" || len(got.Colors) != 2 {
		t.Errorf("saved palette content after reload: %+v", got)
	}
}
