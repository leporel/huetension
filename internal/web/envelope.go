package web

import (
	"encoding/json"
	"net/http"

	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/palette"
)

// schemaVersion is the canonical wire-contract identifier emitted at the
// top of every JSON response. Mirrors exporter.huetensionSchema (private)
// and tools.schemaVersion (in MCP) — the three are kept in sync so any
// consumer can verify the envelope without knowing which transport
// produced it.
const schemaVersion = "huetension/v1"

// genericEnvelope is the on-the-wire shape for tool results whose `result`
// block is not a palette (color.convert formats map, contrast scores,
// blindness variants, export.* render output). Palette-shaped results
// don't use this — they go through writePaletteJSON, which delegates to
// exporter.Export so the bytes are identical to what the CLI emits.
type genericEnvelope struct {
	Schema string `json:"schema"`
	Tool   string `json:"tool,omitempty"`
	Params any    `json:"params,omitempty"`
	Result any    `json:"result"`
}

// writeEnvelope writes a generic huetension/v1 envelope for non-palette
// results. tool is the canonical operation identifier ("color.convert",
// "contrast.check", ...); params echoes back the inputs so consumers can
// replay the call from the envelope alone.
func writeEnvelope(w http.ResponseWriter, tool string, params any, result any) {
	env := genericEnvelope{
		Schema: schemaVersion,
		Tool:   tool,
		Params: params,
		Result: result,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(env); err != nil {
		// Encoding to ResponseWriter rarely fails (only the underlying
		// connection can break here, and the client has already
		// disconnected by definition); log via the access middleware
		// status capture instead of attempting to write a 500 that
		// would also fail.
		return
	}
}

// writePaletteJSON renders pal through exporter.Export(FormatJSON) and
// writes the bytes verbatim. This is the cheapest possible parity check
// with the CLI's --format json mode — by construction, the same palette
// + same Options produce byte-identical responses.
func writePaletteJSON(w http.ResponseWriter, tool string, params map[string]any, pal *palette.Palette) {
	data, err := exporter.Export(pal, exporter.FormatJSON, exporter.Options{
		Tool:   tool,
		Params: params,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

// writeError emits a tiny JSON error body and the given status. The
// shape is `{"error": "..."}` — same as the SDK shape MCP uses, so
// clients can share decoders.
func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// decodeJSON drains the request body into v. Body is closed by the
// stdlib after the handler returns; we do not close it here. Rejects
// unknown fields so typos in client payloads surface as 400s instead of
// silent no-ops.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
