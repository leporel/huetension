package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/lut"
	"github.com/leporel/huetension/internal/palette"
)

// applyLUTImageField carries the user image(s) to grade. The field can
// be repeated — every `image` part is graded against the same generated
// cube and returned in input order under `results`. `params` is a single
// JSON-encoded form text field describing the LUT itself.
const (
	applyLUTImageField  = "image"
	applyLUTParamsField = "params"
)

// applyLUTTimeout caps how long a single ffmpeg invocation may run.
// Multi-image requests apply this per-image, not for the whole batch.
const applyLUTTimeout = 30 * time.Second

// applyLUTParams mirrors lutRequest's grading knobs. The handler always
// re-generates the cube from these knobs and pipes it to ffmpeg's
// `lut3d` filter — there is no caching across requests, but the
// generate step is cheap (deterministic, palette-sized work) and the
// alternative (uploading a multi-MB .cube text per call) is worse.
type applyLUTParams struct {
	Colors            []string `json:"colors"`
	Method            string   `json:"method,omitempty"` // "knn" (legacy default) | "rbf" (smooth)
	IncludeSaturation bool     `json:"include_saturation"`
	// Size is the cube edge per channel. Any integer ≥ 2 is accepted —
	// for `apply-lut` it does not need to be a perfect square because
	// we feed the cube directly to ffmpeg's `lut3d` (text format), not
	// as a 2D texture.
	Size int `json:"size,omitempty"`

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

// applyLUTResult is the wire shape for a single graded output. Multi-
// image requests return them in `applyLUTBatchResult.Results`, in the
// order the `image` form parts were uploaded.
type applyLUTResult struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type applyLUTBatchResult struct {
	Results []applyLUTResult `json:"results"`
}

// handleApplyLUT applies a LUT (built from `params`) to one or more
// user-supplied images via the local ffmpeg binary's `lut3d` filter.
// The cube is generated once per request and reused across every image
// in the batch — this lets the SPA grade a test image and a reference
// spectrum strip in a single round trip.
//
// Returns 503 with an install hint when ffmpeg is not on PATH so the
// SPA can surface a "please install ffmpeg" UI.
func (h apiHandlers) handleApplyLUT(w http.ResponseWriter, r *http.Request) {
	maxBytes := h.sandbox.MaxImageBytes
	if maxBytes == 0 {
		maxBytes = imageio.DefaultMaxBytes
	}
	// Two image parts + headroom for the (small) params field.
	r.Body = http.MaxBytesReader(nil, r.Body, 4*maxBytes+64*1024)

	if err := r.ParseMultipartForm(extractMaxMemory); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parse multipart: %w", err))
		return
	}
	defer cleanupMultipart(r)

	paramsRaw := strings.TrimSpace(firstValue(r.MultipartForm.Value, applyLUTParamsField))
	if paramsRaw == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing %q form field", applyLUTParamsField))
		return
	}
	var params applyLUTParams
	if err := json.Unmarshal([]byte(paramsRaw), &params); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("params: %w", err))
		return
	}
	if err := validateApplyLUTParams(params); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	headers := r.MultipartForm.File[applyLUTImageField]
	if len(headers) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing %q file field", applyLUTImageField))
		return
	}

	imageParts := make([]imagePart, 0, len(headers))
	for i, hdr := range headers {
		body, ext, err := readMultipartFile(hdr, maxBytes)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("image #%d: %w", i+1, err))
			return
		}
		imageParts = append(imageParts, imagePart{bytes: body, ext: ext})
	}

	colors, err := parseColorList(params.Colors)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ffmpegBin, err := exec.LookPath("ffmpeg")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable,
			errors.New("ffmpeg not found on PATH — install ffmpeg to use LUT preview (https://ffmpeg.org/download.html)"))
		return
	}

	cubeSize := params.Size
	if cubeSize == 0 {
		cubeSize = 33
	}
	method := normaliseLUTMethod(params.Method)
	generated, err := lut.Generate(palette.New(colors), lut.Options{
		Size:              cubeSize,
		Method:            method,
		IncludeSaturation: params.IncludeSaturation,
		Radius:            params.Radius,
		Distribution:      params.Distribution,
		Intensity:         params.Intensity,
		BlendNeighbors:    max(params.BlendNeighbors, 1),
		Reach:             params.Reach,
		Sharpness:         params.Sharpness,
		Strength:          params.Strength,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cubeText := lut.EncodeCube(generated, "huetension")

	results, err := runFFmpegLUT3D(r.Context(), ffmpegBin, cubeText, imageParts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	batch := applyLUTBatchResult{Results: make([]applyLUTResult, len(results))}
	for i, b := range results {
		batch.Results[i] = applyLUTResult{
			Content:  base64.StdEncoding.EncodeToString(b),
			Encoding: "base64",
		}
	}

	envParams := map[string]any{
		"images":             len(imageParts),
		"cube_size":          cubeSize,
		"method":             method,
		"include_saturation": params.IncludeSaturation,
	}
	if method == lut.MethodKNN {
		envParams["radius"] = params.Radius
		envParams["distribution"] = params.Distribution
		envParams["intensity"] = params.Intensity
		envParams["blend_neighbors"] = params.BlendNeighbors
	} else {
		envParams["reach"] = params.Reach
		envParams["sharpness"] = params.Sharpness
		envParams["strength"] = params.Strength
	}
	writeEnvelope(w, "lut.apply", envParams, batch)
}

func validateApplyLUTParams(p applyLUTParams) error {
	if len(p.Colors) == 0 {
		return errors.New("no colors provided")
	}
	switch normaliseLUTMethod(p.Method) {
	case lut.MethodKNN:
		if math.IsNaN(p.Radius) || math.IsInf(p.Radius, 0) || p.Radius < 0 {
			return fmt.Errorf("radius: must be a finite value ≥ 0, got %v", p.Radius)
		}
		if p.Distribution < 0 || p.Distribution > 1 {
			return fmt.Errorf("distribution: must be in [0, 1], got %v", p.Distribution)
		}
		if p.Intensity < 0 || p.Intensity > 1 {
			return fmt.Errorf("intensity: must be in [0, 1], got %v", p.Intensity)
		}
	case lut.MethodRBF:
		if math.IsNaN(p.Reach) || math.IsInf(p.Reach, 0) || p.Reach <= 0 {
			return fmt.Errorf("reach: must be a finite value > 0, got %v", p.Reach)
		}
		if math.IsNaN(p.Sharpness) || math.IsInf(p.Sharpness, 0) || p.Sharpness <= 0 {
			return fmt.Errorf("sharpness: must be a finite value > 0, got %v", p.Sharpness)
		}
		if p.Strength < 0 || p.Strength > 1 {
			return fmt.Errorf("strength: must be in [0, 1], got %v", p.Strength)
		}
	}
	return nil
}

type imagePart struct {
	bytes []byte
	ext   string // including the leading dot, e.g. ".png"
}

// readMultipartFile reads one *multipart.FileHeader to bytes, capping
// by maxBytes and probing the filename for an extension hint that
// ffmpeg's demuxer uses for auto-detection.
func readMultipartFile(h *multipart.FileHeader, maxBytes int64) ([]byte, string, error) {
	if h.Size > maxBytes {
		return nil, "", fmt.Errorf("%d bytes exceeds limit %d", h.Size, maxBytes)
	}
	f, err := h.Open()
	if err != nil {
		return nil, "", fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	body, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("exceeds limit %d", maxBytes)
	}
	ext := strings.ToLower(filepath.Ext(h.Filename))
	if ext == "" {
		ext = ".png"
	}
	return body, ext, nil
}

// runFFmpegLUT3D writes the cube text to a scratch directory and invokes
// `ffmpeg -i in -vf lut3d=lut.cube -frames:v 1 out.png` once per image
// in the batch. The cube file is reused across images so the disk cost
// is paid once; ffmpeg startup is paid per image but that is unavoidable.
//
// All outputs land in the same scratch dir and are read back in order;
// the dir (and everything in it) is removed on return.
func runFFmpegLUT3D(ctx context.Context, bin string, cubeText []byte, images []imagePart) ([][]byte, error) {
	dir, err := os.MkdirTemp("", "huetension-lut-*")
	if err != nil {
		return nil, fmt.Errorf("mkdtemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	cubePath := filepath.Join(dir, "lut.cube")
	if err := os.WriteFile(cubePath, cubeText, 0o600); err != nil {
		return nil, fmt.Errorf("write cube: %w", err)
	}

	results := make([][]byte, len(images))
	for i, img := range images {
		inPath := filepath.Join(dir, fmt.Sprintf("in_%d%s", i, img.ext))
		outPath := filepath.Join(dir, fmt.Sprintf("out_%d.png", i))

		if err := os.WriteFile(inPath, img.bytes, 0o600); err != nil {
			return nil, fmt.Errorf("write input #%d: %w", i+1, err)
		}

		runCtx, cancel := context.WithTimeout(ctx, applyLUTTimeout)
		var stderr bytes.Buffer
		cmd := exec.CommandContext(runCtx, bin,
			"-hide_banner", "-loglevel", "error",
			"-i", inPath,
			"-vf", "lut3d=lut.cube",
			"-frames:v", "1",
			"-y", outPath,
		)
		// Set the working directory so `lut3d=lut.cube` resolves without
		// path escaping — ffmpeg filter args have their own quoting rules
		// and Windows paths with colons + backslashes need careful
		// escaping. A simple relative filename sidesteps all of that.
		cmd.Dir = dir
		cmd.Stderr = &stderr

		runErr := cmd.Run()
		cancel()
		if runErr != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = runErr.Error()
			}
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("image #%d: ffmpeg timed out after %s", i+1, applyLUTTimeout)
			}
			const capLen = 800
			if len(msg) > capLen {
				msg = msg[:capLen] + "…"
			}
			return nil, fmt.Errorf("image #%d: ffmpeg failed: %s", i+1, msg)
		}

		out, err := os.ReadFile(outPath)
		if err != nil {
			return nil, fmt.Errorf("read output #%d: %w", i+1, err)
		}
		results[i] = out
	}
	return results, nil
}
