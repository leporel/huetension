/**
 * Client-side contrast math — verbatim port of
 * `internal/contrast/contrast.go`.
 *
 * Lives client-side so the contrast checker updates per-frame as the
 * user nudges fg/bg, with no /api/v1/contrast roundtrip. The API stays
 * the source of truth; parity is pinned by
 * `web/scripts/check-contrast-parity.ts` against a Go-generated
 * fixture (see `internal/contrast/fixture_gen_test.go`).
 *
 * Both algorithms round to 2 decimals exactly where the Go code does,
 * so the fixture comparison is byte-exact rather than within-epsilon.
 */

import { luminance, type RGB } from './useColor';

/** Mirror of `contrast.WCAG21Result`. */
export interface WCAG21Result {
  algo: 'wcag21';
  ratio: number; // 1..21
  aa: boolean; // ratio >= 4.5
  aa_large: boolean; // ratio >= 3
  aaa: boolean; // ratio >= 7
  aaa_large: boolean; // ratio >= 4.5
}

/** Mirror of `contrast.APCAResult`. */
export interface APCAResult {
  algo: 'apca';
  lc: number; // signed Lc, ~ -108..+106
  abs_lc: number;
  body_text: boolean; // |Lc| >= 75
  content: boolean; // |Lc| >= 60
  large_heading: boolean; // |Lc| >= 45
  icon: boolean; // |Lc| >= 30
}

function round2(v: number): number {
  return Math.round(v * 100) / 100;
}

/**
 * WCAG 2.1 contrast ratio of fg over bg. Symmetric — argument order is
 * irrelevant. Ratio is rounded to two decimals to match the Go output.
 */
export function wcag21(fg: RGB, bg: RGB): WCAG21Result {
  const l1 = luminance(fg);
  const l2 = luminance(bg);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  const ratio = round2((lighter + 0.05) / (darker + 0.05));
  return {
    algo: 'wcag21',
    ratio,
    aa: ratio >= 4.5,
    aa_large: ratio >= 3,
    aaa: ratio >= 7,
    aaa_large: ratio >= 4.5,
  };
}

// APCA constants — W3 working draft 0.1.9, verbatim from contrast.go.
const APCA = Object.freeze({
  TRC: 2.4,
  RCO: 0.2126729,
  GCO: 0.7151522,
  BCO: 0.072175,
  NORM_BG: 0.56,
  NORM_TXT: 0.57,
  REV_TXT: 0.62,
  REV_BG: 0.65,
  BLK_THRS: 0.022,
  BLK_CLMP: 1.414,
  SCALE_BOW: 1.14,
  LO_BOW_OFF: 0.027,
  SCALE_WOB: 1.14,
  LO_WOB_OFF: 0.027,
  DELTA_Y_MIN: 0.0005,
  LO_CLIP: 0.1,
});

function apcaY(c: RGB): number {
  const r = Math.pow(c.r / 255, APCA.TRC);
  const g = Math.pow(c.g / 255, APCA.TRC);
  const b = Math.pow(c.b / 255, APCA.TRC);
  return APCA.RCO * r + APCA.GCO * g + APCA.BCO * b;
}

function apcaResult(lc: number): APCAResult {
  const value = round2(lc);
  const abs = round2(Math.abs(lc));
  return {
    algo: 'apca',
    lc: value,
    abs_lc: abs,
    body_text: abs >= 75,
    content: abs >= 60,
    large_heading: abs >= 45,
    icon: abs >= 30,
  };
}

/**
 * APCA perceptual contrast of `text` over `bg`. Argument order matters:
 * text first. Positive Lc = dark text on light bg; negative = the
 * reverse polarity.
 */
export function apca(text: RGB, bg: RGB): APCAResult {
  let txtY = apcaY(text);
  let bgY = apcaY(bg);

  // Soft black clamp near zero luminance.
  if (txtY < APCA.BLK_THRS) {
    txtY += Math.pow(APCA.BLK_THRS - txtY, APCA.BLK_CLMP);
  }
  if (bgY < APCA.BLK_THRS) {
    bgY += Math.pow(APCA.BLK_THRS - bgY, APCA.BLK_CLMP);
  }

  // Below this difference the curve is meaningless — clamp to zero.
  if (Math.abs(bgY - txtY) < APCA.DELTA_Y_MIN) {
    return apcaResult(0);
  }

  let output: number;
  if (bgY > txtY) {
    // Normal polarity: dark text on light background.
    const sapc =
      (Math.pow(bgY, APCA.NORM_BG) - Math.pow(txtY, APCA.NORM_TXT)) * APCA.SCALE_BOW;
    output = sapc < APCA.LO_CLIP ? 0 : sapc - APCA.LO_BOW_OFF;
  } else {
    // Reverse polarity: light text on dark background.
    const sapc =
      (Math.pow(bgY, APCA.REV_BG) - Math.pow(txtY, APCA.REV_TXT)) * APCA.SCALE_WOB;
    output = sapc > -APCA.LO_CLIP ? 0 : sapc + APCA.LO_WOB_OFF;
  }

  return apcaResult(output * 100);
}
