/**
 * Client-side color-vision-deficiency simulation — verbatim port of
 * `internal/blindness/blindness.go`.
 *
 * The four Brettel–Viénot–Mollon channel-mixing matrices are applied
 * directly to gamma-encoded sRGB (no linear round-trip), matching the
 * Go implementation and the Coblis / Stark family of simulators.
 *
 * Pure and cheap (one 3×3 multiply per color), so the blindness card
 * recomputes on every workspace change with no API call — same
 * rationale as `useContrast`.
 */

import { type RGB } from './useColor';

/** CVD kind. Mirrors `blindness.Kind`. */
export type BlindnessKind = 'protan' | 'deutan' | 'tritan' | 'achroma';

/** Canonical iteration order — mirrors `blindness.AllKinds`. */
export const BLINDNESS_KINDS: readonly BlindnessKind[] = Object.freeze([
  'protan',
  'deutan',
  'tritan',
  'achroma',
]);

/** Human-readable labels for the simulation strips. */
export const BLINDNESS_LABELS: Readonly<Record<BlindnessKind, string>> = Object.freeze({
  protan: 'Protanopia',
  deutan: 'Deuteranopia',
  tritan: 'Tritanopia',
  achroma: 'Achromatopsia',
});

// Channel-mixing matrices, verbatim from blindness.go::matrices.
const MATRICES: Readonly<Record<BlindnessKind, readonly (readonly number[])[]>> =
  Object.freeze({
    protan: [
      [0.567, 0.433, 0.0],
      [0.558, 0.442, 0.0],
      [0.0, 0.242, 0.758],
    ],
    deutan: [
      [0.625, 0.375, 0.0],
      [0.7, 0.3, 0.0],
      [0.0, 0.3, 0.7],
    ],
    tritan: [
      [0.95, 0.05, 0.0],
      [0.0, 0.433, 0.567],
      [0.0, 0.475, 0.525],
    ],
    achroma: [
      [0.299, 0.587, 0.114],
      [0.299, 0.587, 0.114],
      [0.299, 0.587, 0.114],
    ],
  });

/** Mirror of `blindness.clampByte` — NaN / non-positive → 0, round-half-up. */
function clampByte(v: number): number {
  if (Number.isNaN(v) || v <= 0) return 0;
  if (v >= 255) return 255;
  return Math.floor(v + 0.5);
}

/**
 * Simulate `c` as perceived under the given deficiency. Returns a fresh
 * RGB; the input is not mutated.
 */
export function simulateBlindness(c: RGB, kind: BlindnessKind): RGB {
  const m = MATRICES[kind];
  return {
    r: clampByte(m[0]![0]! * c.r + m[0]![1]! * c.g + m[0]![2]! * c.b),
    g: clampByte(m[1]![0]! * c.r + m[1]![1]! * c.g + m[1]![2]! * c.b),
    b: clampByte(m[2]![0]! * c.r + m[2]![1]! * c.g + m[2]![2]! * c.b),
  };
}
