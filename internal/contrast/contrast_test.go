package contrast

import (
	"math"
	"testing"

	"github.com/leporel/huetension/internal/color"
)

func mustParse(t *testing.T, s string) color.Color {
	t.Helper()
	c, err := color.Parse(s)
	if err != nil {
		t.Fatalf("color.Parse(%q): %v", s, err)
	}
	return c
}

func TestWCAGBlackOnWhite(t *testing.T) {
	r := WCAG21(mustParse(t, "black"), mustParse(t, "white"))
	if r.Ratio != 21 {
		t.Errorf("ratio = %v, want 21", r.Ratio)
	}
	if !r.AA || !r.AAA || !r.AALarge || !r.AAALarge {
		t.Errorf("black on white should pass everything: %+v", r)
	}
}

func TestWCAGSameColor(t *testing.T) {
	red := mustParse(t, "red")
	r := WCAG21(red, red)
	if r.Ratio != 1 {
		t.Errorf("ratio = %v, want 1", r.Ratio)
	}
	if r.AA || r.AALarge || r.AAA || r.AAALarge {
		t.Errorf("identical colors should fail every threshold: %+v", r)
	}
}

func TestWCAGSymmetric(t *testing.T) {
	a := mustParse(t, "#3366ff")
	b := mustParse(t, "#fafafa")
	r1 := WCAG21(a, b)
	r2 := WCAG21(b, a)
	if r1.Ratio != r2.Ratio {
		t.Errorf("WCAG should be symmetric: %v vs %v", r1.Ratio, r2.Ratio)
	}
}

func TestWCAGKnownPair(t *testing.T) {
	// #777777 on #ffffff is a textbook example: ratio ≈ 4.48 (just under AA normal).
	r := WCAG21(mustParse(t, "#777777"), mustParse(t, "#ffffff"))
	if math.Abs(r.Ratio-4.48) > 0.05 {
		t.Errorf("ratio = %.2f, want ~4.48", r.Ratio)
	}
	if r.AA {
		t.Errorf("#777 on white should narrowly fail AA: ratio %.2f", r.Ratio)
	}
	if !r.AALarge {
		t.Errorf("#777 on white should pass AA-large: ratio %.2f", r.Ratio)
	}
}

func TestAPCABlackOnWhite(t *testing.T) {
	// Reference Lc for #000 text on #fff bg is ~106 per the W3 spec.
	r := APCA(mustParse(t, "black"), mustParse(t, "white"))
	if r.Lc < 105 || r.Lc > 107 {
		t.Errorf("APCA(black, white).Lc = %v, want ~106", r.Lc)
	}
	if !r.BodyText || !r.Content {
		t.Errorf("black on white should clear all flags: %+v", r)
	}
}

func TestAPCAWhiteOnBlack(t *testing.T) {
	// Reverse polarity: Lc ≈ -107.88.
	r := APCA(mustParse(t, "white"), mustParse(t, "black"))
	if r.Lc > -107 || r.Lc < -109 {
		t.Errorf("APCA(white, black).Lc = %v, want ~-108", r.Lc)
	}
	if r.AbsLc < 75 {
		t.Errorf("AbsLc = %v should pass body-text threshold", r.AbsLc)
	}
}

func TestAPCAEqualColors(t *testing.T) {
	c := mustParse(t, "#3366ff")
	r := APCA(c, c)
	if r.Lc != 0 {
		t.Errorf("identical colors should give Lc=0, got %v", r.Lc)
	}
	if r.BodyText || r.Content || r.LargeHeading || r.Icon {
		t.Errorf("identical colors should fail every threshold: %+v", r)
	}
}

func TestAPCAPolarity(t *testing.T) {
	// Same magnitude, opposite signs.
	dark := mustParse(t, "#222222")
	light := mustParse(t, "#eeeeee")
	pos := APCA(dark, light) // dark text on light bg → positive
	neg := APCA(light, dark) // light text on dark bg → negative
	if pos.Lc <= 0 {
		t.Errorf("expected positive Lc for dark-on-light, got %v", pos.Lc)
	}
	if neg.Lc >= 0 {
		t.Errorf("expected negative Lc for light-on-dark, got %v", neg.Lc)
	}
}

