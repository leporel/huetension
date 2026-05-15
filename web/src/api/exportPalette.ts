import { api, type RequestOptions } from './client';
import type { ExportResult } from './types';

export type CSSKind = 'vars' | 'scss' | 'less';

export interface ExportCSSRequest {
  colors: string[];
  name?: string;
  kind?: CSSKind;
}

/** POST /api/v1/export/css. */
export function css(
  req: ExportCSSRequest,
  opts?: RequestOptions,
): Promise<ExportResult> {
  return api.postJSON<ExportResult>('/export/css', req, opts);
}

export interface ExportTailwindRequest {
  colors: string[];
  name?: string;
  shades?: number;
}

/** POST /api/v1/export/tailwind. */
export function tailwind(
  req: ExportTailwindRequest,
  opts?: RequestOptions,
): Promise<ExportResult> {
  return api.postJSON<ExportResult>('/export/tailwind', req, opts);
}
