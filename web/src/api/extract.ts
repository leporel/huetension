import { api, type RequestOptions } from './client';
import type { PaletteEnvelope } from './types';

/**
 * Extraction methods — the real `internal/extract` method names. An
 * unknown value is rejected by the `/api/v1/extract` handler.
 */
export type ExtractMethod =
  | 'soft'
  | 'softk'
  | 'kmeans'
  | 'okkmeans'
  | 'wkmeans'
  | 'mediancut'
  | 'octree'
  | 'wu'
  | 'popularity'
  | 'dbscan';

/** Soft-pipeline presets — accepted only when method is `soft` / `softk`. */
export type SoftPreset =
  | 'default'
  | 'colorful'
  | 'bright'
  | 'muted'
  | 'deep'
  | 'dark';

export interface ExtractOptions {
  /** Palette size. The wire field is `size` (not `count`). */
  size?: number;
  method?: ExtractMethod;
  /** Soft preset. The backend 400s if it is set on a non-soft method. */
  soft_preset?: SoftPreset;
}

interface ExtractRequestJSON {
  url?: string;
  data?: string;
  size?: number;
  method?: ExtractMethod;
  soft_preset?: SoftPreset;
}

// Request builders map ExtractOptions onto the exact wire field names the
// Go handler reads (`size` / `method` / `soft_preset`). They are written
// out field-by-field on purpose: spreading `...opts` is what previously
// leaked an unmapped `count` onto the wire, where it was silently ignored.
function jsonBody(opts: ExtractOptions): Omit<ExtractRequestJSON, 'url' | 'data'> {
  const body: Omit<ExtractRequestJSON, 'url' | 'data'> = {};
  if (opts.size !== undefined) body.size = opts.size;
  if (opts.method) body.method = opts.method;
  if (opts.soft_preset) body.soft_preset = opts.soft_preset;
  return body;
}

function appendForm(form: FormData, opts: ExtractOptions): void {
  if (opts.size !== undefined) form.set('size', String(opts.size));
  if (opts.method) form.set('method', opts.method);
  if (opts.soft_preset) form.set('soft_preset', opts.soft_preset);
}

/** POST /api/v1/extract via multipart file upload. */
export function extractFile(
  file: File | Blob,
  opts: ExtractOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  const form = new FormData();
  form.set('image', file);
  appendForm(form, opts);
  return api.postForm<PaletteEnvelope['result']>('/extract', form, reqOpts);
}

/** POST /api/v1/extract with a URL (server fetches through sandbox). */
export function extractUrl(
  url: string,
  opts: ExtractOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  const body: ExtractRequestJSON = { url, ...jsonBody(opts) };
  return api.postJSON<PaletteEnvelope['result']>('/extract', body, reqOpts);
}

/** POST /api/v1/extract with a data URI (or raw base64). */
export function extractData(
  data: string,
  opts: ExtractOptions = {},
  reqOpts?: RequestOptions,
): Promise<PaletteEnvelope['result']> {
  const body: ExtractRequestJSON = { data, ...jsonBody(opts) };
  return api.postJSON<PaletteEnvelope['result']>('/extract', body, reqOpts);
}
