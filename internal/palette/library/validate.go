package library

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/leporel/huetension/internal/color"
)

// validateAndIndex builds an Index from a parsed File. Validation rules
// are intentionally strict — catalog files are curated, so any
// rejected field is a real authoring bug, not a transient input. Hard
// failure here is preferable to silently dropping entries the UI will
// later try to display.
//
// Rules:
//   - Each palette has a non-empty id, slug-shaped (lowercase alnum +
//     dashes; no leading/trailing dash; no empty segment).
//   - IDs are unique across the whole file.
//   - name is non-empty.
//   - At least one category, each non-empty after trimming. Categories
//     with the same slug count as the same category (Index keys by
//     slug; the first display-name spelling wins).
//   - At least one color, each parseable by color.Parse.
//   - No duplicate colors within a palette (case-insensitive on the hex
//     normalisation; "#FFF" and "#FFFFFF" are equal).
//   - Tags / source / license are free-form; no rules beyond non-empty
//     when present.
func validateAndIndex(f File) (*Index, error) {
	if len(f.Palettes) == 0 {
		return nil, fmt.Errorf("library: no palettes in file")
	}

	idx := &Index{
		palettes:      make([]Palette, 0, len(f.Palettes)),
		byID:          make(map[string]int, len(f.Palettes)),
		bySlug:        make(map[string][]int, len(f.Palettes)),
		categoryNames: make(map[string]string, 8),
	}

	for pi, p := range f.Palettes {
		if err := validatePalette(p, pi); err != nil {
			return nil, err
		}
		if _, dup := idx.byID[p.ID]; dup {
			return nil, fmt.Errorf("library: palettes[%d]: duplicate id %q", pi, p.ID)
		}

		// Normalise & deduplicate categories within the palette: two
		// authored names that share a slug ("Brand & Tech" vs
		// "brand & tech") collapse into one slot, so the same palette
		// is not listed twice in one category.
		seenSlug := map[string]struct{}{}
		canonCats := make([]string, 0, len(p.Categories))
		for _, raw := range p.Categories {
			name := strings.TrimSpace(raw)
			if name == "" {
				return nil, fmt.Errorf("library: palettes[%d] %q: empty category", pi, p.ID)
			}
			slug := SlugifyCategory(name)
			if slug == "" {
				return nil, fmt.Errorf("library: palettes[%d] %q: category %q produces empty slug", pi, p.ID, name)
			}
			if _, ok := seenSlug[slug]; ok {
				continue
			}
			seenSlug[slug] = struct{}{}
			canonCats = append(canonCats, name)

			if existing, ok := idx.categoryNames[slug]; ok {
				_ = existing // first-encountered display name wins
			} else {
				idx.categoryNames[slug] = name
			}
			idx.bySlug[slug] = append(idx.bySlug[slug], pi)
		}
		// Persist the canonicalised category list back so callers see
		// the deduplicated form.
		p.Categories = canonCats

		// Invariant: nothing is ever skipped during this loop, so the
		// File index pi equals the post-append index in idx.palettes.
		// If a future refactor switches to "skip invalid entries", flip
		// these maps to len(idx.palettes)-1 (set after the append) so
		// Get / ByCategory keep working.
		idx.palettes = append(idx.palettes, p)
		idx.byID[p.ID] = pi
	}

	idx.categories = buildCategoryList(idx)
	return idx, nil
}

// validatePalette runs the per-entry rules. pi is the file index of the
// palette, surfaced in error messages so authors can find the offending
// JSON entry without grep.
func validatePalette(p Palette, pi int) error {
	if !isSlugID(p.ID) {
		return fmt.Errorf("library: palettes[%d]: id %q must be lowercase alphanumeric with dashes (e.g. \"aurora\" or \"slate-studio\")", pi, p.ID)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("library: palettes[%d] %q: name is required", pi, p.ID)
	}
	if len(p.Categories) == 0 {
		return fmt.Errorf("library: palettes[%d] %q: at least one category is required", pi, p.ID)
	}
	if len(p.Colors) == 0 {
		return fmt.Errorf("library: palettes[%d] %q: at least one color is required", pi, p.ID)
	}

	seenColor := map[string]struct{}{}
	for ci, raw := range p.Colors {
		hex := strings.TrimSpace(raw)
		if hex == "" {
			return fmt.Errorf("library: palettes[%d] %q: colors[%d] is empty", pi, p.ID, ci)
		}
		c, err := color.Parse(hex)
		if err != nil {
			return fmt.Errorf("library: palettes[%d] %q: colors[%d] %q: %w", pi, p.ID, ci, hex, err)
		}
		key := strings.ToLower(c.Hex())
		if _, dup := seenColor[key]; dup {
			return fmt.Errorf("library: palettes[%d] %q: colors[%d] %q duplicates an earlier entry", pi, p.ID, ci, hex)
		}
		seenColor[key] = struct{}{}
	}
	return nil
}

// isSlugID accepts only lowercase letter/digit/dash, no leading or
// trailing dash, no empty segment. Matches the slug shape we generate
// for categories so palette IDs and category slugs share one alphabet.
func isSlugID(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			prevDash = false
		case r >= '0' && r <= '9':
			prevDash = false
		case r == '-':
			if prevDash {
				return false
			}
			prevDash = true
		default:
			// Reject uppercase, whitespace, punctuation — keeps URL
			// path values free of percent-encoding requirements.
			if unicode.IsUpper(r) {
				return false
			}
			return false
		}
	}
	return true
}

// buildCategoryList materialises the sorted Category slice. Sorted by
// display name (case-insensitive) so output is deterministic and the
// UI's category sidebar renders in stable alphabetical order.
func buildCategoryList(idx *Index) []Category {
	out := make([]Category, 0, len(idx.categoryNames))
	for slug, name := range idx.categoryNames {
		out = append(out, Category{
			Slug:  slug,
			Name:  name,
			Count: len(idx.bySlug[slug]),
		})
	}
	// Manual insertion sort is fine for the small N (≤ a dozen
	// categories); avoids pulling in sort just for this.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.ToLower(out[j-1].Name) > strings.ToLower(out[j].Name); j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
