import { api, type RequestOptions } from './client';

/** LUT output format. `cube` returns a text .cube file; `png` returns a
 *  HALD CLUT image (base64-encoded). */
export type LutFormat = 'cube' | 'png';

export interface LutRequest {
  colors: string[];
  format: LutFormat;
  radius: number;
  distribution: number;
  intensity: number;
  blend_neighbors: number;
  include_saturation: boolean;
  /** Cube grid edge per channel. For `cube` any size ≥ 2 works (default
   *  33). For `png` it must be a perfect square (4, 9, 16, 25, 36, ...)
   *  because the HALD image side is Size·√Size. Default 64 → 512×512. */
  size?: number;
}

export interface LutResult {
  format: LutFormat;
  /** Raw text for `cube`, base64 PNG bytes for `png`. */
  content: string;
  /** Set to "base64" for `png`; omitted for `cube`. */
  encoding?: 'base64';
  filename: string;
}

/** POST /api/v1/lut — generate a 3D colour-grading LUT from a palette. */
export function generate(
  req: LutRequest,
  opts?: RequestOptions,
): Promise<LutResult> {
  return api.postJSON<LutResult>('/lut', req, opts);
}

/** Decode a base64 LUT PNG into a Blob suitable for download. */
export function decodePNG(res: LutResult): Blob {
  const bin = atob(res.content);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new Blob([bytes], { type: 'image/png' });
}

/** Grading knobs sent to /apply-lut — backend re-runs lut.Generate from
 *  these and pipes the resulting .cube text into ffmpeg's `lut3d` filter.
 *  Same shape as LutRequest minus `format` (always cube on the backend). */
export interface ApplyLutParams {
  colors: string[];
  radius: number;
  distribution: number;
  intensity: number;
  blend_neighbors: number;
  include_saturation: boolean;
  size?: number;
}

export interface ApplyLutResult {
  content: string;
  encoding: 'base64';
}

export interface ApplyLutBatchResponse {
  results: ApplyLutResult[];
}

/** POST /api/v1/apply-lut — grade one or more images with the LUT
 *  described by `params`. The cube is generated server-side once per
 *  request and re-used across every image in the batch, so passing the
 *  test image and a reference spectrum strip together costs only the
 *  per-image ffmpeg startup (no double LUT-generation). Throws on 503
 *  when ffmpeg isn't on PATH — caller should surface an install hint. */
export function applyLUT(
  images: Array<File | Blob>,
  params: ApplyLutParams,
  opts?: RequestOptions,
): Promise<ApplyLutBatchResponse> {
  const form = new FormData();
  form.set('params', JSON.stringify(params));
  images.forEach((img, i) => {
    const name = img instanceof File ? img.name : `image_${i}.png`;
    form.append('image', img, name);
  });
  return api.postForm<ApplyLutBatchResponse>('/apply-lut', form, opts);
}

/** Decode a base64 apply-LUT result into a Blob for <img src> via URL.createObjectURL. */
export function decodeApplyResult(res: ApplyLutResult): Blob {
  const bin = atob(res.content);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new Blob([bytes], { type: 'image/png' });
}
