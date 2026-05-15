<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useExtractionStore } from '../stores/extraction';
import { useWorkspaceStore } from '../stores/workspace';
import { useImagePixelSampler } from '../composables/useImagePixelSampler';

/**
 * Kuler-style pin overlay. Each unlocked workspace slot with a
 * `source` field gets a draggable pin on the image. Pin drag samples
 * the pixel under it and calls `workspace.setSlotPin` so the gesture
 * undoes color + position together.
 *
 * Layout: the image uses `object-fit: contain` inside the frame so
 * its natural aspect ratio is preserved. We compute the visible image
 * rect manually (rather than relying on getBoundingClientRect of the
 * <img>) so pin coords stay correct during layout transitions, and
 * so a "tainted canvas" indicator can show without a live sampler.
 */

const extraction = useExtractionStore();
const workspace = useWorkspaceStore();

const imageUrl = computed(() => extraction.image?.url ?? null);
const sampler = useImagePixelSampler(imageUrl);

const frame = ref<HTMLElement | null>(null);
const frameWidth = ref(0);
const frameHeight = ref(0);

let observer: ResizeObserver | null = null;
onMounted(() => {
  if (!frame.value) return;
  observer = new ResizeObserver(() => {
    if (!frame.value) return;
    const r = frame.value.getBoundingClientRect();
    frameWidth.value = r.width;
    frameHeight.value = r.height;
  });
  observer.observe(frame.value);
  const r = frame.value.getBoundingClientRect();
  frameWidth.value = r.width;
  frameHeight.value = r.height;
});
onUnmounted(() => {
  observer?.disconnect();
  observer = null;
});

/**
 * Visible image rectangle inside the frame given object-fit:contain.
 * Falls back to the whole frame when natural size is unknown — pins
 * will be slightly off until the image loads, but the overlay won't
 * collapse to zero size.
 */
const imageRect = computed(() => {
  const img = extraction.image;
  if (!img || !frameWidth.value || !frameHeight.value) {
    return { x: 0, y: 0, w: frameWidth.value, h: frameHeight.value };
  }
  const fw = frameWidth.value;
  const fh = frameHeight.value;
  const ar = img.naturalWidth / img.naturalHeight;
  const fr = fw / fh;
  if (ar > fr) {
    const w = fw;
    const h = fw / ar;
    return { x: 0, y: (fh - h) / 2, w, h };
  }
  const h = fh;
  const w = fh * ar;
  return { x: (fw - w) / 2, y: 0, w, h };
});

interface PinView {
  slot: number;
  hex: string;
  locked: boolean;
  /** Pixel position within the frame. */
  px: number;
  py: number;
}

const pins = computed<PinView[]>(() => {
  const rect = imageRect.value;
  if (!rect.w || !rect.h) return [];
  return workspace.colors
    .map((s, slot): PinView | null => {
      if (!s.source) return null;
      return {
        slot,
        hex: s.hex,
        locked: s.locked,
        px: rect.x + s.source.x * rect.w,
        py: rect.y + s.source.y * rect.h,
      };
    })
    .filter((p): p is PinView => p !== null);
});

// Drag state — only one pin is dragged at a time. Stored as plain
// refs (no Vue reactivity needed for the active pointerId).
let dragSlot: number | null = null;
let dragElement: Element | null = null;
let dragPointerId: number | null = null;

function pointerToNormalised(e: PointerEvent): { nx: number; ny: number } | null {
  const fr = frame.value?.getBoundingClientRect();
  if (!fr) return null;
  const localX = e.clientX - fr.left;
  const localY = e.clientY - fr.top;
  const rect = imageRect.value;
  if (!rect.w || !rect.h) return null;
  let nx = (localX - rect.x) / rect.w;
  let ny = (localY - rect.y) / rect.h;
  if (nx < 0) nx = 0;
  if (nx > 1) nx = 1;
  if (ny < 0) ny = 0;
  if (ny > 1) ny = 1;
  return { nx, ny };
}

