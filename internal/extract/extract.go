// Package extract turns an image into a Palette via one of three algorithms:
//
//   - kmeans     — fast, faithful k-means clustering in CIE Lab space.
//   - mediancut  — the classical median-cut quantiser, RGB-cube partitioning.
//   - soft       — designer-friendly: Lab over-clustering followed by ΔE merge,
//     saturation/lightness pre-filter, and a population × saturation ranking.
//     This is the default and the mode the user explicitly requested for
//     "human" palettes.
//
// All three return *palette.Palette with full provenance metadata so the
// CLI / MCP / Web layers can surface where a palette came from and how.
package extract

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/palette"
)

// Method names a quantisation algorithm.
type Method string

const (
	// MethodKMeans runs k-means in CIE Lab. Faithful, fast.
	MethodKMeans Method = "kmeans"
	// MethodOkKMeans runs k-means in OkLab — perceptually uniform, often
	// gives better separation on muted / pastel palettes than CIE Lab.
	MethodOkKMeans Method = "okkmeans"
	// MethodMedianCut uses the classical RGB-cube median cut.
	MethodMedianCut Method = "mediancut"
	// MethodSoft is the designer-friendly default — Lab over-clustering with
	// ΔE merge and population × saturation ranking.
	MethodSoft Method = "soft"
	// MethodSoftK is the designer-friendly default — Lab over-clustering with
	// more agressive ΔE merge and population × saturation ranking.
	MethodSoftK Method = "softk"
	// MethodOctree partitions pixels into an 8-ary RGB tree, collapsing the
	// least-populated subtree until ≤ N leaves remain. Deterministic, fast.
	MethodOctree Method = "octree"
	// MethodPopularity quantises each channel to 5 bits and returns the top
	// N most-populous bins. The simplest possible algorithm — useful for
	// indexed-color / pixel-art images.
	MethodPopularity Method = "popularity"
	// MethodWu is Xiaolin Wu's variance-minimising quantiser. Higher quality
	// than median cut for the same speed, deterministic.
	MethodWu Method = "wu"
	// MethodDBSCAN runs density-based clustering in OkLab space; cluster
	// count is data-driven, top-K by population is returned.
	MethodDBSCAN Method = "dbscan"
	// MethodWeightedKMeans is k-means in OkLab on de-duplicated pixels with
	// weights = pixel counts. Faster than plain k-means and biased toward
	// dominant colors.
	MethodWeightedKMeans Method = "wkmeans"
)

// AllMethods enumerates the methods Extract knows. Used by tests and by the
// CLI's `--method` flag completion.
var AllMethods = []Method{
	MethodKMeans,
	MethodOkKMeans,
	MethodMedianCut,
	MethodSoft,
	MethodSoftK,
	MethodOctree,
	MethodPopularity,
	MethodWu,
	MethodDBSCAN,
	MethodWeightedKMeans,
}

// Options configures Extract. Zero values fall back to designer-friendly
// defaults documented per-field.
type Options struct {
	// Method to use; defaults to MethodSoft.
	Method Method

	// PaletteSize is the number of colors to extract; defaults to 5,
	// capped at 64.
	PaletteSize int

	// Resize the longest image side to this many pixels before extracting.
	// 0 = no resize. Defaults to 256 — large enough to capture detail but
	// small enough that even soft mode runs in under a second.
	Resize int

	// AlphaMaskThreshold drops pixels with alpha strictly below this value.
	// 0 (default) keeps every pixel.
	AlphaMaskThreshold uint8

	// MinSaturation pre-filters pixels by HSL saturation. Soft and SoftK mode only.
	// 0 = no filter.
	MinSaturation float64
	// MinLightness pre-filters pixels too dark. Soft and SoftK mode only.
	MinLightness float64
	// MaxLightness pre-filters pixels too bright. Soft and SoftK mode only.
	MaxLightness float64
	// MergeEpsilon — Lab ΔE76 distance below which two clusters are merged.
	// Soft mode only. 0 falls back to defaultSoftMergeEpsilon.
	MergeEpsilon float64

	// SortBy applies a final sort to the produced palette. "" = no sort.
	SortBy palette.SortBy
	// Reverse the sort.
	Reverse bool
}

