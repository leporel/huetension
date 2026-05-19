package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leporel/huetension/internal/palette/library"
)

func TestHandleLibrarySave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	store := library.NewStore(library.MustLoadDefaults(), path)

	_, out, err := handleLibrarySave(context.Background(), store, false, LibrarySaveParams{
		Name:       "MCP Palette",
		Colors:     []string{"#123456", "#abcdef"},
		Categories: []string{"Cool"},
	})
	if err != nil {
		t.Fatalf("handleLibrarySave: %v", err)
	}
	if out.Schema != schemaVersion || out.Tool != "library.save" {
		t.Errorf("envelope: %+v", out)
	}
	p := out.Result.Palette
	if p.ID != "mcp-palette" {
		t.Errorf("generated id: got %q, want mcp-palette", p.ID)
	}
	if len(p.Categories) != 2 || p.Categories[0] != library.SavedCategory || p.Categories[1] != "Cool" {
		t.Errorf("categories: got %v, want [%s Cool]", p.Categories, library.SavedCategory)
	}

	// The save is visible to a subsequent library.get on the same store.
	_, getOut, err := handleLibraryGet(context.Background(), store.Index(), LibraryGetParams{ID: "mcp-palette"})
	if err != nil {
		t.Fatalf("library.get after save: %v", err)
	}
	if getOut.Result.Palette.ID != "mcp-palette" {
		t.Error("saved palette not visible via library.get on the same store")
	}
}

func TestHandleLibrarySaveReadOnly(t *testing.T) {
	store := library.NewStore(library.MustLoadDefaults(), filepath.Join(t.TempDir(), "library.json"))
	if _, _, err := handleLibrarySave(context.Background(), store, true, LibrarySaveParams{
		Name: "Nope", Colors: []string{"#000000"},
	}); err == nil {
		t.Error("expected an error when the server is read-only")
	}
}

func TestLibraryCategoriesEnvelope(t *testing.T) {
	idx := library.MustLoadDefaults()
	_, out, err := handleLibraryCategories(context.Background(), idx, LibraryCategoriesParams{})
	if err != nil {
		t.Fatalf("handleLibraryCategories: %v", err)
	}
	if out.Schema != schemaVersion || out.Tool != "library.categories" {
		t.Errorf("envelope: %+v", out)
	}
	if len(out.Result.Categories) < 6 {
		t.Errorf("category count: got %d, want >=6", len(out.Result.Categories))
	}
	for _, c := range out.Result.Categories {
		if c.Slug == "" || c.Name == "" {
			t.Errorf("missing slug/name: %+v", c)
		}
		if c.Count == 0 {
			t.Errorf("zero count for %q", c.Name)
		}
	}
}

func TestLibraryListNoFilter(t *testing.T) {
	idx := library.MustLoadDefaults()
	_, out, err := handleLibraryList(context.Background(), idx, LibraryListParams{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if out.Result.Total < 30 {
		t.Errorf("total = %d, want >=30", out.Result.Total)
	}
	if out.Result.Total != len(out.Result.Palettes) {
		t.Errorf("total != len(palettes): %d vs %d", out.Result.Total, len(out.Result.Palettes))
	}
}

func TestLibraryListByCategorySlugAndName(t *testing.T) {
	idx := library.MustLoadDefaults()
	_, bySlug, err := handleLibraryList(context.Background(), idx, LibraryListParams{Category: "brand-tech"})
	if err != nil {
		t.Fatalf("by slug: %v", err)
	}
	if bySlug.Result.Total == 0 {
		t.Fatal("brand-tech: empty")
	}
	_, byName, err := handleLibraryList(context.Background(), idx, LibraryListParams{Category: "Brand & Tech"})
	if err != nil {
		t.Fatalf("by display name: %v", err)
	}
	if byName.Result.Total != bySlug.Result.Total {
		t.Errorf("display-name vs slug differ: %d vs %d", byName.Result.Total, bySlug.Result.Total)
	}
}

func TestLibraryListUnknownCategoryFails(t *testing.T) {
	idx := library.MustLoadDefaults()
	_, _, err := handleLibraryList(context.Background(), idx, LibraryListParams{Category: "no-such"})
	if err == nil || !strings.Contains(err.Error(), "unknown category") {
		t.Errorf("expected unknown-category error, got %v", err)
	}
}

func TestLibraryListByTagSubstring(t *testing.T) {
	idx := library.MustLoadDefaults()
	_, out, err := handleLibraryList(context.Background(), idx, LibraryListParams{Tag: "warm"})
	if err != nil {
		t.Fatalf("tag list: %v", err)
	}
	if out.Result.Total == 0 {
		t.Fatal("expected at least one palette tagged 'warm'")
	}
	for _, p := range out.Result.Palettes {
		matched := false
		for _, tag := range p.Tags {
			if strings.Contains(strings.ToLower(tag), "warm") {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("palette %q in tag-warm list has no matching tag: %v", p.ID, p.Tags)
		}
	}
}

func TestLibraryGetByID(t *testing.T) {
	idx := library.MustLoadDefaults()
	_, out, err := handleLibraryGet(context.Background(), idx, LibraryGetParams{ID: "arctic-frost"})
	if err != nil {
		t.Fatalf("get arctic-frost: %v", err)
	}
	if out.Result.Palette.ID != "arctic-frost" || out.Result.Palette.Name != "Arctic Frost" {
		t.Errorf("identity: %+v", out.Result.Palette)
	}
	if len(out.Result.Palette.Colors) != 4 {
		t.Errorf("color count = %d, want 4", len(out.Result.Palette.Colors))
	}
	for i, c := range out.Result.Palette.Colors {
		if c.Hex == "" {
			t.Errorf("color[%d] hex empty", i)
		}
	}
}

func TestLibraryGetMissing(t *testing.T) {
	idx := library.MustLoadDefaults()
	_, _, err := handleLibraryGet(context.Background(), idx, LibraryGetParams{ID: "no-such"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestLibraryToolsRefuseWithoutIndex(t *testing.T) {
	if _, _, err := handleLibraryCategories(context.Background(), nil, LibraryCategoriesParams{}); err == nil {
		t.Error("categories: expected error with nil index")
	}
	if _, _, err := handleLibraryList(context.Background(), nil, LibraryListParams{}); err == nil {
		t.Error("list: expected error with nil index")
	}
	if _, _, err := handleLibraryGet(context.Background(), nil, LibraryGetParams{ID: "arctic-frost"}); err == nil {
		t.Error("get: expected error with nil index")
	}
}
