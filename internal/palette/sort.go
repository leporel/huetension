package palette

import (
	"fmt"
	"sort"

	"github.com/leporel/huetension/internal/color"
)

// SortBy is the key passed to Sort and SortColors.
type SortBy string

const (
	SortByLuminance  SortBy = "luminance"  // WCAG relative luminance
	SortByLightness  SortBy = "lightness"  // HSL L
	SortByOkL        SortBy = "okl"        // OkLab L (perceptually uniform)
	SortByHue        SortBy = "hue"
	SortBySaturation SortBy = "saturation"
	SortByFrequency  SortBy = "frequency"
)

// Sort reorders the palette by the chosen key. The sort is stable, so ties
// preserve original ordering. With reverse=false the order is ascending
// (e.g. dark→light for luminance); reverse=true flips it.
func (p *Palette) Sort(by SortBy, reverse bool) error {
	less, err := lessFn(by)
	if err != nil {
		return err
	}
	if p == nil || len(p.Colors) <= 1 {
		return nil
	}
	sort.SliceStable(p.Colors, func(i, j int) bool {
		if reverse {
			return less(p.Colors[j], p.Colors[i])
		}
		return less(p.Colors[i], p.Colors[j])
	})
	return nil
}

// SortColors is the slice-only convenience used by the CLI sort sub-command.
func SortColors(in []color.Color, by SortBy, reverse bool) ([]color.Color, error) {
	p := New(in)
	if err := p.Sort(by, reverse); err != nil {
		return nil, err
	}
	return p.Colors, nil
}

func lessFn(by SortBy) (func(a, b color.Color) bool, error) {
	switch by {
	case SortByLuminance:
		return func(a, b color.Color) bool { return a.Luminance() < b.Luminance() }, nil
	case SortByLightness:
		return func(a, b color.Color) bool { return a.Lightness() < b.Lightness() }, nil
	case SortByOkL:
		return func(a, b color.Color) bool { return a.OkL() < b.OkL() }, nil
	case SortByHue:
		return func(a, b color.Color) bool { return a.HueDeg() < b.HueDeg() }, nil
	case SortBySaturation:
		return func(a, b color.Color) bool { return a.Saturation() < b.Saturation() }, nil
	case SortByFrequency:
		return func(a, b color.Color) bool { return a.Freq < b.Freq }, nil
	}
	return nil, fmt.Errorf("palette: unknown sort key %q", string(by))
}
