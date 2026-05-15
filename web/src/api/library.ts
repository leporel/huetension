import { api, type RequestOptions } from './client';
import type {
  LibraryIndexEnvelope,
  LibraryByCategoryEnvelope,
  LibraryPaletteEnvelope,
} from './types';

/** GET /api/v1/library — full catalogue (categories + every palette). */
export function index(opts?: RequestOptions): Promise<LibraryIndexEnvelope['result']> {
  return api.get<LibraryIndexEnvelope['result']>('/library', undefined, opts);
}

/** GET /api/v1/library/{category} — slug or display name. */
export function byCategory(
  category: string,
  opts?: RequestOptions,
): Promise<LibraryByCategoryEnvelope['result']> {
  return api.get<LibraryByCategoryEnvelope['result']>(
    `/library/${encodeURIComponent(category)}`,
    undefined,
    opts,
  );
}

/** GET /api/v1/library/palette/{id}. */
export function get(
  id: string,
  opts?: RequestOptions,
): Promise<LibraryPaletteEnvelope['result']> {
  return api.get<LibraryPaletteEnvelope['result']>(
    `/library/palette/${encodeURIComponent(id)}`,
    undefined,
    opts,
  );
}
