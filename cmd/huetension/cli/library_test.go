package cli

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/leporel/huetension/internal/palette/library"
)

func TestLibraryAddSavesPalette(t *testing.T) {
	dir := t.TempDir()
	old := dataDir
	dataDir = dir
	t.Cleanup(func() { dataDir = old })

	cmd := newLibraryAddCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"#ff8800", "#cc4400", "--name", "CLI Sunset", "--category", "Warm"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("library add: %v", err)
	}

	idx, err := library.Load(filepath.Join(dir, "library.json"))
	if err != nil {
		t.Fatalf("load saved library: %v", err)
	}
	p, ok := idx.Get("cli-sunset")
	if !ok {
		t.Fatal("saved palette 'cli-sunset' missing from library.json")
	}
	if len(p.Colors) != 2 {
		t.Errorf("colors: got %d, want 2", len(p.Colors))
	}
	// "Saved" is force-added; the --category extra follows it.
	if len(p.Categories) != 2 || p.Categories[0] != library.SavedCategory || p.Categories[1] != "Warm" {
		t.Errorf("categories: got %v, want [%s Warm]", p.Categories, library.SavedCategory)
	}
}

func TestLibraryAddRequiresName(t *testing.T) {
	cmd := newLibraryAddCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"#ffffff"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected an error when --name is omitted")
	}
}
