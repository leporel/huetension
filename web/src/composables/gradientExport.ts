/**
 * Pure builders that turn a gradient into copy / download-ready text.
 *
 * CSS is emitted straight from the stops + their positions — an exact
 * `linear-gradient(in <space> …)` one-liner. It is accurate for linear
 * easing; CSS gradients cannot express an easing curve, so a non-linear
 * easing is not reflected (same limitation as the live preview bar).
 *
 * The GIMP / SVG / JSON builders work off the discrete N-step output of
 * /api/v1/gradient. Those steps already bake in the chosen space, easing
 * and stop positions, so placing them at even offsets faithfully
 * reproduces the curve as a piecewise approximation.
 */

import type { GradientSpace } from '../api/gradient';
import type { ColorJSON, PaletteEnvelope } from '../api/types';
import { SCHEMA_VERSION } from '../api/types';

/** Our gradient space → the CSS `color-interpolation-method` keyword. */
export const CSS_INTERP: Record<GradientSpace, string> = {
  oklch: 'oklch',
  oklab: 'oklab',
  lab: 'lab',
  hsl: 'hsl',
  rgb: 'srgb',
};

/** A copy / download artifact: the text plus its file metadata. */
export interface GradientArtifact {
  text: string;
  ext: string;
  mime: string;
}

/** Sanitise a name into a CSS class / SVG id token; never empty. */
export function safeIdent(name: string): string {
  let cleaned = name
    .trim()
    .replace(/[^\w-]+/g, '-')
    .replace(/^-+|-+$/g, '');
  // CSS classes and SVG ids may not start with a digit.
  if (/^[0-9]/.test(cleaned)) cleaned = `g-${cleaned}`;
  return cleaned || 'gradient';
}

/** Percentage offset of the i-th of n evenly-sampled discrete steps. */
function evenOffset(i: number, n: number): number {
  return n > 1 ? (i / (n - 1)) * 100 : 0;
}

/** 0..255 channel → GIMP's 0..1 six-decimal float. */
function channelFloat(channel: number): string {
  return (channel / 255).toFixed(6);
}

/** CSS `linear-gradient(in <space> …)` rule from the stops and positions. */
export function gradientToCSS(
  stops: string[],
  positions: number[],
  space: GradientSpace,
  name: string,
): GradientArtifact {
  const parts = stops.map(
    (hex, i) => `${hex.toLowerCase()} ${((positions[i] ?? 0) * 100).toFixed(1)}%`,
  );
  const text =
    `.${safeIdent(name)} {\n` +
    `  background: linear-gradient(in ${CSS_INTERP[space]} 90deg, ${parts.join(', ')});\n` +
    `}\n`;
  return { text, ext: 'css', mime: 'text/css' };
}

/**
 * GIMP gradient (.ggr): the discrete output as N-1 linear RGB segments.
 * GIMP's blend types have no OkLCH, so sampling the eased / positioned
 * curve into linear segments is the faithful path.
 */
export function gradientToGGR(result: ColorJSON[], name: string): GradientArtifact {
  const segs = Math.max(0, result.length - 1);
  const lines = ['GIMP Gradient', `Name: ${name.trim() || 'gradient'}`, String(segs)];
  for (let i = 0; i < segs; i++) {
    const [ar, ag, ab] = result[i]!.rgb;
    const [br, bg, bb] = result[i + 1]!.rgb;
    const left = i / segs;
    const right = (i + 1) / segs;
    const mid = (left + right) / 2;
    // left middle right · r0 g0 b0 a0 · r1 g1 b1 a1 · blend(0=linear) color(0=RGB)
    lines.push(
      [
        left.toFixed(6),
        mid.toFixed(6),
        right.toFixed(6),
        channelFloat(ar),
        channelFloat(ag),
        channelFloat(ab),
        '1.000000',
        channelFloat(br),
        channelFloat(bg),
        channelFloat(bb),
        '1.000000',
        '0',
        '0',
      ].join(' '),
    );
  }
  return { text: `${lines.join('\n')}\n`, ext: 'ggr', mime: 'application/octet-stream' };
}

/** Standalone SVG document carrying a <linearGradient> of the discrete output. */
export function gradientToSVG(result: ColorJSON[], name: string): GradientArtifact {
  const id = safeIdent(name);
  const n = result.length;
  const stops = result
    .map(
      (c, i) =>
        `      <stop offset="${evenOffset(i, n).toFixed(1)}%" stop-color="${c.hex.toLowerCase()}" />`,
    )
    .join('\n');
  const text =
    `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="64" viewBox="0 0 320 64">\n` +
    `  <defs>\n` +
    `    <linearGradient id="${id}" x1="0%" y1="0%" x2="100%" y2="0%">\n` +
    `${stops}\n` +
    `    </linearGradient>\n` +
    `  </defs>\n` +
    `  <rect width="320" height="64" fill="url(#${id})" />\n` +
    `</svg>\n`;
  return { text, ext: 'svg', mime: 'image/svg+xml' };
}

/**
 * huetension/v1 envelope of the discrete gradient palette. The backend
 * hard-codes `palette.name` to "gradient"; we override it with the user's
 * name so the JSON matches the CSS / SVG / GGR exports.
 */
export function gradientToJSON(
  result: PaletteEnvelope['result'],
  params: Record<string, unknown>,
  name: string,
): GradientArtifact {
  const envelope: PaletteEnvelope = {
    schema: SCHEMA_VERSION,
    tool: 'gradient.generate',
    params,
    result: {
      ...result,
      palette: { ...result.palette, name: name.trim() || 'gradient' },
    },
  };
  return {
    text: `${JSON.stringify(envelope, null, 2)}\n`,
    ext: 'json',
    mime: 'application/json',
  };
}
