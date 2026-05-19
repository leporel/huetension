package color

import (
	"math"
	"testing"
)

const lastRYBAnchor = len(rybToRGBHue) - 1 // index of the 360° anchor

// TestRYBHueAnchors checks that every RYB anchor maps to its tabulated RGB hue.
func TestRYBHueAnchors(t *testing.T) {
	for i, want := range rybToRGBHue {
		if i == lastRYBAnchor {
			continue // RYB 360° wraps to 0° under modAngle — see TestRYBHueWrap
		}
		ryb := float64(i) * rybAnchorStep
		if got := RYBHueToRGBHue(ryb); math.Abs(got-want) > 1e-9 {
			t.Errorf("RYBHueToRGBHue(%g) = %g, want %g", ryb, got, want)
		}
	}
}

// TestRGBHueToRYBHueAnchors checks the inverse direction at the anchors.
func TestRGBHueToRYBHueAnchors(t *testing.T) {
	for i, rgb := range rybToRGBHue {
		if i == lastRYBAnchor {
			continue // RGB 360° wraps to 0°
		}
		wantRYB := float64(i) * rybAnchorStep
		if got := RGBHueToRYBHue(rgb); math.Abs(got-wantRYB) > 1e-9 {
			t.Errorf("RGBHueToRYBHue(%g) = %g, want %g", rgb, got, wantRYB)
		}
	}
}

// TestRYBComplementOfRedIsGreen is the headline guarantee of the RYB wheel:
// red's +180° complement is a green, not the cyan the technical wheel gives.
func TestRYBComplementOfRedIsGreen(t *testing.T) {
	if red := RYBHueToRGBHue(0); red != 0 {
		t.Errorf("RYB 0° → RGB %g, want 0 (red)", red)
	}
	green := RYBHueToRGBHue(180)
	if math.Abs(green-138) > 1e-9 {
		t.Errorf("RYB 180° → RGB %g, want 138", green)
	}
	// 138° on the HSV wheel is green; cyan (red's technical complement) is
	// near 180°. Guard the band so a table edit cannot silently regress it.
	if green < 90 || green > 160 {
		t.Errorf("red's RYB complement RGB hue %g is outside the green band", green)
	}
}

// TestRYBHueToRGBHueMonotonic guards the property the inverse relies on.
func TestRYBHueToRGBHueMonotonic(t *testing.T) {
	prev := RYBHueToRGBHue(0)
	for h := 1.0; h < 360; h++ {
		cur := RYBHueToRGBHue(h)
		if cur < prev {
			t.Errorf("not monotonic: RYBHueToRGBHue(%g)=%g < previous %g", h, cur, prev)
		}
		prev = cur
	}
}

// TestRYBHueRoundTrip checks the two functions are inverses over the wheel.
func TestRYBHueRoundTrip(t *testing.T) {
	for h := 0.0; h < 360; h += 0.37 {
		if back := RGBHueToRYBHue(RYBHueToRGBHue(h)); math.Abs(back-h) > 1e-9 {
			t.Errorf("RYB→RGB→RYB: %g came back as %g", h, back)
		}
		if fwd := RYBHueToRGBHue(RGBHueToRYBHue(h)); math.Abs(fwd-h) > 1e-9 {
			t.Errorf("RGB→RYB→RGB: %g came back as %g", h, fwd)
		}
	}
}

// TestRYBHueWrap checks both functions normalize out-of-range input.
func TestRYBHueWrap(t *testing.T) {
	if RYBHueToRGBHue(360) != RYBHueToRGBHue(0) {
		t.Error("RYBHueToRGBHue(360) should equal RYBHueToRGBHue(0)")
	}
	if math.Abs(RYBHueToRGBHue(-30)-RYBHueToRGBHue(330)) > 1e-9 {
		t.Error("RYBHueToRGBHue(-30) should equal RYBHueToRGBHue(330)")
	}
	if RGBHueToRYBHue(360) != RGBHueToRYBHue(0) {
		t.Error("RGBHueToRYBHue(360) should equal RGBHueToRYBHue(0)")
	}
}
