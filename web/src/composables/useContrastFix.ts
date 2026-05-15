/**
 * "Suggest fixes" search for the contrast checker.
 *
 * Holds the foreground's OkLCH chroma + hue fixed and sweeps lightness,
 * scoring each candidate against the background. Returns the full sweep
 * (for the ECharts histogram) plus the passing lightness nearest to the
 * current one — a minimal, predictable nudge rather than a jump to the
 * extreme.
 *
 * Out-of-gamut (L, C, H) triples are gamut-clamped by `fromOkLCH`; the
 * score and the suggested hex both reflect the clamped color, so the
 * suggestion is always a real, displayable color.
 */

import { fromOkLCH, toHex, toOkLCH, type RGB } from './useColor';
import { apca, wcag21 } from './useContrast';

export type FixAlgo = 'wcag21' | 'apca';

export interface FixSample {
  L: number; // OkLCH lightness, 0..1
  score: number; // WCAG ratio, or |APCA Lc|
  pass: boolean;
}

export interface FixResult {
  algo: FixAlgo;
  target: number;
  samples: FixSample[];
  currentL: number;
  currentScore: number;
  /** Nearest passing lightness to `currentL`, or null when none passes. */
  suggested: { L: number; hex: string; score: number } | null;
}

/** Sweep resolution — 64 bars reads cleanly without crowding the chart. */
const SAMPLES = 64;
const L_MIN = 0.03;
const L_MAX = 0.99;

/** Score `fg` over `bg` under the chosen algorithm (always positive). */
export function contrastScore(fg: RGB, bg: RGB, algo: FixAlgo): number {
  return algo === 'wcag21' ? wcag21(fg, bg).ratio : apca(fg, bg).abs_lc;
}

export function suggestLightnessFix(
  fg: RGB,
  bg: RGB,
  algo: FixAlgo,
  target: number,
): FixResult {
  const base = toOkLCH(fg);
  const samples: FixSample[] = [];

  let suggested: FixResult['suggested'] = null;
  let bestDist = Infinity;

  for (let i = 0; i < SAMPLES; i++) {
    const L = L_MIN + ((L_MAX - L_MIN) * i) / (SAMPLES - 1);
    const rgb = fromOkLCH(L, base.C, base.H);
    const score = contrastScore(rgb, bg, algo);
    const pass = score >= target;
    samples.push({ L, score, pass });

    if (pass) {
      const dist = Math.abs(L - base.L);
      if (dist < bestDist) {
        bestDist = dist;
        suggested = { L, hex: toHex(rgb), score };
      }
    }
  }

  return {
    algo,
    target,
    samples,
    currentL: base.L,
    currentScore: contrastScore(fg, bg, algo),
    suggested,
  };
}
