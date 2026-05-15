import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { useStorage } from '@vueuse/core';
import { API_BASE, SCHEMA_VERSION } from '../api';
import type {
  LibraryCategory,
  LibraryIndexEnvelope,
  LibraryPalette,
} from '../api/types';

/**
 * Curated palette catalogue, loaded once per session from
 * `GET /api/v1/library` and filtered client-side (plan §6).
 *
 * The full index is persisted to localStorage together with its ETag.
 * On the next session `load()` issues a conditional request — the
 * server answers a bodiless 304 when the catalogue is unchanged, so a
 * reload never re-downloads the whole dataset. A stale persisted copy
 * is kept on a network failure so the UI degrades gracefully.
 */

interface LibraryCache {
  etag: string;
  categories: LibraryCategory[];
  palettes: LibraryPalette[];
}

const STORAGE_KEY = 'huetension:library';

export const useLibraryStore = defineStore('library', () => {
  // Persisted so the ETag survives reloads and revalidation can 304.
  const cache = useStorage<LibraryCache | null>(STORAGE_KEY, null);

  const loading = ref(false);
  const error = ref<string | null>(null);

  // One revalidation per session; `inflight` dedupes concurrent callers
  // (every view that needs the library calls load() on mount).
  let revalidated = false;
  let inflight: Promise<boolean> | null = null;

  const categories = computed<LibraryCategory[]>(() => cache.value?.categories ?? []);
  const palettes = computed<LibraryPalette[]>(() => cache.value?.palettes ?? []);
  const loaded = computed(() => cache.value !== null);

  async function doLoad(): Promise<boolean> {
    loading.value = true;
    error.value = null;
    try {
      const headers: Record<string, string> = { Accept: 'application/json' };
      const prevEtag = cache.value?.etag;
      if (prevEtag) headers['If-None-Match'] = prevEtag;

      // `no-store`: the localStorage entry is the single cache of
      // record, so the browser HTTP cache must stay out of the way —
      // the conditional request always reaches the server.
      const resp = await fetch(`${API_BASE}/library`, { headers, cache: 'no-store' });

      if (resp.status === 304) return true; // persisted copy still valid
      if (!resp.ok) throw new Error(`library load failed (HTTP ${resp.status})`);

      const json = (await resp.json()) as {
        schema?: string;
        result?: LibraryIndexEnvelope['result'];
      };
      if (json.schema !== SCHEMA_VERSION) {
        throw new Error(`unexpected schema ${json.schema}`);
      }
      if (!json.result) throw new Error('library response missing result');

      cache.value = {
        etag: resp.headers.get('ETag') ?? '',
        categories: json.result.categories,
        palettes: json.result.palettes,
      };
      return true;
    } catch (e) {
      // Keep any stale cache — a flaky network shouldn't blank the UI.
      error.value = e instanceof Error ? e.message : String(e);
      return false;
    } finally {
      loading.value = false;
    }
  }

  /** Load + ETag-revalidate the catalogue, at most once per session. */
  async function load(): Promise<void> {
    if (revalidated) return;
    if (inflight) {
      await inflight;
      return;
    }
    inflight = doLoad();
    const ok = await inflight;
    inflight = null;
    if (ok) revalidated = true; // a failed load stays retryable
  }

  function getById(id: string): LibraryPalette | undefined {
    return cache.value?.palettes.find((p) => p.id === id);
  }

  return { categories, palettes, loaded, loading, error, load, getById };
});
