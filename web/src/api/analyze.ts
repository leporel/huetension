import { api, type RequestOptions } from './client';

/** Metric labels — match `internal/analyze.AllMetrics` order. */
export type AnalyzeMetric = 'hue' | 'luminance' | 'saturation' | 'distance';

/** Distance-strip primary. */
export type AnalyzeDistanceTarget = 'red' | 'green' | 'blue';

/** Colour space used to sort the hue / luminance / saturation strips.
 *  Distance is space-independent. */
export type AnalyzeSpace = 'oklch' | 'hsl';

export interface AnalyzeStrip {
  metric: AnalyzeMetric;
  /** base64-encoded PNG bytes. */
  content: string;
  encoding: 'base64';
}

export interface AnalyzeResult {
  strips: AnalyzeStrip[];
}

export interface AnalyzeOptions {
  distance_target?: AnalyzeDistanceTarget;
  space?: AnalyzeSpace;
}

interface AnalyzeRequestJSON extends AnalyzeOptions {
  url?: string;
  data?: string;
}

function appendForm(form: FormData, opts: AnalyzeOptions): void {
  if (opts.distance_target) form.set('distance_target', opts.distance_target);
  if (opts.space) form.set('space', opts.space);
}

/** POST /api/v1/analyze via multipart file upload. */
export function analyzeFile(
  file: File | Blob,
  opts: AnalyzeOptions = {},
  reqOpts?: RequestOptions,
): Promise<AnalyzeResult> {
  const form = new FormData();
  form.set('image', file);
  appendForm(form, opts);
  return api.postForm<AnalyzeResult>('/analyze', form, reqOpts);
}

/** POST /api/v1/analyze with a URL (server fetches through sandbox). */
export function analyzeUrl(
  url: string,
  opts: AnalyzeOptions = {},
  reqOpts?: RequestOptions,
): Promise<AnalyzeResult> {
  const body: AnalyzeRequestJSON = { url, ...opts };
  return api.postJSON<AnalyzeResult>('/analyze', body, reqOpts);
}

/**
 * Build a data URL the SPA can drop into an `<img src>` without going through
 * the Blob+ObjectURL dance. The strips are small (~4-10 KiB each) so the
 * extra base64 overhead in the DOM is negligible.
 */
export function stripToDataURL(strip: AnalyzeStrip): string {
  return `data:image/png;base64,${strip.content}`;
}
