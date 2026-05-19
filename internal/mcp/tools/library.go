package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/palette/library"
)

// LibraryPalette is the wire shape of a single library entry returned by
// library.* tools. Superset of the standard PaletteResult shape: carries
// the curated metadata fields (description, categories, tags, source,
// license) the catalogue keeps alongside the colors. Library tools
// intentionally use their own envelope (rather than the palette
// envelope used by extract / harmony / gradient) so adding curatorial
// fields here doesn't drift the standard wire contract
type LibraryPalette struct {
	ID          string               `json:"id" jsonschema:"unique slug for this palette"`
	Name        string               `json:"name" jsonschema:"display name"`
	Description string               `json:"description,omitempty" jsonschema:"one-line description"`
	Categories  []string             `json:"categories" jsonschema:"category display names this palette belongs to"`
	Tags        []string             `json:"tags,omitempty" jsonschema:"free-form tags"`
	Colors      []exporter.ColorJSON `json:"colors" jsonschema:"colors in the palette in canonical order"`
	Source      string               `json:"source,omitempty" jsonschema:"provenance string (e.g. theme-factory:aurora)"`
	License     string               `json:"license,omitempty" jsonschema:"license identifier"`
}

// LibraryCategory is the wire shape of one entry in library.categories.
// Slug is what REST routing and library.list filters key off; Name is
// the human-readable form to display.
type LibraryCategory struct {
	Slug  string `json:"slug" jsonschema:"URL-safe slug derived from name"`
	Name  string `json:"name" jsonschema:"display name"`
	Count int    `json:"count" jsonschema:"number of palettes in this category"`
}

// ---------- library.categories ----------

type LibraryCategoriesParams struct{}

type LibraryCategoriesResult struct {
	Categories []LibraryCategory `json:"categories" jsonschema:"all categories present in the catalogue, sorted by display name"`
}

type LibraryCategoriesOutput struct {
	Schema string                  `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string                  `json:"tool" jsonschema:"the tool that produced this result"`
	Params LibraryCategoriesParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result LibraryCategoriesResult `json:"result"`
}

func RegisterLibraryCategories(srv *sdk.Server, deps Deps) {
	store := deps.Library
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "library.categories",
		Description: "List the categories in the curated palette catalogue (with palette counts). Slugs are stable URL-safe identifiers usable as the 'category' filter on library.list.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, p LibraryCategoriesParams) (*sdk.CallToolResult, LibraryCategoriesOutput, error) {
		_, out, err := handleLibraryCategories(ctx, store.Index(), p)
		return nil, out, err
	})
}

func handleLibraryCategories(_ context.Context, idx *library.Index, _ LibraryCategoriesParams) (*sdk.CallToolResult, LibraryCategoriesOutput, error) {
	if idx == nil {
		return nil, LibraryCategoriesOutput{}, errors.New("library: index is not configured on this server")
	}
	cats := idx.Categories()
	out := make([]LibraryCategory, len(cats))
	for i, c := range cats {
		out[i] = LibraryCategory{Slug: c.Slug, Name: c.Name, Count: c.Count}
	}
	return nil, LibraryCategoriesOutput{
		Schema: schemaVersion,
		Tool:   "library.categories",
		Result: LibraryCategoriesResult{Categories: out},
	}, nil
}

// ---------- library.list ----------

type LibraryListParams struct {
	Category string `json:"category,omitempty" jsonschema:"filter by category — accepts the slug ('brand-tech') or the display name ('Brand & Tech'); empty matches all"`
	Tag      string `json:"tag,omitempty" jsonschema:"filter by tag (case-insensitive substring match on each palette's tags); empty matches all"`
}

type LibraryListResult struct {
	Total    int              `json:"total" jsonschema:"number of palettes matching the filter"`
	Palettes []LibraryPalette `json:"palettes" jsonschema:"the matched palettes in catalogue order"`
}

type LibraryListOutput struct {
	Schema string            `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string            `json:"tool" jsonschema:"the tool that produced this result"`
	Params LibraryListParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result LibraryListResult `json:"result"`
}

func RegisterLibraryList(srv *sdk.Server, deps Deps) {
	store := deps.Library
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "library.list",
		Description: "List palettes in the curated catalogue, optionally filtered by category (slug or display name) and/or tag.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, p LibraryListParams) (*sdk.CallToolResult, LibraryListOutput, error) {
		_, out, err := handleLibraryList(ctx, store.Index(), p)
		return nil, out, err
	})
}

func handleLibraryList(_ context.Context, idx *library.Index, p LibraryListParams) (*sdk.CallToolResult, LibraryListOutput, error) {
	if idx == nil {
		return nil, LibraryListOutput{}, errors.New("library: index is not configured on this server")
	}
	var pool []library.Palette
	if cat := strings.TrimSpace(p.Category); cat != "" {
		slug := library.SlugifyCategory(cat)
		matched, ok := idx.ByCategory(slug)
		if !ok {
			return nil, LibraryListOutput{}, fmt.Errorf("unknown category %q", cat)
		}
		pool = matched
	} else {
		pool = idx.All()
	}
	tag := strings.ToLower(strings.TrimSpace(p.Tag))
	out := make([]LibraryPalette, 0, len(pool))
	for _, lp := range pool {
		if tag != "" && !matchesTag(lp.Tags, tag) {
			continue
		}
		encoded, err := encodeLibraryPalette(lp)
		if err != nil {
			return nil, LibraryListOutput{}, err
		}
		out = append(out, encoded)
	}
	return nil, LibraryListOutput{
		Schema: schemaVersion,
		Tool:   "library.list",
		Params: LibraryListParams{Category: p.Category, Tag: p.Tag},
		Result: LibraryListResult{Total: len(out), Palettes: out},
	}, nil
}

