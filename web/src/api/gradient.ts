import { api, type RequestOptions } from './client';
import type { PaletteEnvelope } from './types';

export interface GradientOptions {
  from?: string;
  to?: string;
  stops?: string; // comma-separated colors (mutually exclusive with from/to)
  steps: number;
  space?: 'oklch' | 'oklab' | 'lab' | 'lch' | 'hsl' | 'srgb';
  easing?: 'linear' | 'ease-in' | 'ease-out' | 'ease-in-out';
}

/** GET /api/v1/gradient. */
export function generate(
  opts: GradientOptions,
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  return api.get<PaletteEnvelope['result']>('/gradient', opts, reqOpts);
}
