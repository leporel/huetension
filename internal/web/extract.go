package web

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/leporel/huetension/internal/extract"
	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/palette"
	"github.com/leporel/huetension/internal/sandbox"
)

// extractFileFieldName is the multipart form field that carries the
// uploaded image. Keep in sync with the SPA's <input name="..."> when
// Phase 3 ships the real UI.
const extractFileFieldName = "image"

// extractMaxMemory caps the in-memory portion of a multipart parse.
// Anything larger spills to a tempfile under os.TempDir(); we delete
// it via cleanupMultipart at the end of the handler. The cap mirrors
// what go's http.ParseMultipartForm uses as its default, made explicit
// here so it can move with our sandbox limits later.
const extractMaxMemory = 32 << 20 // 32 MiB

// extractJSONRequest is the JSON-body shape for /api/v1/extract. The
// browser SPA can either submit multipart/form-data (with the image as
// a file field) or post JSON with a URL / data URI plus the same
// extraction knobs. Field names mirror MCP's image.extract for cross-
// transport consistency.
type extractJSONRequest struct {
	URL  string `json:"url,omitempty"`
	Data string `json:"data,omitempty"` // base64 bytes OR data:...;base64,... URI

	Method             string `json:"method,omitempty"`
	Size               int    `json:"size,omitempty"`
	Resize             int    `json:"resize,omitempty"`
	SortBy             string `json:"sort_by,omitempty"`
	Reverse            bool   `json:"reverse,omitempty"`
	AlphaMaskThreshold int    `json:"alpha_mask_threshold,omitempty"`
	SoftPreset         string `json:"soft_preset,omitempty"`
}

// extractInputs is the normalised view both code paths (JSON + multipart)
// produce before we touch the extract pipeline. Exactly one source field
// is set per request.
type extractInputs struct {
	url      string
	dataURI  string
	rawBytes []byte // multipart upload body
	opts     extract.Options
}

