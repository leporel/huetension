import { api, type RequestOptions } from './client';
import type { PaletteEnvelope } from './types';

/** Interpolation spaces accepted by the Go gradient engine. */
export type GradientSpace = 'oklch' | 'oklab' | 'lab' | 'hsl' | 'rgb';
export type GradientEasing = 'linear' | 'ease-in' | 'ease-out' | 'ease-in-out';

export interface GradientOptions {
  from?: string;
  to?: string;
  stops?: string; // comma-separated colors (mutually exclusive with from/to)
  positions?: string; // comma-separated 0..1 stop positions, parallel to stops
  steps: number;
  space?: GradientSpace;
  easing?: GradientEasing;
}

/** GET /api/v1/gradient. */
export function generate(
  opts: GradientOptions,
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  return api.get<PaletteEnvelope['result']>('/gradient', opts, reqOpts);
}
