package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/extract"
	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/palette"
	"github.com/leporel/huetension/internal/sandbox"
)

// ImageExtractParams is the typed input for image.extract. Exactly one of
// Path, URL, Data must be set.
type ImageExtractParams struct {
	// Source (mutually exclusive).
	Path string `json:"path,omitempty" jsonschema:"local file path; rejected when read-only or escapes root"`
	URL  string `json:"url,omitempty" jsonschema:"http(s):// URL; subject to allow-host filter"`
	Data string `json:"data,omitempty" jsonschema:"raw base64-encoded image bytes (PNG/JPEG/WebP/...) or a data:...;base64,... URI; capped by max-image-bytes"`

	// Extraction options (subset of the CLI extract flags). All optional;
	// internal/extract applies its own defaults when zero.
	Method             string  `json:"method,omitempty" jsonschema:"extraction method: soft|kmeans|okkmeans|mediancut|softk|octree|popularity|wu|dbscan|wkmeans (default: soft)"`
	Size               int     `json:"size,omitempty" jsonschema:"palette size (default 5, max 32)"`
	Resize             int     `json:"resize,omitempty" jsonschema:"resize longest image side to this many pixels (default 512; 0 disables)"`
	SortBy             string `json:"sort_by,omitempty" jsonschema:"final palette sort: luminance|lightness|okl|hue|saturation|frequency|none"`
	Reverse            bool   `json:"reverse,omitempty" jsonschema:"reverse the sort order"`
	AlphaMaskThreshold int    `json:"alpha_mask_threshold,omitempty" jsonschema:"drop pixels with alpha < threshold (0..255)"`
	SoftPreset         string `json:"soft_preset,omitempty" jsonschema:"soft/softk only: Kuler-like mood preset (default|colorful|bright|muted|deep|dark); drives perceptual filter + ranking. Omit = 'default'."`
}

