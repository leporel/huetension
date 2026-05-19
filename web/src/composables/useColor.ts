/**
 * TS mirror of `internal/color` + `internal/harmony` for the wheel
 * gesture path. Lives client-side because every pointer move must
 * resolve in < 1 frame — a roundtrip to /api/v1/harmony/* would
 * pegcrushing the wheel UX.
 *
 * Parity contract: for the fixture base set in
 * `useColor.parity.test.ts`, this module produces byte-identical hex
 * output to what `/api/v1/harmony/{type}/{color}` returns. Any drift
 * fails CI.
 *
 * Wheel convention: the wheel and per-slot edit intent are HSV (hue /
 * saturation / value). The wheel's *angle* is the RYB artist-wheel hue
 * (see `rgbHueToRybHue`); harmony hue-rotation runs on the RYB wheel
 * via `rotateHueRYB`, mirroring `internal/harmony/harmony.go` — which
 * still works in HSL internally, so the harmony helpers below keep
 * using `fromHSL`/`toHSL` for byte parity. OkLCH is derived for the
 * values panel only — never the gesture input space.
 */

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface RGB {
  r: number;
  g: number;
  b: number;
}

export interface HSL {
  h: number; // 0..360
  s: number; // 0..1
  l: number; // 0..1
}

export interface HSV {
  h: number;
  s: number;
  v: number;
}

export interface OkLab {
  L: number; // 0..1
  a: number;
  b: number;
}

export interface OkLCH {
  L: number;
  C: number;
  H: number; // 0..360
}

// ---------------------------------------------------------------------------
// Math helpers
// ---------------------------------------------------------------------------

export function clamp01(v: number): number {
  if (v < 0) return 0;
  if (v > 1) return 1;
  return v;
}

export function clamp255(v: number): number {
  if (v < 0) return 0;
  if (v > 255) return 255;
  return v;
}

/**
 * Wrap angle to [0, 360). Matches Go's `(x mod 360 + 360) mod 360`
 * pattern — JS `%` returns negative for negative inputs, so the
 * explicit add-360 is required for round-trip determinism.
 */
export function modAngle(deg: number): number {
  const r = deg % 360;
  return r < 0 ? r + 360 : r;
}

function toByte(v: number): number {
  // Match Go's toUint8: round, then clamp.
  const r = Math.round(v);
  if (r < 0) return 0;
  if (r > 255) return 255;
  return r;
}

// ---------------------------------------------------------------------------
// Display formatters
// ---------------------------------------------------------------------------

/**
 * Round `v` to `digits` decimals and strip trailing zeros. Mirrors
 * Go's `formatFloat` close enough for display purposes — exact byte
 * parity with the CLI isn't a goal for the values panel.
 */
function fmt(v: number, digits: number): string {
  return parseFloat(v.toFixed(digits)).toString();
}

export function rgbString(c: RGB): string {
  return `rgb(${Math.round(c.r)}, ${Math.round(c.g)}, ${Math.round(c.b)})`;
}

export function hslString(c: RGB): string {
  const h = toHSL(c);
  return `hsl(${Math.round(h.h)}, ${Math.round(h.s * 100)}%, ${Math.round(h.l * 100)}%)`;
}

export function hsvString(c: RGB): string {
  const h = toHSV(c);
  return `hsv(${Math.round(h.h)}, ${Math.round(h.s * 100)}%, ${Math.round(h.v * 100)}%)`;
}

export function oklabString(c: RGB): string {
  const l = toOkLab(c);
  return `oklab(${fmt(l.L, 4)} ${fmt(l.a, 4)} ${fmt(l.b, 4)})`;
}

export function oklchString(c: RGB): string {
  const l = toOkLCH(c);
  return `oklch(${fmt(l.L, 4)} ${fmt(l.C, 4)} ${fmt(l.H, 1)})`;
}

// ---------------------------------------------------------------------------
// Hex
// ---------------------------------------------------------------------------

export function toHex(c: RGB): string {
  const h = (n: number) => clamp255(Math.round(n)).toString(16).padStart(2, '0');
  return `#${h(c.r)}${h(c.g)}${h(c.b)}`;
}

