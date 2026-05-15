// Package library is huetension's curated palette catalogue. It is the
// pure-Go data layer behind both the REST `/api/v1/library*` endpoints
// (Phase 3) and the MCP `library.*` tools, plus any future programmatic
// caller — the package is transport-agnostic by design.
//
// The catalogue ships embedded as `data/defaults.json` so the binary is
// self-contained. At runtime, callers may pass an external file path to
// Load; when that file exists, it replaces the embedded set entirely
// (defaults plus user additions are written into one file by future
// "add palette" code). When it doesn't exist the embedded defaults win.
// Parse errors on the external file are hard-fail — silently falling
// back to embedded defaults would mask user corruption.
package library

import (
	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/palette"
)

// File is the on-disk JSON shape — top-level wrapper around a slice of
// palettes. The wrapper is forward-compatible: future fields (e.g.
// per-file metadata, schema version negotiation) can be added without a
// migration step. The same shape is what a future "save merged
// defaults+user adds" path will write back next to the binary.
type File struct {
	Schema   string    `json:"schema,omitempty"`
	Palettes []Palette `json:"palettes"`
}

// Palette is one curated entry. Schema is frozen by §6 of
// .prompts/01-phase3-ui.md — JSON consumers (web UI, MCP clients,
// future export tooling) key off these field names. Adding fields is
// safe; renaming or removing them is a wire-contract change.
type Palette struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Categories  []string `json:"categories"`
	Tags        []string `json:"tags,omitempty"`
	Colors      []string `json:"colors"`
	Source      string   `json:"source,omitempty"`
	License     string   `json:"license,omitempty"`
}

// Category is the runtime view of a category that exists in the index:
// its display name (as authored), its URL-safe slug (used by REST
// routing and MCP filter inputs), and how many palettes carry it.
type Category struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Index is an in-memory, read-optimised view over a set of palettes.
// Construction sorts categories by display name (case-insensitive) so
// All() / Categories() output is deterministic across runs and across
// processes — same JSON in, same byte order out.
type Index struct {
	palettes      []Palette
	byID          map[string]int
	bySlug        map[string][]int
	categoryNames map[string]string
	categories    []Category
}

// All returns the palettes in their authored order. The returned slice
// is shared with the index — callers must not mutate it. Mutation
// would invalidate byID / bySlug and break later lookups.
func (idx *Index) All() []Palette {
	if idx == nil {
		return nil
	}
	return idx.palettes
}

// Get returns the palette with the given id and a found flag. ID match
// is case-sensitive — IDs are lowercase slugs by validation, so callers
// should lowercase their input if they accept mixed case.
func (idx *Index) Get(id string) (Palette, bool) {
	if idx == nil {
		return Palette{}, false
	}
	i, ok := idx.byID[id]
	if !ok {
		return Palette{}, false
	}
	return idx.palettes[i], true
}

// ByCategory returns the palettes in the named category (matched by
// slug — see SlugifyCategory). Second return is false when the slug
// matches no known category, so callers can distinguish "empty
// category" from "unknown category".
func (idx *Index) ByCategory(slug string) ([]Palette, bool) {
	if idx == nil {
		return nil, false
	}
	indices, ok := idx.bySlug[slug]
	if !ok {
		return nil, false
	}
	out := make([]Palette, len(indices))
	for i, pi := range indices {
		out[i] = idx.palettes[pi]
	}
	return out, true
}

// Categories returns all categories present in the index, sorted by
// display name. Each Category carries the slug used for routing.
func (idx *Index) Categories() []Category {
	if idx == nil {
		return nil
	}
	return idx.categories
}

// CategoryName resolves a slug back to its display name. Useful for
// transports that received a slug from a URL / tool input but want to
// echo the human-readable form back in the response.
func (idx *Index) CategoryName(slug string) (string, bool) {
	if idx == nil {
		return "", false
	}
	name, ok := idx.categoryNames[slug]
	return name, ok
}

// Len returns the palette count.
func (idx *Index) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.palettes)
}

// ToPalette materialises a library entry as the canonical
// *palette.Palette type used by the rest of huetension. Hex strings are
// re-parsed via color.Parse so caller-side transports can hand the
// result straight to exporter.Export / EncodeColors. Returns an error
// only on a colour that fails to parse — by validation rules every
// stored color must already parse, so this is defensive against
// post-load tampering of the in-memory slice.
func (p Palette) ToPalette() (*palette.Palette, error) {
	cs := make([]color.Color, len(p.Colors))
	for i, hex := range p.Colors {
		c, err := color.Parse(hex)
		if err != nil {
			return nil, err
		}
		cs[i] = c
	}
	pal := palette.New(cs)
	pal.Name = p.Name
	pal.Metadata.Source = "library:" + p.ID
	pal.Metadata.Method = "library"
	return pal, nil
}
