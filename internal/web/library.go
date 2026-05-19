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

// libraryState is the web layer's view of the catalogue: the shared
// library.Store (concurrency-safe, persists saves) plus this server's
// read-only posture. readOnly mirrors the sandbox flag — a read-only
// server refuses saves even when the store has a writable path.
type libraryState struct {
	store    *library.Store
	readOnly bool
}

// newLibraryState wraps the loaded index, its on-disk path, and the
// server's read-only posture. idx may be nil (no catalogue configured);
// path may be empty (no data directory); readOnly true disables saving.
func newLibraryState(idx *library.Index, path string, readOnly bool) *libraryState {
	return &libraryState{store: library.NewStore(idx, path), readOnly: readOnly}
}

// current returns the catalogue index visible right now.
func (st *libraryState) current() *library.Index {
	if st == nil {
		return nil
	}
	return st.store.Index()
}

// registerLibrary mounts the library REST endpoints on mux under base.
// Called from buildAppMux; a nil index produces a 503 from each route so
// misconfiguration is visible to the operator without crashing the rest
// of the server. The POST route appends a user palette and persists it.
func registerLibrary(mux *http.ServeMux, base string, st *libraryState) {
	mux.HandleFunc("GET "+base+"/library", libraryIndexHandler(st))
	mux.HandleFunc("GET "+base+"/library/{category}", libraryByCategoryHandler(st))
	mux.HandleFunc("GET "+base+"/library/palette/{id}", libraryGetHandler(st))
	mux.HandleFunc("POST "+base+"/library/palette", librarySaveHandler(st))
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

func libraryIndexHandler(st *libraryState) http.HandlerFunc {
	// The catalogue is mutable — a save publishes a new index — so the
	// body and its content-hash ETag are rebuilt per request. The
	// catalogue is small (a few dozen palettes), so this is cheap, and
	// it keeps the ETag honest: a save changes the bytes and therefore
	// the ETag, so a stale client revalidates instead of getting a 304.
	return func(w http.ResponseWriter, r *http.Request) {
		idx := st.current()
		if libraryUnavailable(w, idx) {
			return
		}
		body, err := buildLibraryIndexBody(idx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		sum := sha256.Sum256(body)
		etag := `"` + hex.EncodeToString(sum[:16]) + `"`
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

func libraryByCategoryHandler(st *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idx := st.current()
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

func libraryGetHandler(st *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idx := st.current()
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

// librarySaveRequest is the body of POST /library/palette. The server
// owns the ID and force-adds the "Saved" category — clients send only
// the palette content.
type librarySaveRequest struct {
	Name        string   `json:"name"`
	Colors      []string `json:"colors"`
	Categories  []string `json:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Description string   `json:"description,omitempty"`
}

// librarySaveHandler appends a user palette to the catalogue and
// persists it via the shared library.Store. Saving requires a writable
// server: a read-only server answers 403, a server with no data
// directory answers 503, while the read endpoints keep working either way.
func librarySaveHandler(st *libraryState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if libraryUnavailable(w, st.current()) {
			return
		}
		if st.readOnly {
			writeError(w, http.StatusForbidden,
				errors.New("library: saving is disabled on this read-only server"))
			return
		}
		var req librarySaveRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		saved, err := st.store.Save(library.SaveInput{
			Name:        req.Name,
			Description: req.Description,
			Colors:      req.Colors,
			Categories:  req.Categories,
			Tags:        req.Tags,
		})
		if err != nil {
			writeLibrarySaveError(w, err)
			return
		}
		encoded, err := encodeLibraryPalette(saved)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeEnvelope(w, "library.save", map[string]any{"name": saved.Name},
			libraryGetResult{Palette: encoded})
	}
}

// writeLibrarySaveError maps a library.Store.Save failure onto an HTTP
// status: no on-disk path is 503, a disk fault is 500 (its detail kept
// server-side), and anything else is a 400 on the client's input.
func writeLibrarySaveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, library.ErrNotPersistable):
		writeError(w, http.StatusServiceUnavailable, err)
	case errors.Is(err, library.ErrPersist):
		writeError(w, http.StatusInternalServerError,
			errors.New("library: failed to persist the palette"))
	default:
		writeError(w, http.StatusBadRequest, err)
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
