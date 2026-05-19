package library

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// schemaVersion is the wire-contract identifier stamped into a saved
// library File and the only schema parseAndValidate accepts.
const schemaVersion = "huetension/v1"

// File materialises the index as the on-disk JSON shape, palettes in
// authored order. Save round-trips through this, and Load reads it
// straight back — so a saved catalogue replaces the embedded defaults
// wholesale on the next start (defaults + user adds in one file).
func (idx *Index) File() File {
	if idx == nil {
		return File{Schema: schemaVersion}
	}
	pals := make([]Palette, len(idx.palettes))
	copy(pals, idx.palettes)
	return File{Schema: schemaVersion, Palettes: pals}
}

// Add returns a new Index holding every palette of idx plus p. The
// receiver is left untouched: a caller that publishes the result behind
// an atomic pointer keeps the old Index valid for in-flight readers.
// Validation (bad colour, duplicate id, ...) returns the error and no
// Index.
func (idx *Index) Add(p Palette) (*Index, error) {
	f := idx.File()
	f.Palettes = append(f.Palettes, p)
	return validateAndIndex(f)
}

// Save writes idx to path as pretty-printed JSON. The write is atomic —
// the bytes land in a sibling temp file that is renamed over path — so a
// crash mid-write cannot leave a half-written, corrupt catalogue.
func Save(path string, idx *Index) error {
	data, err := json.MarshalIndent(idx.File(), "", "  ")
	if err != nil {
		return fmt.Errorf("library: marshal: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("library: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("library: replace %s: %w", path, err)
	}
	return nil
}

// GenerateID derives a unique slug-shaped palette ID from a display
// name. The slug is lowercase ASCII alphanumerics with dashes (the shape
// isSlugID accepts); a name with no usable characters falls back to
// "palette". When taken reports the candidate as already present a
// numeric suffix is appended ("aurora", "aurora-2", "aurora-3", ...).
func GenerateID(name string, taken func(id string) bool) string {
	base := slugifyID(name)
	if base == "" {
		base = "palette"
	}
	if taken == nil || !taken(base) {
		return base
	}
	for n := 2; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if !taken(candidate) {
			return candidate
		}
	}
}

// slugifyID reduces a string to ASCII slug form: lowercase letters and
// digits survive, every other run collapses to a single dash, edges
// trimmed. Unlike SlugifyCategory it drops non-ASCII letters — palette
// IDs are also URL path values and must satisfy isSlugID's ASCII-only
// rule.
func slugifyID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := true // a leading run produces no dash
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
