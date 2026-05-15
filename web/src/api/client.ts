import { SCHEMA_VERSION, type Envelope } from './types';

/**
 * Thin fetch wrapper over the huetension/v1 REST API.
 *
 * - Same-origin only — uses relative paths so it works in both dev
 *   flows (Go-fronts-Vite via `--dev`, Vite-fronts-Go via vite.config
 *   proxy) and in production with the embedded SPA.
 * - Envelope unwrap: every non-error response is a
 *   `{schema, tool?, params?, result}` document; callers receive the
 *   typed `result` directly, with the schema asserted.
 * - Error mapping: non-2xx replies carry `{error: string}`; the
 *   wrapper throws an `ApiError` carrying the status + parsed message.
 * - Cancellation: every request accepts an `AbortSignal`. The
 *   component-level `useAbortableRequest` composable is the planned
 *   consumer; this layer only forwards the signal.
 */

export const API_BASE = '/api/v1';

export class ApiError extends Error {
  readonly status: number;
  readonly body?: unknown;

  constructor(message: string, status: number, body?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

export interface RequestOptions {
  signal?: AbortSignal;
  headers?: Record<string, string>;
}

interface JsonBody {
  kind: 'json';
  value: unknown;
}

interface FormBody {
  kind: 'form';
  value: FormData;
}

type RequestBody = JsonBody | FormBody;

/**
 * Permissive query type — accepts any object whose own values stringify
 * cleanly. Closed interface / type-alias shapes from each thin client
 * (HarmonyOptions, GradientOptions, …) don't structurally widen to a
 * Record index signature in TS, so we accept the broader `object` here
 * and runtime-check each value during stringification. Stricter typing
 * lives at each thin client's public API.
 */
export type QueryParams = object;

interface RequestInit_ {
  method: 'GET' | 'POST';
  path: string;
  query?: QueryParams;
  body?: RequestBody;
  signal?: AbortSignal;
  headers?: Record<string, string>;
}

async function request<R>(init: RequestInit_): Promise<R> {
  const url = API_BASE + init.path + buildQuery(init.query);
  const headers: Record<string, string> = { Accept: 'application/json', ...init.headers };

  let body: BodyInit | undefined;
  if (init.body) {
    if (init.body.kind === 'json') {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(init.body.value);
    } else {
      // FormData sets its own Content-Type with boundary.
      body = init.body.value;
    }
  }

  let resp: Response;
  try {
    resp = await fetch(url, { method: init.method, headers, body, signal: init.signal });
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw err;
    }
    throw new ApiError(
      err instanceof Error ? `network: ${err.message}` : 'network failure',
      0,
    );
  }

  const text = await resp.text();
  let parsed: unknown;
  if (text.length > 0) {
    try {
      parsed = JSON.parse(text);
    } catch {
      throw new ApiError(`invalid JSON from ${url}: ${text.slice(0, 120)}`, resp.status);
    }
  }

  if (!resp.ok) {
    const msg = extractErrorMessage(parsed) ?? `request failed (${resp.status})`;
    throw new ApiError(msg, resp.status, parsed);
  }

  if (parsed === undefined || typeof parsed !== 'object' || parsed === null) {
    throw new ApiError(`empty response from ${url}`, resp.status, parsed);
  }

  if ((parsed as { schema?: unknown }).schema !== SCHEMA_VERSION) {
    throw new ApiError(
      `unexpected schema ${(parsed as { schema?: unknown }).schema} (want ${SCHEMA_VERSION})`,
      resp.status,
      parsed,
    );
  }

  return (parsed as Envelope<R>).result;
}

function extractErrorMessage(body: unknown): string | undefined {
  if (body && typeof body === 'object' && 'error' in body) {
    const v = (body as { error: unknown }).error;
    if (typeof v === 'string') return v;
  }
  return undefined;
}

function buildQuery(q?: QueryParams): string {
  if (!q) return '';
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(q)) {
    if (v === undefined || v === null || v === '') continue;
    if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') {
      sp.set(k, String(v));
    }
  }
  const s = sp.toString();
  return s ? `?${s}` : '';
}

export const api = {
  get<R>(
    path: string,
    query?: RequestInit_['query'],
    opts?: RequestOptions,
  ): Promise<R> {
    return request<R>({ method: 'GET', path, query, ...opts });
  },
  postJSON<R>(path: string, value: unknown, opts?: RequestOptions): Promise<R> {
    return request<R>({
      method: 'POST',
      path,
      body: { kind: 'json', value },
      ...opts,
    });
  },
  postForm<R>(path: string, form: FormData, opts?: RequestOptions): Promise<R> {
    return request<R>({
      method: 'POST',
      path,
      body: { kind: 'form', value: form },
      ...opts,
    });
  },
};
