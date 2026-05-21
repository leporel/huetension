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
	Method            string          `json:"method,omitempty"` // "knn" (legacy default) | "rbf" (smooth)
	IncludeSaturation bool            `json:"include_saturation"`
	Size              json.RawMessage `json:"size,omitempty"`

	// K-NN knobs.
	Radius         float64 `json:"radius"`
	Distribution   float64 `json:"distribution"`
	Intensity      float64 `json:"intensity"`
	BlendNeighbors int     `json:"blend_neighbors"`

	// RBF knobs.
	Reach     float64 `json:"reach,omitempty"`
	Sharpness float64 `json:"sharpness,omitempty"`
	Strength  float64 `json:"strength,omitempty"`
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
	method := normaliseLUTMethod(req.Method)
	if err := validateLUTMethodParams(method, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
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
		Method:            method,
		IncludeSaturation: req.IncludeSaturation,
		Radius:            req.Radius,
		Distribution:      req.Distribution,
		Intensity:         req.Intensity,
		BlendNeighbors:    blend,
		Reach:             req.Reach,
		Sharpness:         req.Sharpness,
		Strength:          req.Strength,
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
		"method":             method,
		"size":               lutSize,
		"include_saturation": req.IncludeSaturation,
	}
	if method == lut.MethodKNN {
		params["radius"] = req.Radius
		params["distribution"] = req.Distribution
		params["intensity"] = req.Intensity
		params["blend_neighbors"] = blend
	} else {
		params["reach"] = req.Reach
		params["sharpness"] = req.Sharpness
		params["strength"] = req.Strength
	}
	writeEnvelope(w, "lut.generate", params, res)
}

// normaliseLUTMethod maps the wire `method` field to the canonical
// algorithm constant. Empty / unrecognised values resolve to "knn" so
// older clients keep producing the legacy output.
func normaliseLUTMethod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", lut.MethodKNN:
		return lut.MethodKNN
	case lut.MethodRBF:
		return lut.MethodRBF
	}
	// Unrecognised → fall back to the safe legacy default rather than
	// 400-ing; the algorithm dispatcher will reject any genuinely
	// invalid value at Generate time.
	return lut.MethodKNN
}

// validateLUTMethodParams enforces the per-method knob ranges. The
// dispatcher in lut.Generate also validates, but doing it here lets us
// return a 400 with the offending field name rather than letting the
// algorithm-side error bubble up under the same status code.
func validateLUTMethodParams(method string, req lutRequest) error {
	switch method {
	case lut.MethodKNN:
		if math.IsNaN(req.Radius) || math.IsInf(req.Radius, 0) || req.Radius < 0 {
			return fmt.Errorf("radius: must be a finite value ≥ 0, got %v", req.Radius)
		}
		if req.Distribution < 0 || req.Distribution > 1 {
			return fmt.Errorf("distribution: must be in [0, 1], got %v", req.Distribution)
		}
		if req.Intensity < 0 || req.Intensity > 1 {
			return fmt.Errorf("intensity: must be in [0, 1], got %v", req.Intensity)
		}
	case lut.MethodRBF:
		if math.IsNaN(req.Reach) || math.IsInf(req.Reach, 0) || req.Reach <= 0 {
			return fmt.Errorf("reach: must be a finite value > 0, got %v", req.Reach)
		}
		if math.IsNaN(req.Sharpness) || math.IsInf(req.Sharpness, 0) || req.Sharpness <= 0 {
			return fmt.Errorf("sharpness: must be a finite value > 0, got %v", req.Sharpness)
		}
		if req.Strength < 0 || req.Strength > 1 {
			return fmt.Errorf("strength: must be in [0, 1], got %v", req.Strength)
		}
	}
	return nil
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
