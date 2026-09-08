package web

import (
	"fmt"
	"math"
	"strings"

	"github.com/leporel/huetension/internal/lut"
)

// lutMaxWebSize caps the cube edge accepted over HTTP. The SPA never asks
// for more than 64 (its top "level 8" preset), and lut.Generate's own
// ceiling of 256 means 16.7M nodes per request — enough memory and CPU
// to stall the server from a single unauthenticated loopback call.
// The CLI keeps the full range; it only ever loads the user's own box.
const lutMaxWebSize = 64

// validateWebLUTSize rejects cube edges outside [2, lutMaxWebSize].
func validateWebLUTSize(size int) error {
	if size < 2 || size > lutMaxWebSize {
		return fmt.Errorf("size: must be between 2 and %d, got %d", lutMaxWebSize, size)
	}
	return nil
}

// lutKnobs is the grading parameter set shared by POST /lut and
// POST /apply-lut. Both endpoints re-run lut.Generate from the same
// knobs, so the wire fields, validation and envelope echo live here once
// and the two request types embed it.
type lutKnobs struct {
	Method            string `json:"method,omitempty"` // "grade" | "rbf" | "knn" (legacy default when empty)
	IncludeSaturation bool   `json:"include_saturation"`

	// K-NN knobs.
	Radius         float64 `json:"radius"`
	Distribution   float64 `json:"distribution"`
	Intensity      float64 `json:"intensity"`
	BlendNeighbors int     `json:"blend_neighbors"`

	// RBF knobs.
	Reach     float64 `json:"reach,omitempty"`
	Sharpness float64 `json:"sharpness,omitempty"`
	Strength  float64 `json:"strength,omitempty"`

	// Grade knobs.
	Compression float64 `json:"compression,omitempty"`
	Mute        float64 `json:"mute,omitempty"`
}

// method resolves the wire `method` field to the canonical constant.
// Empty / unrecognised values resolve to "knn" so older clients keep
// producing the legacy output rather than getting a 400; the dispatcher
// in lut.Generate still rejects genuinely invalid values.
func (k lutKnobs) method() string {
	switch strings.ToLower(strings.TrimSpace(k.Method)) {
	case lut.MethodRBF:
		return lut.MethodRBF
	case lut.MethodGrade:
		return lut.MethodGrade
	}
	return lut.MethodKNN
}

// blend clamps the K-NN neighbour count to the algorithm's minimum.
func (k lutKnobs) blend() int {
	return max(k.BlendNeighbors, 1)
}

// validate enforces the per-method knob ranges. lut.Generate validates
// too, but checking here returns a 400 naming the offending field
// instead of the algorithm-side wording.
func (k lutKnobs) validate() error {
	switch k.method() {
	case lut.MethodKNN:
		if !finiteNonNegative(k.Radius) {
			return fmt.Errorf("radius: must be a finite value ≥ 0, got %v", k.Radius)
		}
		if !inUnit(k.Distribution) {
			return fmt.Errorf("distribution: must be in [0, 1], got %v", k.Distribution)
		}
		if !inUnit(k.Intensity) {
			return fmt.Errorf("intensity: must be in [0, 1], got %v", k.Intensity)
		}
	case lut.MethodRBF:
		if !finitePositive(k.Reach) {
			return fmt.Errorf("reach: must be a finite value > 0, got %v", k.Reach)
		}
		if !finitePositive(k.Sharpness) {
			return fmt.Errorf("sharpness: must be a finite value > 0, got %v", k.Sharpness)
		}
		if !inUnit(k.Strength) {
			return fmt.Errorf("strength: must be in [0, 1], got %v", k.Strength)
		}
	case lut.MethodGrade:
		if !inUnit(k.Compression) {
			return fmt.Errorf("compression: must be in [0, 1], got %v", k.Compression)
		}
		if !inUnit(k.Mute) {
			return fmt.Errorf("mute: must be in [0, 1], got %v", k.Mute)
		}
	}
	return nil
}

// options builds the lut.Options for the given cube edge.
func (k lutKnobs) options(size int) lut.Options {
	return lut.Options{
		Size:              size,
		Method:            k.method(),
		IncludeSaturation: k.IncludeSaturation,
		Radius:            k.Radius,
		Distribution:      k.Distribution,
		Intensity:         k.Intensity,
		BlendNeighbors:    k.blend(),
		Reach:             k.Reach,
		Sharpness:         k.Sharpness,
		Strength:          k.Strength,
		Compression:       k.Compression,
		Mute:              k.Mute,
	}
}

// envelopeParams adds the method and only the knobs that method reads
// to the envelope's `params` block, so the echo never advertises fields
// that had no effect on the result.
func (k lutKnobs) envelopeParams(params map[string]any) {
	method := k.method()
	params["method"] = method
	params["include_saturation"] = k.IncludeSaturation
	switch method {
	case lut.MethodKNN:
		params["radius"] = k.Radius
		params["distribution"] = k.Distribution
		params["intensity"] = k.Intensity
		params["blend_neighbors"] = k.blend()
	case lut.MethodRBF:
		params["reach"] = k.Reach
		params["sharpness"] = k.Sharpness
		params["strength"] = k.Strength
	case lut.MethodGrade:
		params["compression"] = k.Compression
		params["mute"] = k.Mute
	}
}

// inUnit reports 0 ≤ v ≤ 1; NaN fails both comparisons and is rejected.
func inUnit(v float64) bool {
	return v >= 0 && v <= 1
}

func finiteNonNegative(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}

func finitePositive(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0
}
