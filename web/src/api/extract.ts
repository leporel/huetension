import { api, type RequestOptions } from './client';
import type { PaletteEnvelope } from './types';

export type ExtractMethod = 'soft' | 'freq' | 'kmeans' | 'median';

export interface ExtractOptions {
  count?: number;
  method?: ExtractMethod;
  quality?: 'low' | 'medium' | 'high';
}

interface ExtractRequestJSON extends ExtractOptions {
  url?: string;
  data?: string;
}

/** POST /api/v1/extract via multipart file upload. */
export function extractFile(
  file: File | Blob,
  opts: ExtractOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  const form = new FormData();
  form.set('image', file);
  if (opts.count !== undefined) form.set('count', String(opts.count));
  if (opts.method) form.set('method', opts.method);
  if (opts.quality) form.set('quality', opts.quality);
  return api.postForm<PaletteEnvelope['result']>('/extract', form, reqOpts);
}

/** POST /api/v1/extract with a URL (server fetches through sandbox). */
export function extractUrl(
  url: string,
  opts: ExtractOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  const body: ExtractRequestJSON = { url, ...opts };
  return api.postJSON<PaletteEnvelope['result']>('/extract', body, reqOpts);
}

/** POST /api/v1/extract with a data URI (or raw base64). */
export function extractData(
  data: string,
  opts: ExtractOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  const body: ExtractRequestJSON = { data, ...opts };
  return api.postJSON<PaletteEnvelope['result']>('/extract', body, reqOpts);
}
