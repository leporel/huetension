import { api, type RequestOptions } from './client';
import type { PaletteEnvelope } from './types';
import type { HarmonyType } from './harmony';

export interface RandomOptions {
  count?: number;
  seed?: number;
  harmony?: HarmonyType | 'none';
}

/** GET /api/v1/random. */
export function random(
  opts: RandomOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  return api.get<PaletteEnvelope['result']>('/random', opts, reqOpts);
}