func (h apiHandlers) handleExtract(w http.ResponseWriter, r *http.Request) {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	// Strip any charset/boundary suffix so the prefix match is reliable.
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}

	var (
		inputs extractInputs
		err    error
	)
	switch ct {
	case "multipart/form-data":
		inputs, err = h.parseMultipartExtract(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		defer cleanupMultipart(r)
	case "application/json", "":
		// JSON body (and bare POSTs from curl without a Content-Type
		// header — accept them for convenience, the decoder will reject
		// malformed payloads).
		inputs, err = parseJSONExtract(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	default:
		writeError(w, http.StatusUnsupportedMediaType,
			fmt.Errorf("unsupported Content-Type %q (want multipart/form-data or application/json)", ct))
		return
	}

	loaded, err := h.loadInput(inputs)
	if err != nil {
		// Sandbox rejections (host not allowed, oversized, ...) and decode
		// failures both surface as 400 — the caller can fix either by
		// changing their request, the server can't.
		writeError(w, http.StatusBadRequest, err)
		return
	}

	pal, err := extract.FromLoaded(r.Context(), loaded, inputs.opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	params := map[string]any{
		"url":                  inputs.url,
		"data":                 truncateForEcho(inputs.dataURI, 96),
		"method":               string(inputs.opts.Method),
		"size":                 inputs.opts.PaletteSize,
		"resize":               inputs.opts.Resize,
		"sort_by":              string(inputs.opts.SortBy),
		"reverse":              inputs.opts.Reverse,
		"alpha_mask_threshold": int(inputs.opts.AlphaMaskThreshold),
		"soft_preset":          string(inputs.opts.SoftPreset),
	}
	if len(inputs.rawBytes) > 0 {
		params["upload_bytes"] = len(inputs.rawBytes)
	}
	writePaletteJSON(w, "image.extract", params, pal)
}

// parseMultipartExtract reads the file field plus extraction knobs from
// form fields (one per option, same names as JSON). Body size is bounded
// by sandbox.MaxImageBytes (when set) plus a small fixed overhead for
// form metadata.
func (h apiHandlers) parseMultipartExtract(r *http.Request) (extractInputs, error) {
	max := h.sandbox.MaxImageBytes
	if max == 0 {
		max = imageio.DefaultMaxBytes
	}
	// +64 KiB headroom for boundary markers, field names, and the small
	// extraction-option fields. Anything over this is almost certainly an
	// attempt to defeat the cap.
	r.Body = http.MaxBytesReader(nil, r.Body, max+64*1024)

	if err := r.ParseMultipartForm(extractMaxMemory); err != nil {
		return extractInputs{}, fmt.Errorf("parse multipart: %w", err)
	}

	file, header, err := r.FormFile(extractFileFieldName)
	if err != nil {
		return extractInputs{}, fmt.Errorf("missing %q file field: %w", extractFileFieldName, err)
	}
	defer func() { _ = file.Close() }()

	if header.Size > max {
		return extractInputs{}, fmt.Errorf("upload %d bytes exceeds limit %d", header.Size, max)
	}

	body, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return extractInputs{}, fmt.Errorf("read upload: %w", err)
	}
	if int64(len(body)) > max {
		return extractInputs{}, fmt.Errorf("upload exceeds limit %d", max)
	}

	opts, err := optionsFromForm(r.Form)
	if err != nil {
		return extractInputs{}, err
	}
	return extractInputs{rawBytes: body, opts: opts}, nil
}

func parseJSONExtract(r *http.Request) (extractInputs, error) {
	var req extractJSONRequest
	if err := decodeJSON(r, &req); err != nil {
		return extractInputs{}, err
	}
	url := strings.TrimSpace(req.URL)
	data := strings.TrimSpace(req.Data)
	if url == "" && data == "" {
		return extractInputs{}, errors.New("provide either 'url' or 'data'")
	}
	if url != "" && data != "" {
		return extractInputs{}, errors.New("'url' and 'data' are mutually exclusive")
	}
	opts, err := buildExtractOptions(req.Method, req.Size, req.Resize,
		req.SortBy, req.Reverse, req.AlphaMaskThreshold, req.SoftPreset)
	if err != nil {
		return extractInputs{}, err
	}
	return extractInputs{url: url, dataURI: data, opts: opts}, nil
}

// loadInput funnels every input shape through one sandbox-aware loader
// so the security checks live in exactly one place. Path inputs are NOT
// supported via the web API — the SPA never has a server-side path to
// hand us, and accepting one would re-introduce the read-arbitrary-file
// attack the sandbox is supposed to block.
func (h apiHandlers) loadInput(in extractInputs) (*imageio.Loaded, error) {
	ioOpts := sandbox.LoadOptionsFor(h.sandbox)

	switch {
	case len(in.rawBytes) > 0:
		// Multipart upload — already buffered and length-checked.
		return imageio.LoadBytes(in.rawBytes)

	case in.url != "":
		return imageio.Load(in.url, ioOpts)

	case in.dataURI != "":
		raw := in.dataURI
		if strings.HasPrefix(raw, "data:") {
			return imageio.Load(raw, ioOpts)
		}
		// Plain base64 — accept for client convenience, then enforce
		// the cap on decoded length so consumers can't bypass it by
		// stripping the data: prefix.
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
		max := h.sandbox.MaxImageBytes
		if max == 0 {
			max = imageio.DefaultMaxBytes
		}
		if int64(len(decoded)) > max {
			return nil, fmt.Errorf("data exceeds %d bytes", max)
		}
		return imageio.LoadBytes(decoded)
	}
	return nil, errors.New("no input source resolved (internal bug)")
}

// buildExtractOptions assembles extract.Options from individual values.
// Centralised so JSON and multipart paths share the option-parsing
// semantics — including soft-preset alias resolution and sort-by
// validation. Mirrors mcp/tools.buildExtractOptions but does not import
// it (cross-transport coupling).
func buildExtractOptions(method string, size, resize int, sortBy string, reverse bool,
	alphaThreshold int, softPreset string,
) (extract.Options, error) {
	m := extract.Method(strings.ToLower(strings.TrimSpace(method)))
	preset, err := extract.ParseSoftPreset(softPreset)
	if err != nil {
		return extract.Options{}, err
	}
	if preset != "" && m != "" && m != extract.MethodSoft && m != extract.MethodSoftK {
		return extract.Options{}, fmt.Errorf("soft_preset requires method 'soft' or 'softk', got %q", string(m))
	}
	opts := extract.Options{
		Method:             m,
		PaletteSize:        size,
		Resize:             resize,
		AlphaMaskThreshold: clampAlphaThreshold(alphaThreshold),
		SoftPreset:         preset,
		Reverse:            reverse,
	}
	if sb := strings.ToLower(strings.TrimSpace(sortBy)); sb != "" && sb != "none" {
		opts.SortBy = palette.SortBy(sb)
	}
	return opts, nil
}

func clampAlphaThreshold(v int) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	}
	return uint8(v)
}

// optionsFromForm pulls the extraction knobs out of multipart form
// fields. Integers come in as strings — empty or unparsable values fall
// back to zero, which extract.applyDefaults turns into the same defaults
// the JSON path would get.
func optionsFromForm(form map[string][]string) (extract.Options, error) {
	size, _ := strconv.Atoi(firstValue(form, "size"))
	resize, _ := strconv.Atoi(firstValue(form, "resize"))
	alpha, _ := strconv.Atoi(firstValue(form, "alpha_mask_threshold"))
	reverse, _ := strconv.ParseBool(firstValue(form, "reverse"))
	return buildExtractOptions(
		firstValue(form, "method"),
		size,
		resize,
		firstValue(form, "sort_by"),
		reverse,
		alpha,
		firstValue(form, "soft_preset"),
	)
}

func firstValue(form map[string][]string, key string) string {
	if v := form[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// cleanupMultipart removes any temp files multipart parsing spilled to
// disk. Idempotent; safe to defer regardless of whether parsing
// completed.
func cleanupMultipart(r *http.Request) {
	if r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

// truncateForEcho clamps a long string before echoing it back in the
// params block of the envelope — we don't want to round-trip a 5 MiB
// data URI through the response.
func truncateForEcho(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