// Defaults applied to a zero-value Options before extraction.
const (
	defaultPaletteSize  = 5
	defaultResize       = 512
	defaultSoftMinSat   = 0.05
	defaultSoftMinLight = 0.05
	defaultSoftMaxLight = 0.95

	maxPaletteSize = 32

	maxKMeansSamplePixels = 32768 // sub-sample threshold for KMeans speed (16384 optimal)

	kOverClusterFactor = 5
	wkMaxOverK         = 32 // ceiling on over-clustering — Lloyd is O(N·K')
	softMaxOverK       = 64

	dbscanSaliencyPow = 1.5 // saturation-bias exponent for saliency proxy
	wkSaliencyPow     = 1.5

	softSaturationBias     = 0.5
	softSaturationExponent = 1.0
)

// Per-method merge / radius thresholds. Centralised here so palette tuning
// happens in one file instead of being scattered across method
// implementations. Each constant is documented in its own units — they
// LOOK similar (single-digit floats) but live in different spaces:
//
//   - defaultSoftMergeEpsilon — CIE Lab ΔE76 (units of perceptual difference,
//     ~8 = "noticeable but not jarring"). Drives cluster merging in soft
//     mode and is the fallback when Options.MergeEpsilon is zero.
//   - wkMeansMergeEpsilon — OkLab × 100 (numerically comparable to ΔE76
//     since OkLab is roughly perceptually-uniform like CIE Lab).
//   - dbScanEpsilon — OkLab × 100, but used as a DENSITY RADIUS (the ε
//     in DBSCAN), not a merge threshold. Two cells are connected if
//     their OkLab × 100 distance is ≤ this value.
const (
	defaultSoftMergeEpsilon = 12.0
	wkMeansMergeEpsilon     = 8.0
	dbScanEpsilon           = 6.0
)

