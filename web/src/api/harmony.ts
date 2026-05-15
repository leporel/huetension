import { api, type RequestOptions } from './client';
import type { PaletteEnvelope } from './types';

export type HarmonyType =
  | 'complementary'
  | 'analogous'
  | 'triadic'
  | 'split'
  | 'tetradic'
  | 'square'
  | 'double'
  | 'monochromatic'
  | 'shades';

export interface HarmonyOptions {
  count?: number;
  step?: number;
}

/** GET /api/v1/harmony/{type}/{color}. */
export function generate(
  type: HarmonyType,
  color: string,
  opts: HarmonyOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  const path = `/harmony/${encodeURIComponent(type)}/${encodeURIComponent(color)}`;
  return api.get<PaletteEnvelope['result']>(path, opts, reqOpts);
}
