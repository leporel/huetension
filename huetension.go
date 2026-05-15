// Package huetension is the public library facade of the huetension color
// palette toolkit. It re-exports the most useful types and functions from
// the internal subpackages so external Go programs can use huetension as a
// library:
//
//	import "github.com/leporel/huetension"
//
//	pal, err := huetension.Extract(ctx, "photo.jpg", huetension.ExtractOptions{
//	    Method:      huetension.MethodSoft,
//	    PaletteSize: 6,
//	})
//	if err != nil { ... }
//	css, _ := huetension.Export(pal, huetension.FormatCSS, huetension.ExportOptions{})
//
// The same internal packages back the CLI, MCP server, and Web UI binaries,
// so the library and the binaries are guaranteed to stay in sync.
//
// The internal/* subpackages are private and may change between minor
// versions; use the symbols re-exported here for stable API guarantees.
package huetension

import (
	"context"

	"github.com/leporel/huetension/internal/blindness"
	"github.com/leporel/huetension/internal/color"
	"github.com/leporel/huetension/internal/contrast"
	"github.com/leporel/huetension/internal/exporter"
	"github.com/leporel/huetension/internal/extract"
	"github.com/leporel/huetension/internal/gradient"
	"github.com/leporel/huetension/internal/harmony"
	"github.com/leporel/huetension/internal/imageio"
	"github.com/leporel/huetension/internal/palette"
)

// Version is the current library version. Wire-format JSON envelopes use
// the separate "huetension/v1" string from the exporter package; bumping
// Version here is allowed without bumping the wire contract.
const Version = "0.1.0"

// Core types.
//
// Color is huetension's canonical sRGB color, Palette is the ordered list
// of colors with provenance metadata.
type (
	Color    = color.Color
	Palette  = palette.Palette
	Metadata = palette.Metadata
)

// SortBy enumerates palette sort keys (luminance, lightness, hue, ...).
type SortBy = palette.SortBy

// Color parsers / constructors. Aliasing functions as vars keeps the
// public surface aware of upstream signature changes.
var (
	NewColor          = color.New
	NewColorWithAlpha = color.NewWithAlpha
	ParseHex          = color.ParseHex
	Parse             = color.Parse
	NewPalette        = palette.New
	RandomPalette     = palette.Random
)

// Extraction surface — turn an image into a palette. ExtractMethod values
// are typed string constants suitable for CLI flag parsing.
type (
	ExtractMethod  = extract.Method
	ExtractOptions = extract.Options
	// SoftPreset is a Kuler-like mood preset for the Soft/SoftK methods.
	SoftPreset = extract.SoftPreset
)

// Soft preset values. Empty string means "no preset" — the legacy
// HSL-knob behavior stays in effect.
const (
	SoftPresetDefault  = extract.SoftPresetDefault
	SoftPresetColorful = extract.SoftPresetColorful
	SoftPresetBright   = extract.SoftPresetBright
	SoftPresetMuted    = extract.SoftPresetMuted
	SoftPresetDeep     = extract.SoftPresetDeep
	SoftPresetDark     = extract.SoftPresetDark
)

// AllSoftPresets is the canonical list of supported soft presets.
var (
	AllSoftPresets  = extract.AllSoftPresets
	ParseSoftPreset = extract.ParseSoftPreset
)

const (
	MethodKMeans         = extract.MethodKMeans
	MethodOkKMeans       = extract.MethodOkKMeans
	MethodMedianCut      = extract.MethodMedianCut
	MethodSoft           = extract.MethodSoft
	MethodSoftK          = extract.MethodSoftK
	MethodOctree         = extract.MethodOctree
	MethodPopularity     = extract.MethodPopularity
	MethodWu             = extract.MethodWu
	MethodDBSCAN         = extract.MethodDBSCAN
	MethodWeightedKMeans = extract.MethodWeightedKMeans
)

// AllExtractMethods is the canonical list of supported extraction methods.
var AllExtractMethods = extract.AllMethods

// Extract pulls a palette from any source string — file path, URL, data:
// URI, or "-" for stdin. Network sources require a non-zero
// ImageHostAllowList; pass it through extract.FromSource directly when
// custom imageio options are needed.
func Extract(ctx context.Context, source string, opts ExtractOptions) (*Palette, error) {
	return extract.FromSource(ctx, source, opts, imageio.LoadOptions{})
}

// ExtractFromImage is the in-memory variant for callers that already
// decoded the image (e.g. via image.Decode).
var ExtractFromImage = extract.Extract

// Harmony surface — generate complementary / analogous / etc. palettes
// around a base color.
type (
	HarmonyType    = harmony.Type
	HarmonyOptions = harmony.Options
)

const (
	HarmonyComplementary       = harmony.Complementary
	HarmonyAnalogous           = harmony.Analogous
	HarmonyTriadic             = harmony.Triadic
	HarmonySplit               = harmony.Split
	HarmonyTetradic            = harmony.Tetradic
	HarmonySquare              = harmony.Square
	HarmonyDoubleComplementary = harmony.DoubleComplementary
	HarmonyMonochromatic       = harmony.Monochromatic
	HarmonyShades              = harmony.Shades
)

// GenerateHarmony returns the colors of a harmony around base.
var GenerateHarmony = harmony.Generate

// Gradient surface — interpolate between two or more colors in a chosen
// space with optional easing.
type (
	GradientOptions = gradient.Options
	GradientSpace   = gradient.Space
)

const (
	GradientRGB   = gradient.SpaceRGB
	GradientLab   = gradient.SpaceLab
	GradientOkLab = gradient.SpaceOkLab
	GradientOkLCH = gradient.SpaceOkLCH
	GradientHSL   = gradient.SpaceHSL
)

var (
	BuildGradient     = gradient.Build
	MultiStopGradient = gradient.MultiStop
)

// Contrast surface — WCAG 2.1 ratio + APCA Lc score.
type (
	ContrastAlgo = contrast.Algo
	WCAG21Result = contrast.WCAG21Result
	APCAResult   = contrast.APCAResult
)

const (
	ContrastWCAG21 = contrast.AlgoWCAG21
	ContrastAPCA   = contrast.AlgoAPCA
)

var (
	WCAG21        = contrast.WCAG21
	APCA          = contrast.APCA
	CheckContrast = contrast.Check
)

// Blindness surface — Brettel-Viénot-Mollon CVD simulation.
type BlindnessKind = blindness.Kind

const (
	BlindnessProtan  = blindness.Protan
	BlindnessDeutan  = blindness.Deutan
	BlindnessTritan  = blindness.Tritan
	BlindnessAchroma = blindness.Achroma
)

var (
	SimulateBlindness        = blindness.Simulate
	SimulateBlindnessPalette = blindness.SimulatePalette
	SimulateAllBlindness     = blindness.SimulateAll
)

// Exporter surface — render a palette to JSON / CSS / SCSS / Tailwind /
// plain hex / GIMP .gpl.
type (
	ExportFormat  = exporter.Format
	ExportOptions = exporter.Options
)

const (
	FormatJSON     = exporter.FormatJSON
	FormatCSS      = exporter.FormatCSS
	FormatSCSS     = exporter.FormatSCSS
	FormatLESS     = exporter.FormatLESS
	FormatTailwind = exporter.FormatTailwind
	FormatPlain    = exporter.FormatPlain
	FormatGPL      = exporter.FormatGPL
	FormatPNG      = exporter.FormatPNG
	FormatJPEG     = exporter.FormatJPEG
)

var (
	AllExportFormats = exporter.AllFormats
	Export           = exporter.Export
	FileExtension    = exporter.FileExtension
)
