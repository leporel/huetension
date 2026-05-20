/**
 * State for the active extraction image. Kept in Pinia (session-scoped,
 * not persisted) so the user can switch tabs without losing the image
 * but a fresh reload starts blank — the extracted palette already
 * survives reload via the workspace store.
 *
 * - `imageUrl` is whatever can go straight into `<img src>` and a
 *   `new Image()` for canvas sampling. File uploads become object URLs;
 *   URL inputs reuse the user's URL as-is (CORS-tainted, no live
 *   sampling — the `canvasTainted` flag surfaces this to the pin
 *   overlay).
 * - `naturalSize` lets the pin overlay convert normalised source
 *   coordinates back to image-space pixels without waiting for the
 *   `<img>` load event on every render.
 * - `metadata` carries the extract response's metadata blob so the
 *   freq-bar sidebar can show stats (method, duration_ms, etc.).
 */

import { defineStore } from 'pinia';
import { ref } from 'vue';
import type { ColorJSON, PaletteMetadata } from '../api/types';
import type {
  AnalyzeDistanceTarget,
  AnalyzeMetric,
  AnalyzeSpace,
  AnalyzeStrip,
} from '../api/analyze';

export type ImageSourceKind = 'file' | 'url' | 'data';

export interface ExtractionImage {
  kind: ImageSourceKind;
  /** URL safe to put into `<img src>`. Blob URL for file/data, raw URL otherwise. */
  url: string;
  naturalWidth: number;
  naturalHeight: number;
}

/** Strips are keyed by metric so the template can look one up without
 *  scanning an array. Populated from `/analyze` once per new image; the
 *  image-card strip row reads from this map. */
export type StripMap = Partial<Record<AnalyzeMetric, AnalyzeStrip>>;

export const useExtractionStore = defineStore('extraction', () => {
  const image = ref<ExtractionImage | null>(null);
  const metadata = ref<PaletteMetadata | null>(null);
  /** Last extraction request — held so the user can re-run with a tweaked count without re-uploading. */
  const lastFile = ref<File | null>(null);
  /** Full last-extracted palette (with freq) for the right-side freq-bar
   *  card. Workspace owns the *active* palette (which may diverge from
   *  this once the user drags pins or edits hex); this snapshot stays
   *  pinned to the extraction moment. */
  const lastPalette = ref<ColorJSON[]>([]);
  /** Colour-distribution strips for the current image. Cleared whenever
   *  a new image starts loading and repopulated when `/analyze` resolves;
   *  method/preset re-extractions do NOT touch it (the strips depend only
   *  on pixels, not on the chosen palette algorithm). */
  const strips = ref<StripMap>({});
  /** Colour space the strip-row selectors are set to. Persists across
   *  re-extractions (the analysis controls live in the image card and
   *  shouldn't reset when the method changes). */
  const analyzeSpace = ref<AnalyzeSpace>('oklch');
  /** Distance-target primary the distance strip ranks against. Persists
   *  across re-extractions for the same reason as analyzeSpace. */
  const analyzeDistanceTarget = ref<AnalyzeDistanceTarget>('blue');

  function setImage(next: ExtractionImage | null) {
    // Revoke the previous blob URL to avoid leaking memory; raw URLs
    // (kind === 'url') aren't blob URLs and revoke is a no-op for
    // those.
    if (image.value && image.value.kind !== 'url' && image.value.url.startsWith('blob:')) {
      URL.revokeObjectURL(image.value.url);
    }
    image.value = next;
  }

  function setMetadata(m: PaletteMetadata | null) {
    metadata.value = m;
  }

  function setLastFile(f: File | null) {
    lastFile.value = f;
  }

  function setLastPalette(p: ColorJSON[]) {
    lastPalette.value = p;
  }

  function setStrips(s: AnalyzeStrip[]) {
    const next: StripMap = {};
    for (const strip of s) next[strip.metric] = strip;
    strips.value = next;
  }

  function clearStrips() {
    strips.value = {};
  }

  function setAnalyzeSpace(s: AnalyzeSpace) {
    analyzeSpace.value = s;
  }

  function setAnalyzeDistanceTarget(t: AnalyzeDistanceTarget) {
    analyzeDistanceTarget.value = t;
  }

  function clear() {
    setImage(null);
    setMetadata(null);
    setLastFile(null);
    setLastPalette([]);
    clearStrips();
  }

  return {
    image,
    metadata,
    lastFile,
    lastPalette,
    strips,
    analyzeSpace,
    analyzeDistanceTarget,
    setImage,
    setMetadata,
    setLastFile,
    setLastPalette,
    setStrips,
    clearStrips,
    setAnalyzeSpace,
    setAnalyzeDistanceTarget,
    clear,
  };
});