export function fromHex(hex: string): RGB {
  let s = hex.trim().replace(/^#/, '');
  if (s.length === 3) {
    s = s.split('').map((ch) => ch + ch).join('');
  }
  if (s.length !== 6 || !/^[0-9a-fA-F]{6}$/.test(s)) {
    throw new Error(`useColor: invalid hex ${hex}`);
  }
  return {
    r: parseInt(s.slice(0, 2), 16),
    g: parseInt(s.slice(2, 4), 16),
    b: parseInt(s.slice(4, 6), 16),
  };
}

// ---------------------------------------------------------------------------
// HSL — go-colorful compatible
// ---------------------------------------------------------------------------

export function toHSL(c: RGB): HSL {
  const r = c.r / 255;
  const g = c.g / 255;
  const b = c.b / 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;
  if (max === min) return { h: 0, s: 0, l };
  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  let h: number;
  if (max === r) {
    h = ((g - b) / d + (g < b ? 6 : 0));
  } else if (max === g) {
    h = ((b - r) / d) + 2;
  } else {
    h = ((r - g) / d) + 4;
  }
  return { h: h * 60, s, l };
}

export function fromHSL(h: number, s: number, l: number): RGB {
  h = modAngle(h);
  s = clamp01(s);
  l = clamp01(l);

  if (s === 0) {
    const v = toByte(l * 255);
    return { r: v, g: v, b: v };
  }

  const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
  const p = 2 * l - q;

  const conv = (t: number): number => {
    if (t < 0) t += 1;
    if (t > 1) t -= 1;
    if (t < 1 / 6) return p + (q - p) * 6 * t;
    if (t < 1 / 2) return q;
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
    return p;
  };

  const hk = h / 360;
  return {
    r: toByte(conv(hk + 1 / 3) * 255),
    g: toByte(conv(hk) * 255),
    b: toByte(conv(hk - 1 / 3) * 255),
  };
}

// ---------------------------------------------------------------------------
// HSV — RGB ↔ HSV conversion
// ---------------------------------------------------------------------------

export function toHSV(c: RGB): HSV {
  const r = c.r / 255;
  const g = c.g / 255;
  const b = c.b / 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const d = max - min;
  const v = max;
  const s = max === 0 ? 0 : d / max;
  let h = 0;
  if (d !== 0) {
    if (max === r) h = ((g - b) / d + (g < b ? 6 : 0));
    else if (max === g) h = ((b - r) / d) + 2;
    else h = ((r - g) / d) + 4;
    h *= 60;
  }
  return { h, s, v };
}

export function fromHSV(h: number, s: number, v: number): RGB {
  h = modAngle(h);
  s = clamp01(s);
  v = clamp01(v);
  const c = v * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = v - c;
  let r = 0;
  let g = 0;
  let b = 0;
  const sextant = Math.floor(h / 60);
  switch (sextant) {
    case 0: r = c; g = x; b = 0; break;
    case 1: r = x; g = c; b = 0; break;
    case 2: r = 0; g = c; b = x; break;
    case 3: r = 0; g = x; b = c; break;
    case 4: r = x; g = 0; b = c; break;
    default: r = c; g = 0; b = x; break;
  }
  return {
    r: toByte((r + m) * 255),
    g: toByte((g + m) * 255),
    b: toByte((b + m) * 255),
  };
}

// ---------------------------------------------------------------------------
// OkLab / OkLCH — verbatim matrices from internal/color/oklab.go
// ---------------------------------------------------------------------------

export function srgbToLinear(v: number): number {
  return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
}

function linearToSRGB(v: number): number {
  return v <= 0.0031308 ? v * 12.92 : 1.055 * Math.pow(v, 1 / 2.4) - 0.055;
}

export function toOkLab(c: RGB): OkLab {
  const rl = srgbToLinear(c.r / 255);
  const gl = srgbToLinear(c.g / 255);
  const bl = srgbToLinear(c.b / 255);

  const lp = Math.cbrt(0.4122214708 * rl + 0.5363325363 * gl + 0.0514459929 * bl);
  const mp = Math.cbrt(0.2119034982 * rl + 0.6806995451 * gl + 0.1073969566 * bl);
  const sp = Math.cbrt(0.0883024619 * rl + 0.2817188376 * gl + 0.6299787005 * bl);

  return {
    L: 0.2104542553 * lp + 0.7936177850 * mp - 0.0040720468 * sp,
    a: 1.9779984951 * lp - 2.4285922050 * mp + 0.4505937099 * sp,
    b: 0.0259040371 * lp + 0.7827717662 * mp - 0.8086757660 * sp,
  };
}

export function fromOkLab(lab: OkLab): RGB {
  const lp = lab.L + 0.3963377774 * lab.a + 0.2158037573 * lab.b;
  const mp = lab.L - 0.1055613458 * lab.a - 0.0638541728 * lab.b;
  const sp = lab.L - 0.0894841775 * lab.a - 1.2914855480 * lab.b;

  const l = lp ** 3;
  const m = mp ** 3;
  const s = sp ** 3;

  const rl = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const gl = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const bl = -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s;

  return {
    r: toByte(clamp01(linearToSRGB(rl)) * 255),
    g: toByte(clamp01(linearToSRGB(gl)) * 255),
    b: toByte(clamp01(linearToSRGB(bl)) * 255),
  };
}

export function toOkLCH(c: RGB): OkLCH {
  const lab = toOkLab(c);
  const C = Math.hypot(lab.a, lab.b);
  const H = modAngle((Math.atan2(lab.b, lab.a) * 180) / Math.PI);
  return { L: lab.L, C, H };
}

export function fromOkLCH(L: number, C: number, H: number): RGB {
  const rad = (H * Math.PI) / 180;
  return fromOkLab({ L, a: C * Math.cos(rad), b: C * Math.sin(rad) });
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

/**
 * WCAG 2.1 relative luminance, 0..1. Verbatim mirror of
 * `internal/color/metrics.go::Luminance` — same linearisation and the
 * same 0.2126 / 0.7152 / 0.0722 channel weights. Used by the contrast
 * composable and by the "extract gradient" extreme-luminance pick.
 */
export function luminance(c: RGB): number {
  const rl = srgbToLinear(c.r / 255);
  const gl = srgbToLinear(c.g / 255);
  const bl = srgbToLinear(c.b / 255);
  return 0.2126 * rl + 0.7152 * gl + 0.0722 * bl;
}

// ---------------------------------------------------------------------------
// RYB artist wheel — mirrors internal/color/ryb.go
// ---------------------------------------------------------------------------

/**
 * RGB/HSV hue for each evenly-spaced RYB anchor (index i ⇒ RYB hue i·15°).
 * Verbatim from `internal/color/ryb.go::rybToRGBHue` — the Nodebox / Sighack
 * RYB hue-correction table. Strictly increasing 0→360, which makes the remap
 * invertible.
 */
const RYB_TO_RGB_HUE: readonly number[] = [
  // RYB 0°..120°: red → orange → yellow
  0, 8, 17, 26, 34, 41, 48, 54, 60,
  // RYB 135°..240°: → green → blue
  81, 103, 123, 138, 155, 171, 187, 204,
  // RYB 255°..360°: → violet → red
  219, 234, 251, 267, 282, 298, 329, 360,
];

/** Angular gap between adjacent RYB anchors (= 15°). */
const RYB_ANCHOR_STEP = 360 / (RYB_TO_RGB_HUE.length - 1);

/**
 * Map a hue on the RYB artist wheel to the equivalent HSV/HSL hue (both in
 * degrees). Use it to render the color at an RYB-wheel angle. Mirrors
 * `color.RYBHueToRGBHue`.
 */
export function rybHueToRgbHue(rybHue: number): number {
  rybHue = modAngle(rybHue);
  const seg = rybHue / RYB_ANCHOR_STEP;
  const i = Math.floor(seg);
  const lo = RYB_TO_RGB_HUE[i]!;
  const hi = RYB_TO_RGB_HUE[i + 1]!;
  return lo + (hi - lo) * (seg - i);
}

/**
 * Inverse of `rybHueToRgbHue` — map an HSV/HSL hue to its position on the RYB
 * artist wheel. Use it to place a color's handle on the wheel. Mirrors
 * `color.RGBHueToRYBHue`.
 */
export function rgbHueToRybHue(rgbHue: number): number {
  rgbHue = modAngle(rgbHue);
  // RYB_TO_RGB_HUE is strictly increasing, so exactly one segment contains
  // rgbHue. A linear scan over the 24 segments is negligible.
  for (let i = 0; i < RYB_TO_RGB_HUE.length - 1; i++) {
    const lo = RYB_TO_RGB_HUE[i]!;
    const hi = RYB_TO_RGB_HUE[i + 1]!;
    if (rgbHue < hi) {
      return (i + (rgbHue - lo) / (hi - lo)) * RYB_ANCHOR_STEP;
    }
  }
  return 360; // unreachable: modAngle keeps rgbHue < 360
}

/**
 * Rotate an HSL/HSV hue by `offsetDeg` degrees on the RYB artist wheel: the
 * hue is mapped to its RYB-wheel position, advanced, then mapped back. This
 * is what makes "complementary" land on green rather than cyan. Mirrors
 * `harmony.rotateHueRYB`.
 */
function rotateHueRYB(rgbHue: number, offsetDeg: number): number {
  return rybHueToRgbHue(rgbHueToRybHue(rgbHue) + offsetDeg);
}

// ---------------------------------------------------------------------------
// Harmony — mirrors internal/harmony/harmony.go
// ---------------------------------------------------------------------------

export type HarmonyType =
  | 'complementary'
  | 'analogous'
  | 'triadic'
  | 'split-complementary'
  | 'tetradic'
  | 'square'
  | 'double-complementary'
  | 'compound'
  | 'monochromatic'
  | 'shades';

interface HarmonyOpts {
  count?: number;
  step?: number; // analogous only
}

/** Natural anchor angle offsets per hue-rotation harmony. */
export const HARMONY_ANCHORS: Readonly<Record<string, number[]>> = Object.freeze({
  complementary: [0, 180],
  triadic: [0, 120, 240],
  'split-complementary': [0, 199, 161],
  tetradic: [0, 90, 180, 270],
  square: [0, 90, 180, 270],
  'double-complementary': [0, 38, 180, 218],
  compound: [0, -30, -150, 180],
});

/**
 * Value floor a ramped slot never drops below — verbatim from
 * internal/harmony/harmony.go.
 */
const MONO_MIN_V = 0.2;

/**
 * HSV saturation/value for Monochromatic ramp position `e` (≥ 1) around a
 * base HSV, given the per-step fraction `step`. Saturation is a reflecting
 * triangle wave off the [0,1] gamut edges; value dips one step then ramps
 * away from base (lighter for a dark base, darker for a bright one). `step`
 * is 1/count, so a longer palette subdivides the ramp more finely. Mirrors
 * internal/harmony/harmony.go::monoRamp bit-for-bit.
 */
function monoRamp(
  baseS: number,
  baseV: number,
  step: number,
  e: number,
): { s: number; v: number } {
  let s = baseS;
  let dir = -1;
  for (let k = 0; k < e; k++) {
    let next = s + dir * step;
    if (next < 0 || next > 1) {
      dir = -dir; // bounce off the gamut edge
      next = s + dir * step;
    }
    s = next;
  }
  const vDir = baseV >= 0.5 ? -1 : 1; // bright base ramps darker
  let v = e === 1 ? baseV - step : baseV + vDir * step * e;
  if (v < MONO_MIN_V) v = MONO_MIN_V;
  else if (v > 1) v = 1;
  return { s, v };
}

/**
 * rotateHues mirrors the Go function bit-for-bit: offset 0 returns
 * `base` verbatim (no HSL round-trip); every other offset is rotated
 * on the RYB artist wheel via `rotateHueRYB`. Preserves desaturated
 * bases against drift.
 */
function rotateHues(base: RGB, offsets: number[]): RGB[] {
  const hsl = toHSL(base);
  return offsets.map((o) =>
    o === 0 ? base : fromHSL(rotateHueRYB(hsl.h, o), hsl.s, hsl.l),
  );
}

/**
 * Fill the slots beyond the n natural anchors. Extra slots are grouped
 * into cycles of n: every slot in a cycle reuses an anchor hue round-robin
 * and shares one S/V level, the cycles evenly subdividing the base's S/V
 * down towards zero (1 extra cycle → 50%, 2 → 67%/33%, 3 → 75%/50%/25%).
 * Value is floored at MONO_MIN_V. Mirrors
 * internal/harmony/harmony.go::expandIfNeeded.
 */
function expandIfNeeded(t: HarmonyType, anchors: RGB[], count: number): RGB[] {
  const n = anchors.length;
  if (count === 0 || count === n) return anchors;
  if (count < n) {
    throw new Error(`harmony: count=${count} too small for ${t} (min ${n})`);
  }
  // rotateHues varies only hue, so every anchor shares the base's S/V —
  // anchors[0] (the verbatim base) carries the ramp's reference HSV.
  const base = toHSV(anchors[0]!);
  const cycles = Math.floor((count - 1) / n); // ceil((count-n) / n)
  const out = anchors.slice();
  for (let i = n; i < count; i++) {
    const cycle = Math.floor((i - n) / n) + 1;
    const f = 1 - cycle / (cycles + 1);
    const h = toHSV(anchors[i % n]!).h;
    let v = base.v * f;
    if (v < MONO_MIN_V) v = MONO_MIN_V;
    out.push(fromHSV(h, base.s * f, v));
  }
  return out;
}

function analogous(base: RGB, count: number, step: number): RGB[] {
  const hsl = toHSL(base);
  const half = (count - 1) / 2;
  const out: RGB[] = [];
  for (let i = 0; i < count; i++) {
    const offset = (i - half) * step;
    out.push(
      offset === 0 ? base : fromHSL(rotateHueRYB(hsl.h, offset), hsl.s, hsl.l),
    );
  }
  return out;
}

function monochromatic(base: RGB, count: number): RGB[] {
  if (count === 1) return [base];
  const hsv = toHSV(base);
  const step = 1 / count;
  // Slot 0 is base verbatim — no HSV round-trip drift on desaturated
  // bases; every later slot applies the monoRamp tint/shade ramp.
  const out: RGB[] = [base];
  for (let i = 1; i < count; i++) {
    const { s, v } = monoRamp(hsv.s, hsv.v, step, i);
    out.push(fromHSV(hsv.h, s, v));
  }
  return out;
}

function shades(base: RGB, count: number): RGB[] {
  if (count === 1) return [base];
  const hsl = toHSL(base);
  const startPct = hsl.l * 100;
  const out: RGB[] = [base];
  for (let i = 1; i < count; i++) {
    const lp = startPct - ((startPct - 5) * i) / (count - 1);
    out.push(fromHSL(hsl.h, hsl.s, lp / 100));
  }
  return out;
}

/**
 * Generate a harmony around `base`. The base sits at element 0 for the
 * hue-rotation harmonies, Shades, and Monochromatic, and at the centre
 * for Analogous (see `harmonyBaseIndex`) — an even-count Analogous
 * spreads the base between slots. Mirrors internal/harmony/harmony.go.
 */
export function generateHarmony(
  t: HarmonyType,
  base: RGB,
  opts: HarmonyOpts = {},
): RGB[] {
  const count = opts.count ?? 0;
  switch (t) {
    case 'complementary':
      return expandIfNeeded(t, rotateHues(base, [0, 180]), count);
    case 'triadic':
      return expandIfNeeded(t, rotateHues(base, [0, 120, 240]), count);
    case 'split-complementary':
      return expandIfNeeded(t, rotateHues(base, [0, 199, 161]), count);
    case 'tetradic':
    case 'square':
      return expandIfNeeded(t, rotateHues(base, [0, 90, 180, 270]), count);
    case 'double-complementary':
      return expandIfNeeded(t, rotateHues(base, [0, 38, 180, 218]), count);
    case 'compound':
      return expandIfNeeded(t, rotateHues(base, [0, -30, -150, 180]), count);
    case 'analogous':
      return analogous(base, count || 3, opts.step ?? 30);
    case 'monochromatic':
      return monochromatic(base, count || 5);
    case 'shades':
      return shades(base, count || 5);
  }
}

/**
 * Natural-anchor count for hue-rotation harmonies. Returns 0 for
 * non-anchor harmonies (analogous/monochromatic/shades) which use
 * a count-driven layout instead. Used by ColorWheel to decide how
 * many handles to render on the ring.
 */
export function naturalAnchorCount(t: HarmonyType): number {
  switch (t) {
    case 'complementary': return 2;
    case 'triadic': return 3;
    case 'split-complementary': return 3;
    case 'tetradic':
    case 'square':
      return 4;
    case 'double-complementary': return 4;
    case 'compound': return 4;
    default: return 0;
  }
}

/**
 * Slot index where `generateHarmony` places the base color. Hue-rotation
 * harmonies, Shades, and Monochromatic keep it at 0; only Analogous
 * centres it. For an even-count Analogous the base falls *between* slots
 * — the centre index is returned as the nearest approximation, used only
 * for the wheel's base marker and which-handle-propagates. The
 * authoritative base colour is tracked separately (harmony store), so
 * this never affects regeneration correctness.
 */
export function harmonyBaseIndex(t: HarmonyType, count: number): number {
  if (t === 'analogous') {
    return Math.floor(Math.max(1, count) / 2);
  }
  return 0;
}
