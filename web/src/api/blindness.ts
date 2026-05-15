import { api, type RequestOptions } from './client';
import type { BlindnessResult } from './types';

export type BlindnessKind = 'protan' | 'deutan' | 'tritan' | 'achroma' | 'all';

export interface BlindnessRequest {
  colors: string[];
  kind?: BlindnessKind;
}

/** POST /api/v1/blindness/simulate. */
export function simulate(
  req: BlindnessRequest,
  opts?: RequestOptions,
): Promise<BlindnessResult> {
  return api.postJSON<BlindnessResult>('/blindness/simulate', req, opts);
}
