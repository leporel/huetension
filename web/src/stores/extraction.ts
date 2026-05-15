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

export type ImageSourceKind = 'file' | 'url' | 'data';

export interface ExtractionImage {
  kind: ImageSourceKind;
  /** URL safe to put into `<img src>`. Blob URL for file/data, raw URL otherwise. */
  url: string;
  naturalWidth: number;
  naturalHeight: number;
}

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

  function clear() {
    setImage(null);
    setMetadata(null);
    setLastFile(null);
    setLastPalette([]);
  }

  return {
    image,
    metadata,
    lastFile,
    lastPalette,
    setImage,
    setMetadata,
    setLastFile,
    setLastPalette,
    clear,
  };
});
