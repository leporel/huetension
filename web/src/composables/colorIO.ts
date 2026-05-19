/**
 * Parse / format for the picker's typed color input. Conversions
 * are reused verbatim from useColor.ts — this module only adds the
 * per-format field layer, so useColor.ts stays the single source of
 * truth for the math.
 *
 * HEX is a single text field; the other five formats are entered as
 * three separate numeric fields (HSV as `340` `70` `72`, not the
 * `hsv(340, 70%, 72%)` string).
 */
import {
  clamp255,
  fromHex,
  fromHSL,
  fromHSV,
  fromOkLab,
  fromOkLCH,
  toHex,
  toHSL,
  toHSV,
  toOkLab,
  toOkLCH,
  type RGB,
} from './useColor';

export type ColorFormat = 'hex' | 'rgb' | 'hsv' | 'hsl' | 'oklab' | 'oklch';

/** Dropdown order + display labels for the picker's format `<select>`. */
export const COLOR_FORMATS: ReadonlyArray<{ value: ColorFormat; label: string }> = [
  { value: 'hex', label: 'HEX' },
  { value: 'rgb', label: 'RGB' },
  { value: 'hsv', label: 'HSV' },
  { value: 'hsl', label: 'HSL' },
  { value: 'oklab', label: 'OkLab' },
  { value: 'oklch', label: 'OkLCH' },
];

/** One component field of a multi-field format. */
export interface FormatField {
  label: string;
  /** Decimal places used when displaying the live value in this field. */
  decimals: number;
  /** Value change per arrow-key press / wheel tick (×10 with Shift). */
  step: number;
  /**
   * Inclusive value bounds. They clamp arrow-key / scroll nudges so a
   * value can't run past its valid range, and set the min/max of the
   * per-field range slider. The OkLab a/b and OkLCH C bounds are a
   * comfortable superset of the sRGB gamut — wide enough to reach every
   * displayable colour without the slider feeling cramped.
   */
  min: number;
  max: number;
}

/**
 * Per-format component layout. HEX has no component fields — it is the
 * single-text-field exception, handled separately by the picker.
 */
export const FORMAT_FIELDS: Readonly<Record<ColorFormat, readonly FormatField[]>> = {
  hex: [],
  rgb: [
    { label: 'R', decimals: 0, step: 1, min: 0, max: 255 },
    { label: 'G', decimals: 0, step: 1, min: 0, max: 255 },
    { label: 'B', decimals: 0, step: 1, min: 0, max: 255 },
  ],
  hsv: [
    { label: 'H', decimals: 0, step: 1, min: 0, max: 360 },
    { label: 'S', decimals: 0, step: 1, min: 0, max: 100 },
    { label: 'V', decimals: 0, step: 1, min: 0, max: 100 },
  ],
  hsl: [
    { label: 'H', decimals: 0, step: 1, min: 0, max: 360 },
    { label: 'S', decimals: 0, step: 1, min: 0, max: 100 },
    { label: 'L', decimals: 0, step: 1, min: 0, max: 100 },
  ],
  oklab: [
    { label: 'L', decimals: 4, step: 0.01, min: 0, max: 1 },
    { label: 'a', decimals: 4, step: 0.01, min: -0.4, max: 0.4 },
    { label: 'b', decimals: 4, step: 0.01, min: -0.4, max: 0.4 },
  ],
  oklch: [
    { label: 'L', decimals: 4, step: 0.01, min: 0, max: 1 },
    { label: 'C', decimals: 4, step: 0.01, min: 0, max: 0.4 },
    { label: 'H', decimals: 1, step: 1, min: 0, max: 360 },
  ],
};

/** Raw component values of `rgb` in `fmt` (HSV/HSL saturation as 0–100). */
function componentsOf(fmt: ColorFormat, rgb: RGB): number[] {
  switch (fmt) {
    case 'rgb':
      return [rgb.r, rgb.g, rgb.b];
    case 'hsv': {
      const c = toHSV(rgb);
      return [c.h, c.s * 100, c.v * 100];
    }
    case 'hsl': {
      const c = toHSL(rgb);
      return [c.h, c.s * 100, c.l * 100];
    }
    case 'oklab': {
      const c = toOkLab(rgb);
      return [c.L, c.a, c.b];
    }
    case 'oklch': {
      const c = toOkLCH(rgb);
      return [c.L, c.C, c.H];
    }
    case 'hex':
      return [];
  }
}

/** Round `v` to `decimals` and drop trailing zeros for display. */
function trim(v: number, decimals: number): string {
  return parseFloat(v.toFixed(decimals)).toString();
}

/**
 * `rgb` as the editable field values for `fmt`: a 1-element array (the
 * hex string) for HEX, a 3-element array of component strings otherwise.
 */
export function colorToFields(fmt: ColorFormat, rgb: RGB): string[] {
  if (fmt === 'hex') return [toHex(rgb).toUpperCase()];
  const fields = FORMAT_FIELDS[fmt];
  return componentsOf(fmt, rgb).map((v, i) => trim(v, fields[i]!.decimals));
}

/**
 * Build an RGB color from the picker's field values. Throws on a blank
 * or non-numeric field (HEX: on a malformed hex string) — the caller
 * turns the throw into an inline error and skips the workspace write.
 * Out-of-range numbers are clamped by the conversions, not rejected.
 */
export function fieldsToColor(fmt: ColorFormat, parts: readonly string[]): RGB {
  if (fmt === 'hex') {
    const t = (parts[0] ?? '').trim();
    if (!t) throw new Error('enter a hex color');
    try {
      return fromHex(t);
    } catch {
      throw new Error('invalid hex — use #rgb or #rrggbb');
    }
  }
  const fields = FORMAT_FIELDS[fmt];
  const n: number[] = [];
  for (let i = 0; i < fields.length; i++) {
    const raw = (parts[i] ?? '').trim();
    if (raw === '') throw new Error(`${fields[i]!.label} is empty`);
    const v = Number(raw);
    if (!Number.isFinite(v)) throw new Error(`${fields[i]!.label} is not a number`);
    n.push(v);
  }
  switch (fmt) {
    case 'rgb':
      return { r: clamp255(n[0]!), g: clamp255(n[1]!), b: clamp255(n[2]!) };
    case 'hsv':
      // S / V fields are 0–100 percentages.
      return fromHSV(n[0]!, n[1]! / 100, n[2]! / 100);
    case 'hsl':
      return fromHSL(n[0]!, n[1]! / 100, n[2]! / 100);
    case 'oklab':
      return fromOkLab({ L: n[0]!, a: n[1]!, b: n[2]! });
    case 'oklch':
      return fromOkLCH(n[0]!, n[1]!, n[2]!);
  }
}
