package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/leporel/huetension/internal/lut"
	"github.com/leporel/huetension/internal/palette"
)

// lutRequest is the wire shape for POST /api/v1/lut. `format` selects the
// output kind (`cube` = text .cube file, `png` = 2D LUT texture image).
// `size` is the cube grid edge per channel; for `png` it must be a
// perfect square because the texture layout maps the Size³ cube into a
// Size·√Size square.
//
// `size` accepts either an integer or a named preset string
// (json.RawMessage so we can branch on the wire type): "standard" → 64
// (the cube-64 level-8 default).
type lutRequest struct {
	Colors            []string        `json:"colors"`
	Format            string          `json:"format"`
	Radius            float64         `json:"radius"`
	Distribution      float64         `json:"distribution"`
	Intensity         float64         `json:"intensity"`
	BlendNeighbors    int             `json:"blend_neighbors"`
	IncludeSaturation bool            `json:"include_saturation"`
	Size              json.RawMessage `json:"size,omitempty"`
}

// lutResult mirrors exportResult shape — raw text for `cube`, base64 for
// `png` (encoding = "base64" then). Filename suggests a download name.
type lutResult struct {
	Format   string `json:"format"`
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"`
	Filename string `json:"filename"`
}

// handleLUT generates a 3D colour-grading LUT from a palette. The shape
// of the response is identical to /export's so the SPA can reuse the same
// download / base64 decode helpers.
func handleLUT(w http.ResponseWriter, r *http.Request) {
	var req lutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Colors) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no colors provided"))
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format != "cube" && format != "png" {
		writeError(w, http.StatusBadRequest,
			fmt.Errorf("format: unknown %q (want cube|png)", req.Format))
		return
	}
	if math.IsNaN(req.Radius) || math.IsInf(req.Radius, 0) || req.Radius < 0 {
		writeError(w, http.StatusBadRequest,
			fmt.Errorf("radius: must be a finite value ≥ 0, got %v", req.Radius))
		return
	}
	if req.Distribution < 0 || req.Distribution > 1 {
		writeError(w, http.StatusBadRequest,
			fmt.Errorf("distribution: must be in [0, 1], got %v", req.Distribution))
		return
	}
	if req.Intensity < 0 || req.Intensity > 1 {
		writeError(w, http.StatusBadRequest,
			fmt.Errorf("intensity: must be in [0, 1], got %v", req.Intensity))
		return
	}

	colors, err := parseColorList(req.Colors)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	lutSize, err := resolveLUTSize(req.Size, format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if format == "png" {
		sqrt := int(math.Sqrt(float64(lutSize)))
		if sqrt*sqrt != lutSize {
			writeError(w, http.StatusBadRequest,
				fmt.Errorf("size: LUT texture requires a perfect square (4, 9, 16, 25, ...), got %d", lutSize))
			return
		}
	}

	blend := max(req.BlendNeighbors, 1)

	pal := palette.New(colors)
	generated, err := lut.Generate(pal, lut.Options{
		Size:              lutSize,
		Radius:            req.Radius,
		Distribution:      req.Distribution,
		Intensity:         req.Intensity,
		BlendNeighbors:    blend,
		IncludeSaturation: req.IncludeSaturation,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	name := strings.TrimSpace(pal.Name)
	if name == "" {
		name = "palette"
	}

	res := lutResult{Format: format}
	switch format {
	case "cube":
		res.Content = string(lut.EncodeCube(generated, name))
		res.Filename = name + ".cube"
	case "png":
		pngBytes, err := lut.EncodeHaldPNG(generated)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		res.Content = base64.StdEncoding.EncodeToString(pngBytes)
		res.Encoding = "base64"
		side := lutSize * int(math.Sqrt(float64(lutSize)))
		res.Filename = fmt.Sprintf("%s.hald%dx%d.png", name, side, side)
	}

	params := map[string]any{
		"colors":             req.Colors,
		"format":             format,
		"size":               lutSize,
		"radius":             req.Radius,
		"distribution":       req.Distribution,
		"intensity":          req.Intensity,
		"blend_neighbors":    blend,
		"include_saturation": req.IncludeSaturation,
	}
	writeEnvelope(w, "lut.generate", params, res)
}

// resolveLUTSize accepts either a JSON integer or a named preset string.
// Unset → format-specific default (33 cube / 64 png). The preset names
// resolve to cube edge 64 (the level-8 default) for backwards-compatible
// callers.
func resolveLUTSize(raw json.RawMessage, format string) (int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		if format == "png" {
			return 64, nil
		}
		return 33, nil
	}
	// Numeric path first — covers the common UI case.
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	// Fall back to string preset.
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("size: must be an integer or a preset name")
	}
	return parseSizePreset(s)
}

// parseSizePreset maps a named preset to its cube edge. Only the level-8
// (cube edge 64, 512×512 texture) preset ships — kept for the small
// number of API callers that rely on the legacy aliases. Other
// dimensions should pass an explicit integer.
func parseSizePreset(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "obs", "ffmpeg", "standard":
		return 64, nil
	}
	return 0, fmt.Errorf("size: unknown preset %q (want obs|ffmpeg|standard or an integer)", s)
}
