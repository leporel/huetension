/**
 * One off-screen `<canvas>` per loaded image. The canvas is drawn once
 * on image change, then `sample(nx, ny)` only does a single
 * `getImageData(1×1)` per call. Pin drags fire many pointermove events
 * per second; we never want per-move allocations of canvases or
 * `Image` instances.
 *
 * CORS handling: `getImageData` throws on a tainted canvas (image
 * served without permissive CORS). We catch it once and switch into
 * a disabled state — subsequent samples return null without retrying.
 * The pin overlay handles null by leaving the slot's color unchanged.
 */

import { ref, watch, type Ref } from 'vue';

export interface PixelSampler {
  /** Sample the pixel at normalised (0..1) coordinates. Returns null
   *  when the canvas is unavailable or tainted. */
  sample: (nx: number, ny: number) => string | null;
  /** True when the canvas is ready and not tainted. Reactive so
   *  templates can react to the async image-load + CORS outcome. */
  ready: Ref<boolean>;
}

function byte(n: number): string {
  return n.toString(16).padStart(2, '0');
}

export function useImagePixelSampler(imageUrl: Ref<string | null>): PixelSampler {
  let canvas: HTMLCanvasElement | null = null;
  let ctx: CanvasRenderingContext2D | null = null;
  const ready = ref(false);

  watch(
    imageUrl,
    async (url) => {
      canvas = null;
      ctx = null;
      ready.value = false;
      if (!url) return;

      const img = new Image();
      // crossOrigin must be set BEFORE src for browsers to attempt
      // an anonymous CORS load. Blob URLs ignore the attribute (they're
      // same-origin); raw URLs use it.
      img.crossOrigin = 'anonymous';
      img.src = url;
      try {
        await img.decode();
      } catch {
        return;
      }

      const cv = document.createElement('canvas');
      cv.width = img.naturalWidth;
      cv.height = img.naturalHeight;
      const c = cv.getContext('2d', { willReadFrequently: true });
      if (!c) return;
      c.drawImage(img, 0, 0);

      // Probe getImageData once to detect a tainted canvas before any
      // user gesture hits it. SecurityError → keep ready = false.
      try {
        c.getImageData(0, 0, 1, 1);
      } catch {
        return;
      }

      canvas = cv;
      ctx = c;
      ready.value = true;
    },
    { immediate: true },
  );

  function sample(nx: number, ny: number): string | null {
    if (!ready.value || !canvas || !ctx) return null;
    const x = Math.min(canvas.width - 1, Math.max(0, Math.floor(nx * canvas.width)));
    const y = Math.min(canvas.height - 1, Math.max(0, Math.floor(ny * canvas.height)));
    try {
      const d = ctx.getImageData(x, y, 1, 1).data;
      return `#${byte(d[0]!)}${byte(d[1]!)}${byte(d[2]!)}`;
    } catch {
      // Defensive fallback in case the canvas state changes (e.g.,
      // the image is replaced mid-drag); flip ready so the warning
      // surfaces and stop retrying.
      ready.value = false;
      return null;
    }
  }

  return { sample, ready };
}
