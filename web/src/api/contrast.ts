import { api, type RequestOptions } from './client';
import type { ContrastResult } from './types';

export type ContrastAlgo = 'wcag21' | 'apca' | 'both';

/** GET /api/v1/contrast. */
export function check(
  fg: string,
  bg: string,
  algo: ContrastAlgo = 'wcag21',
  opts?: RequestOptions,
): Promise<ContrastResult> {
  return api.get<ContrastResult>('/contrast', { fg, bg, algo }, opts);
}