// ---------- library.get ----------

type LibraryGetParams struct {
	ID string `json:"id" jsonschema:"unique palette id (slug — see library.list result.palettes[].id)"`
}

type LibraryGetResult struct {
	Palette LibraryPalette `json:"palette" jsonschema:"the requested palette"`
}

type LibraryGetOutput struct {
	Schema string           `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string           `json:"tool" jsonschema:"the tool that produced this result"`
	Params LibraryGetParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result LibraryGetResult `json:"result"`
}

func RegisterLibraryGet(srv *sdk.Server, deps Deps) {
	store := deps.Library
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "library.get",
		Description: "Fetch a single palette from the curated catalogue by id.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, p LibraryGetParams) (*sdk.CallToolResult, LibraryGetOutput, error) {
		_, out, err := handleLibraryGet(ctx, store.Index(), p)
		return nil, out, err
	})
}

func handleLibraryGet(_ context.Context, idx *library.Index, p LibraryGetParams) (*sdk.CallToolResult, LibraryGetOutput, error) {
	if idx == nil {
		return nil, LibraryGetOutput{}, errors.New("library: index is not configured on this server")
	}
	id := strings.TrimSpace(p.ID)
	if id == "" {
		return nil, LibraryGetOutput{}, errors.New("id is required")
	}
	lp, ok := idx.Get(id)
	if !ok {
		return nil, LibraryGetOutput{}, fmt.Errorf("library: palette %q not found", id)
	}
	encoded, err := encodeLibraryPalette(lp)
	if err != nil {
		return nil, LibraryGetOutput{}, err
	}
	return nil, LibraryGetOutput{
		Schema: schemaVersion,
		Tool:   "library.get",
		Params: LibraryGetParams{ID: id},
		Result: LibraryGetResult{Palette: encoded},
	}, nil
}

// ---------- library.save ----------

type LibrarySaveParams struct {
	Name        string   `json:"name" jsonschema:"display name for the palette"`
	Colors      []string `json:"colors" jsonschema:"palette colors (hex, rgb(), CSS name, ...) — at least one"`
	Categories  []string `json:"categories,omitempty" jsonschema:"extra categories to file the palette under; the \"Saved\" category is always added"`
	Tags        []string `json:"tags,omitempty" jsonschema:"free-form tags"`
	Description string   `json:"description,omitempty" jsonschema:"one-line description"`
}

type LibrarySaveResult struct {
	Palette LibraryPalette `json:"palette" jsonschema:"the saved palette, including its server-generated id"`
}

type LibrarySaveOutput struct {
	Schema string            `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string            `json:"tool" jsonschema:"the tool that produced this result"`
	Params LibrarySaveParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result LibrarySaveResult `json:"result"`
}

func RegisterLibrarySave(srv *sdk.Server, deps Deps) {
	store := deps.Library
	readOnly := deps.ImageSandbox.ReadOnly
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "library.save",
		Description: "Save a palette to the on-disk library so it joins the catalogue (filed under the \"Saved\" category). The server generates the id. Disabled on a read-only server or one with no data directory.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, p LibrarySaveParams) (*sdk.CallToolResult, LibrarySaveOutput, error) {
		_, out, err := handleLibrarySave(ctx, store, readOnly, p)
		return nil, out, err
	})
}

func handleLibrarySave(_ context.Context, store *library.Store, readOnly bool, p LibrarySaveParams) (*sdk.CallToolResult, LibrarySaveOutput, error) {
	if store.Index() == nil {
		return nil, LibrarySaveOutput{}, errors.New("library: index is not configured on this server")
	}
	if readOnly {
		return nil, LibrarySaveOutput{}, errors.New("library: saving is disabled on this read-only server")
	}
	saved, err := store.Save(library.SaveInput{
		Name:        p.Name,
		Description: p.Description,
		Colors:      p.Colors,
		Categories:  p.Categories,
		Tags:        p.Tags,
	})
	if err != nil {
		return nil, LibrarySaveOutput{}, err
	}
	encoded, err := encodeLibraryPalette(saved)
	if err != nil {
		return nil, LibrarySaveOutput{}, err
	}
	return nil, LibrarySaveOutput{
		Schema: schemaVersion,
		Tool:   "library.save",
		Params: p,
		Result: LibrarySaveResult{Palette: encoded},
	}, nil
}

// encodeLibraryPalette converts a stored library.Palette into the wire
// LibraryPalette shape. Hex strings are re-parsed via ToPalette so the
// emitted color blocks share the same encoder (exporter.EncodeColors)
// the REST handler uses — that's what guarantees byte-identical
// `colors` arrays across the two transports.
func encodeLibraryPalette(lp library.Palette) (LibraryPalette, error) {
	pal, err := lp.ToPalette()
	if err != nil {
		return LibraryPalette{}, fmt.Errorf("library: palette %q: %w", lp.ID, err)
	}
	return LibraryPalette{
		ID:          lp.ID,
		Name:        lp.Name,
		Description: lp.Description,
		Categories:  append([]string(nil), lp.Categories...),
		Tags:        append([]string(nil), lp.Tags...),
		Colors:      exporter.EncodeColors(pal.Colors),
		Source:      lp.Source,
		License:     lp.License,
	}, nil
}

// matchesTag returns true when any of the palette's tags contains
// needle (case-insensitive). Substring rather than exact match so
// "warm" matches both "warm" and "warm-orange" — friendlier to free-form
// tag authoring.
func matchesTag(tags []string, needle string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), needle) {
			return true
		}
	}
	return false
}