func TestCheckDispatch(t *testing.T) {
	black := mustParse(t, "black")
	white := mustParse(t, "white")

	got, err := Check(black, white, AlgoWCAG21)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(WCAG21Result); !ok {
		t.Errorf("Check wcag21 returned %T", got)
	}

	got, err = Check(black, white, AlgoAPCA)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(APCAResult); !ok {
		t.Errorf("Check apca returned %T", got)
	}

	// Empty algo defaults to WCAG21.
	got, err = Check(black, white, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(WCAG21Result); !ok {
		t.Errorf("Check default returned %T", got)
	}

	if _, err := Check(black, white, Algo("not-a-thing")); err == nil {
		t.Errorf("unknown algo should error")
	}
}

func TestSuggestRejectsBadInputs(t *testing.T) {
	fg := mustParse(t, "#777777")
	bg := mustParse(t, "#ffffff")
	if _, err := Suggest(fg, bg, AlgoWCAG21, 0); err == nil {
		t.Errorf("zero target should error")
	}
	if _, err := Suggest(fg, bg, AlgoWCAG21, -1); err == nil {
		t.Errorf("negative target should error")
	}
	if _, err := Suggest(fg, bg, Algo("both"), 4.5); err == nil {
		t.Errorf("algo=both should error for Suggest")
	}
	if _, err := Suggest(fg, bg, Algo("nope"), 4.5); err == nil {
		t.Errorf("unknown algo should error")
	}
}

func TestSuggestAlreadyPassingNudgesMinimally(t *testing.T) {
	black := mustParse(t, "#000000")
	white := mustParse(t, "#ffffff")
	res, err := Suggest(black, white, AlgoWCAG21, 4.5)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if res.Suggested == nil {
		t.Fatalf("black on white must have a passing suggestion at AA")
	}
	// Foreground already passes (ratio 21); the nearest passing L is L0
	// itself, so the suggested L should sit very close to the current one.
	if math.Abs(res.Suggested.L-res.CurrentL) > 0.05 {
		t.Errorf("suggested L jumped from %v to %v even though current passes", res.CurrentL, res.Suggested.L)
	}
	if !(res.Suggested.Score >= 4.5) {
		t.Errorf("suggested score %v should clear target 4.5", res.Suggested.Score)
	}
	if len(res.Samples) != suggestSamples {
		t.Errorf("len(samples) = %d, want %d", len(res.Samples), suggestSamples)
	}
}

func TestSuggestFailingFGFindsFix(t *testing.T) {
	// Mid-grey on white fails WCAG AA. The fix should pull L darker until
	// the ratio reaches the target.
	fg := mustParse(t, "#888888")
	bg := mustParse(t, "#ffffff")
	res, err := Suggest(fg, bg, AlgoWCAG21, 4.5)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if res.CurrentScore >= 4.5 {
		t.Fatalf("test premise broken: %v already passes", res.CurrentScore)
	}
	if res.Suggested == nil {
		t.Fatalf("a darker grey on white should reach 4.5")
	}
	if res.Suggested.L >= res.CurrentL {
		t.Errorf("on white, suggested L (%v) should be darker than current (%v)", res.Suggested.L, res.CurrentL)
	}
	if res.Suggested.Score < 4.5 {
		t.Errorf("suggested score %v < target 4.5", res.Suggested.Score)
	}
}

func TestSuggestNoPassReturnsNilSuggested(t *testing.T) {
	// Same-color pair can never reach a meaningful target; the sweep
	// holds chroma + hue fixed so no L produces real contrast.
	red := mustParse(t, "red")
	res, err := Suggest(red, red, AlgoWCAG21, 4.5)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if res.Suggested != nil {
		t.Errorf("identical fg/bg should not yield a suggestion, got %+v", res.Suggested)
	}
	for _, s := range res.Samples {
		if s.Pass {
			t.Errorf("identical fg/bg sweep should have no passing sample, got %+v", s)
		}
	}
}

func TestSuggestAPCAUsesAbsLc(t *testing.T) {
	// Reverse polarity: white on black has a strongly negative Lc.
	// Suggest treats the magnitude as the score, so target 60 should
	// already pass at the current L without nudging.
	white := mustParse(t, "#ffffff")
	black := mustParse(t, "#000000")
	res, err := Suggest(white, black, AlgoAPCA, 60)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if res.CurrentScore < 60 {
		t.Errorf("white on black |Lc| = %v, expected ≥ 60", res.CurrentScore)
	}
	if res.Suggested == nil {
		t.Fatalf("APCA suggest should find a passing L")
	}
	if res.Algo != AlgoAPCA {
		t.Errorf("result algo = %v, want apca", res.Algo)
	}
}
