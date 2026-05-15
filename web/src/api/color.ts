import { api, type RequestOptions } from './client';
import type { ConvertResult, PaletteEnvelope } from './types';

/** GET /api/v1/color/convert. */
export function convert(
  color: string,
  to?: string,
  opts?: RequestOptions,
): Promise<ConvertResult> {
  return api.get<ConvertResult>('/color/convert', { color, to }, opts);
}

export interface SortRequest {
  colors: string[];
  by?: string;
  reverse?: boolean;
}

/** POST /api/v1/color/sort. */
export function sort(
  req: SortRequest,
  opts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  return api.postJSON<PaletteEnvelope['result']>('/color/sort', req, opts);
}