// Extract runs the configured algorithm on the given image and returns the
// resulting Palette with full provenance metadata. The image is not mutated.
func Extract(ctx context.Context, img image.Image, opts Options) (*palette.Palette, error) {
	if img == nil {
		return nil, errors.New("extract: nil image")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	opts = applyDefaults(opts)
	start := time.Now()

	originalBounds := img.Bounds()
	resized := imageio.Resize(img, opts.Resize)
	processedBounds := resized.Bounds()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pixels, stats := imageio.PixelsWithStats(resized, opts.AlphaMaskThreshold)
	if len(pixels) == 0 {
		return nil, errors.New("extract: no pixels remain after alpha-mask filter")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var colors []color.Color
	var err error
	switch opts.Method {
	case MethodKMeans:
		colors, err = extractKMeans(pixels, opts.PaletteSize)
	case MethodOkKMeans:
		colors, err = extractOkKMeans(pixels, opts.PaletteSize)
	case MethodMedianCut:
		colors = extractMedianCut(pixels, opts.PaletteSize)
	case MethodSoft:
		colors, err = extractSoft(pixels, opts.PaletteSize, opts)
	case MethodSoftK:
		colors, err = extractSoftK(pixels, opts.PaletteSize, opts)
	case MethodOctree:
		colors = extractOctree(pixels, opts.PaletteSize)
	case MethodPopularity:
		colors = extractPopularity(pixels, opts.PaletteSize, defaultPopularityBits)
	case MethodWu:
		colors = extractWu(pixels, opts.PaletteSize)
	case MethodDBSCAN:
		colors = extractDBSCAN(pixels, opts.PaletteSize)
	case MethodWeightedKMeans:
		colors = extractWeightedKMeans(pixels, opts.PaletteSize)
	default:
		err = fmt.Errorf("unknown method %q", string(opts.Method))
	}
	if err != nil {
		return nil, fmt.Errorf("extract: %s: %w", opts.Method, err)
	}
	if len(colors) == 0 {
		return nil, fmt.Errorf("extract: %s produced no colors", opts.Method)
	}

	p := palette.New(colors)
	p.Metadata = palette.Metadata{
		Method: string(opts.Method),
		Params: extractParams(opts),
		ImageInfo: &palette.ImageInfo{
			OriginalSize:  [2]int{originalBounds.Dx(), originalBounds.Dy()},
			ProcessedSize: [2]int{processedBounds.Dx(), processedBounds.Dy()},
			HasAlpha:      hasAlpha(img),
		},
		Stats: &palette.Stats{
			TotalPixels: stats.TotalPixels,
			ValidPixels: stats.ValidPixels,
			DurationMS:  time.Since(start).Milliseconds(),
		},
		GeneratedAt: time.Now(),
	}

	if opts.SortBy != "" {
		if err := p.Sort(opts.SortBy, opts.Reverse); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// FromLoaded extracts a palette from an *imageio.Loaded, copying its
// source/format into the palette metadata. Equivalent to calling Extract
// with Loaded.Image and then patching ImageInfo.Format / Metadata.Source.
func FromLoaded(ctx context.Context, l *imageio.Loaded, opts Options) (*palette.Palette, error) {
	if l == nil {
		return nil, errors.New("extract: nil loaded")
	}
	p, err := Extract(ctx, l.Image, opts)
	if err != nil {
		return nil, err
	}
	p.Metadata.Source = l.Source
	if p.Metadata.ImageInfo != nil {
		p.Metadata.ImageInfo.Format = l.Format
		p.Metadata.ImageInfo.HasAlpha = l.HasAlpha
	}
	return p, nil
}

// FromSource is the convenience wrapper used by both the CLI extract
// command and the MCP image.extract tool.
func FromSource(ctx context.Context, source string, opts Options, ioOpts imageio.LoadOptions) (*palette.Palette, error) {
	loaded, err := imageio.Load(source, ioOpts)
	if err != nil {
		return nil, err
	}
	return FromLoaded(ctx, loaded, opts)
}

// Result is a single entry in a Batch run.
type Result struct {
	Source  string
	Palette *palette.Palette
	Err     error
}

// Batch runs FromSource on each source in parallel, capped at maxWorkers
// goroutines. The output preserves input order. Failures don't abort the
// run — they appear as Result.Err.
func Batch(ctx context.Context, sources []string, opts Options, ioOpts imageio.LoadOptions, maxWorkers int) []Result {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	if maxWorkers > len(sources) {
		maxWorkers = len(sources)
	}

	results := make([]Result, len(sources))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	for i, s := range sources {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, s string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := ctx.Err(); err != nil {
				results[i] = Result{Source: s, Err: err}
				return
			}
			p, err := FromSource(ctx, s, opts, ioOpts)
			results[i] = Result{Source: s, Palette: p, Err: err}
		}(i, s)
	}
	wg.Wait()
	return results
}

func applyDefaults(opts Options) Options {
	if opts.Method == "" {
		opts.Method = MethodSoft
	}
	if opts.PaletteSize <= 0 {
		opts.PaletteSize = defaultPaletteSize
	}
	if opts.PaletteSize > maxPaletteSize {
		opts.PaletteSize = maxPaletteSize
	}
	if opts.Resize == 0 {
		opts.Resize = defaultResize
	}
	if opts.MergeEpsilon == 0 {
		opts.MergeEpsilon = defaultSoftMergeEpsilon
	}
	if opts.MinSaturation == 0 && (opts.Method == MethodSoft || opts.Method == MethodSoftK) {
		opts.MinSaturation = defaultSoftMinSat
	}
	if opts.MinLightness == 0 && (opts.Method == MethodSoft || opts.Method == MethodSoftK) {
		opts.MinLightness = defaultSoftMinLight
	}
	if opts.MaxLightness == 0 && (opts.Method == MethodSoft || opts.Method == MethodSoftK) {
		opts.MaxLightness = defaultSoftMaxLight
	}
	return opts
}

func extractParams(opts Options) map[string]any {
	p := map[string]any{
		"method":       string(opts.Method),
		"palette_size": opts.PaletteSize,
		"resize":       opts.Resize,
	}
	if opts.AlphaMaskThreshold > 0 {
		p["alpha_mask_threshold"] = int(opts.AlphaMaskThreshold)
	}
	if opts.Method == MethodSoft || opts.Method == MethodSoftK {
		p["min_saturation"] = opts.MinSaturation
		p["min_lightness"] = opts.MinLightness
		p["max_lightness"] = opts.MaxLightness
		if opts.Method == MethodSoft {
			p["merge_epsilon"] = opts.MergeEpsilon
		}
	}
	if opts.SortBy != "" {
		p["sort_by"] = string(opts.SortBy)
		p["reverse"] = opts.Reverse
	}
	return p
}

func hasAlpha(img image.Image) bool {
	switch img.(type) {
	case *image.RGBA, *image.RGBA64, *image.NRGBA, *image.NRGBA64, *image.NYCbCrA, *image.Alpha, *image.Alpha16:
		return true
	}
	return false
}

// pickOverK returns K' = min(K × factor, cap, points), but never below K.
// Capping at wkMaxOverK keeps Lloyd's O(N·K') cost in check; capping at
// len(points) prevents wasted slots when the image has few unique colors.
func pickOverK(k, numPoints, maxOverK int) int {
	overK := min(k*kOverClusterFactor, maxOverK, numPoints)
	return max(overK, k)
}
