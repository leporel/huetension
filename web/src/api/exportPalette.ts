import { api, type RequestOptions } from './client';
import type { ExportResult } from './types';

/** Every format POST /export accepts — mirrors exporter.AllFormats. */
export type ExportFormat =
  | 'json'
  | 'css'
  | 'scss'
  | 'less'
  | 'tailwind'
  | 'txt'
  | 'gpl'
  | 'ggr'
  | 'svg'
  | 'png'
  | 'jpeg'
  | 'ase'
  | 'aco';

export interface ExportRequest {
  format: ExportFormat;
  colors: string[];
  name?: string;
  /** Shade-scale count; only meaningful for format=tailwind. */
  shades?: number;
}

/** POST /api/v1/export — render a color list into any exporter format.
 *  Binary formats (png/jpeg) come back base64-encoded; see ExportResult. */
export function general(
  req: ExportRequest,
  opts?: RequestOptions,
): Promise<ExportResult> {
  return api.postJSON<ExportResult>('/export', req, opts);
}

/** MIME type for a base64 ExportResult. Image formats get a real image
 *  type so previews render; the Adobe swatch formats are opaque binaries
 *  meant only for download. */
function binaryMime(format: string): string {
  switch (format) {
    case 'jpeg':
      return 'image/jpeg';
    case 'png':
      return 'image/png';
    default:
      return 'application/octet-stream';
  }
}

/** Decode a binary ExportResult (encoding === "base64") into a Blob.
 *  Shared by every caller that needs to preview or download binary
 *  output (png/jpeg images, Adobe ase/aco swatches). */
export function decodeBinary(res: ExportResult): Blob {
  const bin = atob(res.content);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new Blob([bytes], { type: binaryMime(res.format) });
}
