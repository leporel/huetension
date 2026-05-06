// Package palette holds the ordered-color-list type used everywhere in
// huetension and its operations: sort, random generation, and (in later
// slices) extraction-result wrapping and library lookup.
package palette

import (
	"maps"
	"time"

	"github.com/leporel/huetension/internal/color"
)

// Palette is an ordered list of colors plus optional metadata describing how
// the palette was produced.
type Palette struct {
	Colors   []color.Color
	Name     string
	Metadata Metadata
}

// Metadata is provenance — recorded by extractors and generators, ignored by
// pure-color operations. It is shaped to match the JSON contract the CLI and
// MCP slices will both emit.
type Metadata struct {
	Source      string         `json:"source,omitempty"`
	Method      string         `json:"method,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	ImageInfo   *ImageInfo     `json:"image_info,omitempty"`
	Stats       *Stats         `json:"stats,omitempty"`
	GeneratedAt time.Time      `json:"generated_at,omitzero"`
}

// ImageInfo describes the source image of an extracted palette.
type ImageInfo struct {
	OriginalSize  [2]int `json:"original_size"`
	ProcessedSize [2]int `json:"processed_size"`
	Format        string `json:"format,omitempty"`
	HasAlpha      bool   `json:"has_alpha"`
}

// Stats records timings and pixel counts from an extraction run.
type Stats struct {
	TotalPixels int   `json:"total_pixels"`
	ValidPixels int   `json:"valid_pixels"`
	DurationMS  int64 `json:"duration_ms"`
}

// New builds a Palette from a slice of colors. The input slice is copied so
// the caller can safely reuse it.
func New(colors []color.Color) *Palette {
	cp := make([]color.Color, len(colors))
	copy(cp, colors)
	return &Palette{Colors: cp}
}

// Len returns the number of colors. Safe on nil receiver.
func (p *Palette) Len() int {
	if p == nil {
		return 0
	}
	return len(p.Colors)
}

// At returns the color at index i; panics if out of range, like slice access.
func (p *Palette) At(i int) color.Color {
	return p.Colors[i]
}

// Clone returns a deep-enough copy: colors slice is independent, ImageInfo
// and Stats pointers are shared (they are immutable in practice).
func (p *Palette) Clone() *Palette {
	if p == nil {
		return nil
	}
	cp := &Palette{
		Colors:   append([]color.Color(nil), p.Colors...),
		Name:     p.Name,
		Metadata: p.Metadata,
	}
	if p.Metadata.Params != nil {
		cp.Metadata.Params = maps.Clone(p.Metadata.Params)
	}
	return cp
}