function applyPin(slot: number, nx: number, ny: number): void {
  // Sample first — fallback to the slot's current hex when the canvas
  // is tainted or the sampler isn't ready, so the pin still moves but
  // the color holds.
  const sampled = sampler.sample(nx, ny);
  const cur = workspace.colors[slot];
  if (!cur) return;
  const hex = sampled ?? cur.hex;
  workspace.setSlotPin(slot, hex, { x: nx, y: ny });
}

function onPinDown(e: PointerEvent, slot: number) {
  const cur = workspace.colors[slot];
  if (!cur || cur.locked) return;
  dragSlot = slot;
  dragElement = e.currentTarget as Element;
  dragPointerId = e.pointerId;
  workspace.historyAdapter.pause();
  if (dragElement && 'setPointerCapture' in dragElement) {
    try {
      (dragElement as Element & { setPointerCapture: (id: number) => void })
        .setPointerCapture(e.pointerId);
    } catch {
      /* ignore — drag still works without capture, just clipped to handle */
    }
  }
  e.preventDefault();
  e.stopPropagation();
}

function onPinMove(e: PointerEvent) {
  if (dragSlot === null) return;
  const n = pointerToNormalised(e);
  if (!n) return;
  applyPin(dragSlot, n.nx, n.ny);
}

function onPinUp(e: PointerEvent) {
  if (dragSlot === null) return;
  if (dragElement && dragPointerId !== null) {
    try {
      (dragElement as Element & { releasePointerCapture: (id: number) => void })
        .releasePointerCapture(dragPointerId);
    } catch {
      /* ignore */
    }
  }
  dragSlot = null;
  dragElement = null;
  dragPointerId = null;
  workspace.historyAdapter.resume(true);
  e.stopPropagation();
}
</script>

<template>
  <div ref="frame" class="frame">
    <img
      v-if="extraction.image"
      :src="extraction.image.url"
      class="img"
      alt="extraction source"
      draggable="false"
    />
    <div v-else class="placeholder mono">no image extracted</div>

    <button
      v-for="p in pins"
      :key="p.slot"
      type="button"
      class="pin"
      :class="{ locked: p.locked }"
      :style="{ left: p.px + 'px', top: p.py + 'px', background: p.hex }"
      :title="`Slot ${p.slot + 1} · ${p.hex}`"
      :aria-label="`Pin for slot ${p.slot + 1}, color ${p.hex}`"
      @pointerdown="onPinDown($event, p.slot)"
      @pointermove="onPinMove"
      @pointerup="onPinUp"
      @pointercancel="onPinUp"
    >
      <span class="pin-idx">{{ p.slot + 1 }}</span>
    </button>

    <div v-if="extraction.image && !sampler.ready.value" class="cors-note mono">
      pin sampling unavailable · CORS-blocked or still loading
    </div>
  </div>
</template>

<style scoped>
.frame {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 200px;
  border-radius: var(--r-md);
  overflow: hidden;
  background:
    repeating-linear-gradient(
      45deg,
      oklch(0.22 0.008 50) 0 6px,
      oklch(0.26 0.012 50) 6px 12px
    );
}

.img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  user-select: none;
  -webkit-user-drag: none;
}

.placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--fg-3);
  font-size: 11px;
  letter-spacing: 0.04em;
}

.pin {
  position: absolute;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  transform: translate(-50%, -50%);
  border: 2px solid #fff;
  padding: 0;
  cursor: grab;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow:
    0 0 0 1px oklch(0 0 0 / 0.5),
    0 4px 12px -4px oklch(0 0 0 / 0.6);
  touch-action: none;
  font-family: 'JetBrains Mono', monospace;
}

.pin:hover:not(.locked) {
  width: 26px;
  height: 26px;
}

.pin:active:not(.locked) {
  cursor: grabbing;
}

.pin.locked {
  cursor: not-allowed;
  border-color: var(--accent);
  opacity: 0.85;
}

.pin-idx {
  font-size: 9px;
  font-weight: 700;
  color: oklch(1 0 0 / 0.95);
  text-shadow: 0 1px 1px oklch(0 0 0 / 0.6);
  pointer-events: none;
}

.cors-note {
  position: absolute;
  left: 8px;
  bottom: 8px;
  background: oklch(0 0 0 / 0.55);
  color: oklch(0.82 0.14 85);
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 10px;
  letter-spacing: 0.03em;
}
</style>
