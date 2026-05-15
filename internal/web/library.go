package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/palette/library"
)

// libraryPaletteJSON is the wire shape for one library entry. Mirrors
// MCP's tools.LibraryPalette — we keep an independent type so neither
// transport package imports the other (CLAUDE.md project-layers rule),
// but both share exporter.ColorJSON for the colors[] array. That shared
// encoder is the load-bearing piece of the cross-parity test: same
// input → same bytes through `exporter.EncodeColors`.
type libraryPaletteJSON struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Categories  []string             `json:"categories"`
	Tags        []string             `json:"tags,omitempty"`
	Colors      []exporter.ColorJSON `json:"colors"`
	Source      string               `json:"source,omitempty"`
	License     string               `json:"license,omitempty"`
}

// libraryCategoryJSON is the runtime view of a category — slug plus
// display name plus member count. The slug is what /api/v1/library/{slug}
// keys off; clients that round-trip a category should send the slug
// even though the display name appears in human-facing UI.
type libraryCategoryJSON struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type libraryIndexResult struct {
	Categories []libraryCategoryJSON `json:"categories"`
	Palettes   []libraryPaletteJSON  `json:"palettes"`
}

type libraryListResult struct {
	Category libraryCategoryJSON  `json:"category"`
	Palettes []libraryPaletteJSON `json:"palettes"`
}

type libraryGetResult struct {
	Palette libraryPaletteJSON `json:"palette"`
}

// registerLibrary mounts the three library REST endpoints on mux under
// base. Called from buildHandler when cfg.Library is non-nil; nil is
// allowed but produces a 503 from each route so misconfiguration is
// visible to the operator without crashing the rest of the server.
func registerLibrary(mux *http.ServeMux, base string, idx *library.Index) {
	mux.HandleFunc("GET "+base+"/library", libraryIndexHandler(idx))
	mux.HandleFunc("GET "+base+"/library/{category}", libraryByCategoryHandler(idx))
	mux.HandleFunc("GET "+base+"/library/palette/{id}", libraryGetHandler(idx))
}

// libraryUnavailable returns true and writes a 503 when no index is
// configured. Centralised so the three handlers share the same error
// shape and operators see one consistent message.
func libraryUnavailable(w http.ResponseWriter, idx *library.Index) bool {
	if idx != nil {
		return false
	}
	writeError(w, http.StatusServiceUnavailable, errors.New("library: catalogue is not configured on this server"))
	return true
}

func libraryIndexHandler(idx *library.Index) http.HandlerFunc {
	// The catalogue is immutable for the process lifetime (loaded once
	// at startup — there is no reload path on library.Index), so the
	// full /library body and its content-hash ETag are computed here at
	// registration. The handler just replays them, letting clients
	// short-circuit a reload to a bodiless 304.
	var (
		body     []byte
		etag     string
		buildErr error
	)
	if idx != nil {
		body, buildErr = buildLibraryIndexBody(idx)
		if buildErr == nil {
			sum := sha256.Sum256(body)
			etag = `"` + hex.EncodeToString(sum[:16]) + `"`
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if libraryUnavailable(w, idx) {
			return
		}
		if buildErr != nil {
			writeError(w, http.StatusInternalServerError, buildErr)
			return
		}
		h := w.Header()
		h.Set("ETag", etag)
		// no-cache = the browser must revalidate every time; our ETag
		// then turns the revalidation into a cheap 304.
		h.Set("Cache-Control", "no-cache")
		if etagMatches(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		h.Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(body)
	}
}

// buildLibraryIndexBody encodes the full /library envelope once — the
// categories list plus every palette in wire shape.
func buildLibraryIndexBody(idx *library.Index) ([]byte, error) {
	cats := idx.Categories()
	catsOut := make([]libraryCategoryJSON, len(cats))
	for i, c := range cats {
		catsOut[i] = libraryCategoryJSON{Slug: c.Slug, Name: c.Name, Count: c.Count}
	}
	all := idx.All()
	palOut := make([]libraryPaletteJSON, 0, len(all))
	for _, lp := range all {
		encoded, err := encodeLibraryPalette(lp)
		if err != nil {
			return nil, err
		}
		palOut = append(palOut, encoded)
	}
	return marshalEnvelope("library.index", nil, libraryIndexResult{
		Categories: catsOut,
		Palettes:   palOut,
	})
}

// etagMatches reports whether the comma-separated If-None-Match header
// covers etag. "*" matches anything; otherwise each candidate is
// compared verbatim — the client only ever echoes back what we sent.
func etagMatches(inm, etag string) bool {
	inm = strings.TrimSpace(inm)
	if inm == "" || etag == "" {
		return false
	}
	if inm == "*" {
		return true
	}
	for part := range strings.SplitSeq(inm, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}

func libraryByCategoryHandler(idx *library.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if libraryUnavailable(w, idx) {
			return
		}
		raw := strings.TrimSpace(r.PathValue("category"))
		if raw == "" {
			writeError(w, http.StatusBadRequest, errors.New("category is required"))
			return
		}
		// Accept either the slug (canonical, used by URL routing) or
		// the display name (lets a human curl the readable form). We
		// normalise both through SlugifyCategory so "/library/Brand%20%26%20Tech"
		// and "/library/brand-tech" hit the same bucket.
		slug := library.SlugifyCategory(raw)
		matched, ok := idx.ByCategory(slug)
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("unknown category %q", raw))
			return
		}
		name, _ := idx.CategoryName(slug)
		palOut := make([]libraryPaletteJSON, 0, len(matched))
		for _, lp := range matched {
			encoded, err := encodeLibraryPalette(lp)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			palOut = append(palOut, encoded)
		}
		writeEnvelope(w, "library.byCategory", map[string]any{"category": raw}, libraryListResult{
			Category: libraryCategoryJSON{Slug: slug, Name: name, Count: len(matched)},
			Palettes: palOut,
		})
	}
}

func libraryGetHandler(idx *library.Index) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if libraryUnavailable(w, idx) {
			return
		}
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, errors.New("id is required"))
			return
		}
		lp, ok := idx.Get(id)
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("library: palette %q not found", id))
			return
		}
		encoded, err := encodeLibraryPalette(lp)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeEnvelope(w, "library.get", map[string]any{"id": id}, libraryGetResult{Palette: encoded})
	}
}

// encodeLibraryPalette converts a stored library.Palette into the wire
// shape. Hex strings are re-parsed via ToPalette so the colors[] array
// is encoded by exporter.EncodeColors — the same encoder MCP's
// library.* tools use, which is what makes the cross-parity test pass
// by construction.
func encodeLibraryPalette(lp library.Palette) (libraryPaletteJSON, error) {
	pal, err := lp.ToPalette()
	if err != nil {
		return libraryPaletteJSON{}, fmt.Errorf("library: palette %q: %w", lp.ID, err)
	}
	return libraryPaletteJSON{
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