// ImageExtractOutput wraps a single-image extraction in the huetension/v1
// envelope. The result block carries the palette plus extraction metadata.
type ImageExtractOutput struct {
	Schema string             `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string             `json:"tool" jsonschema:"the tool that produced this result"`
	Params ImageExtractParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result PaletteResult      `json:"result"`
}

// ImageExtractBatchParams is the typed input for image.extractBatch. Each
// entry in Sources is auto-detected the same way imageio.Load does it
// (path / http(s):// URL / data: URI). Raw base64 (no data: prefix) is NOT
// accepted in batch — wrap it as a `data:application/octet-stream;base64,…`
// URI to keep the per-entry shape uniform.
type ImageExtractBatchParams struct {
	Sources []string `json:"sources" jsonschema:"image sources: file paths, http(s):// URLs, or data:...;base64,... URIs"`

	// Same extraction knobs as image.extract, applied to each source.
	Method             string `json:"method,omitempty" jsonschema:"extraction method"`
	Size               int    `json:"size,omitempty" jsonschema:"palette size"`
	Resize             int    `json:"resize,omitempty" jsonschema:"resize longest side (px)"`
	SortBy             string `json:"sort_by,omitempty" jsonschema:"final palette sort"`
	Reverse            bool   `json:"reverse,omitempty" jsonschema:"reverse sort order"`
	AlphaMaskThreshold int    `json:"alpha_mask_threshold,omitempty" jsonschema:"drop low-alpha pixels"`
	SoftPreset         string `json:"soft_preset,omitempty" jsonschema:"soft/softk only: Kuler-like mood preset (default|colorful|bright|muted|deep|dark). Omit = 'default'."`

	// MaxWorkers caps parallel extractions; 0 → 4.
	MaxWorkers int `json:"max_workers,omitempty" jsonschema:"max parallel extractions (default 4)"`
}

// ImageExtractBatchEntry is one row of a batch response. Either Result or
// Error is populated.
type ImageExtractBatchEntry struct {
	Source string         `json:"source"`
	Result *PaletteResult `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// ImageExtractBatchResult is the result block for image.extractBatch.
type ImageExtractBatchResult struct {
	Entries []ImageExtractBatchEntry `json:"entries"`
}

// ImageExtractBatchOutput wraps the batch response in the huetension/v1
// envelope. Partial failures (some sources OK, others failed) are NOT a
// tool error — they show up as Error fields on individual entries so the
// LLM can react. A total failure (sandbox rejection of every source, no
// usable input) returns a Go error and IsError on the MCP layer.
type ImageExtractBatchOutput struct {
	Schema string                  `json:"schema" jsonschema:"wire-contract version (huetension/v1)"`
	Tool   string                  `json:"tool" jsonschema:"the tool that produced this result"`
	Params ImageExtractBatchParams `json:"params" jsonschema:"the parameters the tool was invoked with"`
	Result ImageExtractBatchResult `json:"result"`
}

// buildExtractOptions translates an ImageExtractParams (or the matching
// fields in ImageExtractBatchParams) into an extract.Options.
// Returns an error when soft_preset is set to an unknown value so the
// tool surfaces it as an MCP error rather than silently dropping the
// preset.
func buildExtractOptions(p ImageExtractParams) (extract.Options, error) {
	method := extract.Method(strings.ToLower(strings.TrimSpace(p.Method)))
	preset, err := extract.ParseSoftPreset(p.SoftPreset)
	if err != nil {
		return extract.Options{}, err
	}
	if preset != "" && method != "" && method != extract.MethodSoft && method != extract.MethodSoftK {
		return extract.Options{}, fmt.Errorf("soft_preset requires method 'soft' or 'softk', got %q", string(method))
	}
	opts := extract.Options{
		Method:             method,
		PaletteSize:        p.Size,
		Resize:             p.Resize,
		AlphaMaskThreshold: clampAlphaThreshold(p.AlphaMaskThreshold),
		SoftPreset:         preset,
		Reverse:            p.Reverse,
	}
	if sb := strings.ToLower(strings.TrimSpace(p.SortBy)); sb != "" && sb != "none" {
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

// loadImageInput resolves an ImageExtractParams into an *imageio.Loaded,
// applying sandbox checks against the chosen source. Exactly one of
// Path/URL/Data must be set.
func loadImageInput(p ImageExtractParams, sb ImageSandbox) (*imageio.Loaded, error) {
	setCount := 0
	if p.Path != "" {
		setCount++
	}
	if p.URL != "" {
		setCount++
	}
	if p.Data != "" {
		setCount++
	}
	switch setCount {
	case 0:
		return nil, errors.New("provide one of path, url, or data")
	case 1:
		// ok
	default:
		return nil, errors.New("path, url, and data are mutually exclusive")
	}

	ioOpts := sandbox.LoadOptionsFor(sb)

	switch {
	case p.Path != "":
		abs, err := sb.CheckPath(p.Path)
		if err != nil {
			return nil, err
		}
		return imageio.Load(abs, ioOpts)

	case p.URL != "":
		// imageio.loadURL applies AllowedHosts and MaxBytes for us.
		return imageio.Load(p.URL, ioOpts)

	default: // p.Data != ""
		raw := strings.TrimSpace(p.Data)
		if strings.HasPrefix(raw, "data:") {
			// Full data URI — imageio.loadDataURI applies MaxBytes.
			return imageio.Load(raw, ioOpts)
		}
		// Raw base64 — decode, enforce size, then delegate.
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
		max := sb.MaxImageBytes
		if max == 0 {
			max = imageio.DefaultMaxBytes
		}
		if int64(len(decoded)) > max {
			return nil, fmt.Errorf("data exceeds %d bytes", max)
		}
		return imageio.LoadBytes(decoded)
	}
}

// loadBatchSource resolves a single batch source string with sandbox checks.
// Recognises the same prefixes imageio.Load does (data: / http(s):// / path).
// Raw base64 (no data: prefix) is rejected here — see ImageExtractBatchParams.
func loadBatchSource(source string, sb ImageSandbox) (*imageio.Loaded, error) {
	src := strings.TrimSpace(source)
	if src == "" {
		return nil, errors.New("empty source")
	}
	ioOpts := sandbox.LoadOptionsFor(sb)
	switch {
	case strings.HasPrefix(src, "data:"):
		return imageio.Load(src, ioOpts)
	case strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://"):
		return imageio.Load(src, ioOpts)
	default:
		abs, err := sb.CheckPath(src)
		if err != nil {
			return nil, err
		}
		return imageio.Load(abs, ioOpts)
	}
}

func handleImageExtract(ctx context.Context, p ImageExtractParams, sb ImageSandbox) (*sdk.CallToolResult, ImageExtractOutput, error) {
	opts, err := buildExtractOptions(p)
	if err != nil {
		return nil, ImageExtractOutput{}, err
	}
	loaded, err := loadImageInput(p, sb)
	if err != nil {
		return nil, ImageExtractOutput{}, err
	}
	pal, err := extract.FromLoaded(ctx, loaded, opts)
	if err != nil {
		return nil, ImageExtractOutput{}, err
	}
	return nil, ImageExtractOutput{
		Schema: schemaVersion,
		Tool:   "image.extract",
		Params: p,
		Result: encodePalette(pal),
	}, nil
}

func handleImageExtractBatch(ctx context.Context, p ImageExtractBatchParams, sb ImageSandbox) (*sdk.CallToolResult, ImageExtractBatchOutput, error) {
	if len(p.Sources) == 0 {
		return nil, ImageExtractBatchOutput{}, errors.New("sources is required")
	}

	// Validate extraction options up front so a bad preset / method
	// combination fails fast without touching the filesystem.
	opts, err := buildExtractOptions(ImageExtractParams{
		Method:             p.Method,
		Size:               p.Size,
		Resize:             p.Resize,
		SortBy:             p.SortBy,
		Reverse:            p.Reverse,
		AlphaMaskThreshold: p.AlphaMaskThreshold,
		SoftPreset:         p.SoftPreset,
	})
	if err != nil {
		return nil, ImageExtractBatchOutput{}, err
	}

	// Pre-resolve sources with sandbox checks. We do this serially because
	// it's cheap and lets us short-circuit total failure cleanly. The actual
	// per-source extract work runs in parallel below.
	type prepared struct {
		source string
		loaded *imageio.Loaded
		err    error
	}
	prep := make([]prepared, len(p.Sources))
	successCount := 0
	for i, src := range p.Sources {
		l, err := loadBatchSource(src, sb)
		prep[i] = prepared{source: src, loaded: l, err: err}
		if err == nil {
			successCount++
		}
	}
	if successCount == 0 {
		// Total failure: every source rejected. Return a tool error so the
		// LLM sees IsError, with the per-entry detail in the structured
		// content.
		entries := make([]ImageExtractBatchEntry, len(prep))
		for i, x := range prep {
			entries[i] = ImageExtractBatchEntry{Source: x.source, Error: x.err.Error()}
		}
		out := ImageExtractBatchOutput{
			Schema: schemaVersion,
			Tool:   "image.extractBatch",
			Params: p,
			Result: ImageExtractBatchResult{Entries: entries},
		}
		return nil, out, fmt.Errorf("image.extractBatch: all %d sources rejected or failed to load", len(p.Sources))
	}

	// Run extract on the loaded images. Reuse extract.Batch's worker pool
	// indirectly: feed it pre-validated paths/URLs so the sandbox stays on
	// the loadBatchSource side. Easier to do it inline here.
	maxWorkers := p.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	if maxWorkers > len(p.Sources) {
		maxWorkers = len(p.Sources)
	}

	results := make([]ImageExtractBatchEntry, len(prep))
	sem := make(chan struct{}, maxWorkers)
	doneCh := make(chan int, len(prep))

	for i, x := range prep {
		if x.err != nil {
			results[i] = ImageExtractBatchEntry{Source: x.source, Error: x.err.Error()}
			doneCh <- i
			continue
		}
		sem <- struct{}{}
		go func(i int, loaded *imageio.Loaded, src string) {
			defer func() {
				<-sem
				doneCh <- i
			}()
			pal, err := extract.FromLoaded(ctx, loaded, opts)
			if err != nil {
				results[i] = ImageExtractBatchEntry{Source: src, Error: err.Error()}
				return
			}
			pr := encodePalette(pal)
			results[i] = ImageExtractBatchEntry{Source: src, Result: &pr}
		}(i, x.loaded, x.source)
	}
	for range prep {
		<-doneCh
	}

	return nil, ImageExtractBatchOutput{
		Schema: schemaVersion,
		Tool:   "image.extractBatch",
		Params: p,
		Result: ImageExtractBatchResult{Entries: results},
	}, nil
}

// RegisterImageExtract installs image.extract on srv. The handler closes
// over deps.ImageSandbox so the sandbox config is captured once at server
// build time, not re-fetched per call.
func RegisterImageExtract(srv *sdk.Server, deps Deps) {
	sb := deps.ImageSandbox
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "image.extract",
		Description: "Extract a color palette from an image. Source is one of: local path (subject to read-only/root), http(s):// URL (subject to host allowlist), or base64 image bytes. For soft/softk methods, use 'soft_preset' to pick a Kuler-like mood (default|colorful|bright|muted|deep|dark).",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, p ImageExtractParams) (*sdk.CallToolResult, ImageExtractOutput, error) {
		return handleImageExtract(ctx, p, sb)
	})
}

// RegisterImageExtractBatch installs image.extractBatch on srv.
func RegisterImageExtractBatch(srv *sdk.Server, deps Deps) {
	sb := deps.ImageSandbox
	sdk.AddTool(srv, &sdk.Tool{
		Name:        "image.extractBatch",
		Description: "Extract palettes from multiple images in parallel. Per-source failures appear as 'error' fields on each entry; a total failure (every source rejected) is reported as a tool error. For soft/softk methods, use 'soft_preset' (default|colorful|bright|muted|deep|dark) — applied uniformly to all sources.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, p ImageExtractBatchParams) (*sdk.CallToolResult, ImageExtractBatchOutput, error) {
		return handleImageExtractBatch(ctx, p, sb)
	})
}
