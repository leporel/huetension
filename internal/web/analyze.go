package web

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/leporel/huetension/internal/analyze"
	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/sandbox"
)

// analyzeJSONRequest is the JSON-body shape for /api/v1/analyze. Mirrors
// the JSON path of /extract — `url` or `data` (mutually exclusive), plus
// optional `distance_target` and `space` selectors. The strip dimensions
// are server-controlled defaults so the Web UI can rely on a known image
// size without negotiating it per request.
type analyzeJSONRequest struct {
	URL  string `json:"url,omitempty"`
	Data string `json:"data,omitempty"` // base64 bytes OR data:...;base64,... URI

	DistanceTarget string `json:"distance_target,omitempty"`
	Space          string `json:"space,omitempty"`
}

// analyzeInputs is the normalised view both code paths (JSON + multipart)
// produce. Exactly one source field is set per request.
type analyzeInputs struct {
	url            string
	dataURI        string
	rawBytes       []byte
	distanceTarget analyze.DistanceTarget
	space          analyze.Space
}

// analyzeStripJSON is the wire-side strip: metric label + base64 PNG. The
// Web SPA decodes content into a Blob/dataURL for an <img>.
type analyzeStripJSON struct {
	Metric   string `json:"metric"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type analyzeResult struct {
	Strips []analyzeStripJSON `json:"strips"`
}

func (h apiHandlers) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}

	var (
		inputs analyzeInputs
		err    error
	)
	switch {
	case ct == "multipart/form-data":
		inputs, err = h.parseMultipartAnalyze(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		defer cleanupMultipart(r)
	case ct == "application/json", ct == "":
		inputs, err = parseJSONAnalyze(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	default:
		writeError(w, http.StatusUnsupportedMediaType,
			fmt.Errorf("unsupported Content-Type %q (want multipart/form-data or application/json)", ct))
		return
	}

	loaded, err := h.loadAnalyzeInput(inputs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := analyze.Strips(loaded.Image, analyze.Options{
		DistanceTarget: inputs.distanceTarget,
		Space:          inputs.space,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	out := analyzeResult{Strips: make([]analyzeStripJSON, 0, len(result.Strips))}
	for _, s := range result.Strips {
		out.Strips = append(out.Strips, analyzeStripJSON{
			Metric:   string(s.Metric),
			Content:  base64.StdEncoding.EncodeToString(s.PNG),
			Encoding: "base64",
		})
	}

	params := map[string]any{
		"url":             inputs.url,
		"data":            truncateForEcho(inputs.dataURI, 96),
		"distance_target": string(inputs.distanceTarget),
		"space":           string(inputs.space),
	}
	if len(inputs.rawBytes) > 0 {
		params["upload_bytes"] = len(inputs.rawBytes)
	}
	writeEnvelope(w, "image.analyze", params, out)
}

// parseMultipartAnalyze reads the image file plus the optional
// distance_target field. Mirrors parseMultipartExtract but skips all the
// extraction-knob fields it doesn't need.
func (h apiHandlers) parseMultipartAnalyze(r *http.Request) (analyzeInputs, error) {
	max := h.sandbox.MaxImageBytes
	if max == 0 {
		max = imageio.DefaultMaxBytes
	}
	r.Body = http.MaxBytesReader(nil, r.Body, max+64*1024)

	if err := r.ParseMultipartForm(extractMaxMemory); err != nil {
		return analyzeInputs{}, fmt.Errorf("parse multipart: %w", err)
	}

	file, header, err := r.FormFile(extractFileFieldName)
	if err != nil {
		return analyzeInputs{}, fmt.Errorf("missing %q file field: %w", extractFileFieldName, err)
	}
	defer file.Close()

	if header.Size > max {
		return analyzeInputs{}, fmt.Errorf("upload %d bytes exceeds limit %d", header.Size, max)
	}

	body, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return analyzeInputs{}, fmt.Errorf("read upload: %w", err)
	}
	if int64(len(body)) > max {
		return analyzeInputs{}, fmt.Errorf("upload exceeds limit %d", max)
	}

	target, err := resolveAnalyzeTarget(firstValue(r.Form, "distance_target"))
	if err != nil {
		return analyzeInputs{}, err
	}
	space, err := resolveAnalyzeSpace(firstValue(r.Form, "space"))
	if err != nil {
		return analyzeInputs{}, err
	}
	return analyzeInputs{rawBytes: body, distanceTarget: target, space: space}, nil
}

func parseJSONAnalyze(r *http.Request) (analyzeInputs, error) {
	var req analyzeJSONRequest
	if err := decodeJSON(r, &req); err != nil {
		return analyzeInputs{}, err
	}
	url := strings.TrimSpace(req.URL)
	data := strings.TrimSpace(req.Data)
	if url == "" && data == "" {
		return analyzeInputs{}, errors.New("provide either 'url' or 'data'")
	}
	if url != "" && data != "" {
		return analyzeInputs{}, errors.New("'url' and 'data' are mutually exclusive")
	}
	target, err := resolveAnalyzeTarget(req.DistanceTarget)
	if err != nil {
		return analyzeInputs{}, err
	}
	space, err := resolveAnalyzeSpace(req.Space)
	if err != nil {
		return analyzeInputs{}, err
	}
	return analyzeInputs{url: url, dataURI: data, distanceTarget: target, space: space}, nil
}

// loadAnalyzeInput funnels every input shape through one sandbox-aware
// loader, mirroring loadInput on the extract path. Path inputs are NOT
// supported via the web API — same rationale as /extract.
func (h apiHandlers) loadAnalyzeInput(in analyzeInputs) (*imageio.Loaded, error) {
	ioOpts := sandbox.LoadOptionsFor(h.sandbox)

	switch {
	case len(in.rawBytes) > 0:
		return imageio.LoadBytes(in.rawBytes)

	case in.url != "":
		return imageio.Load(in.url, ioOpts)

	case in.dataURI != "":
		raw := in.dataURI
		if strings.HasPrefix(raw, "data:") {
			return imageio.Load(raw, ioOpts)
		}
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

// resolveAnalyzeTarget maps the wire-side distance-target string to the
// canonical DistanceTarget. Empty input falls through to the package
// default so the Web UI can omit the field for the common case.
func resolveAnalyzeTarget(raw string) (analyze.DistanceTarget, error) {
	t, ok := analyze.ParseDistanceTarget(strings.ToLower(strings.TrimSpace(raw)))
	if !ok {
		return "", fmt.Errorf("distance_target: unknown primary %q (want red|green|blue)", raw)
	}
	return t, nil
}

// resolveAnalyzeSpace maps the wire-side space string to the canonical
// Space. Empty input falls through to the package default (OkLCH).
func resolveAnalyzeSpace(raw string) (analyze.Space, error) {
	s, ok := analyze.ParseSpace(strings.ToLower(strings.TrimSpace(raw)))
	if !ok {
		return "", fmt.Errorf("space: unknown colour space %q (want oklch|hsl)", raw)
	}
	return s, nil
}
