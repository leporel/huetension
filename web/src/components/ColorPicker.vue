<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import {
  fromHex,
  fromHSV,
  toHex,
  toHSV,
} from '../composables/useColor';
import { useHarmonyApply } from '../composables/useHarmonyApply';

// Geometry — matches the design source's .picker-wheel (sat square
// fits a 232×232 socket). Kept square so we can use integer pixels
// for the knob math; the hue bar lives below.
const SV_SIZE = 220;
const HUE_HEIGHT = 14;

const workspace = useWorkspaceStore();
const { applyBaseHex } = useHarmonyApply();

const base = computed(() => workspace.colors[0]);

/**
 * The picker reads slot 0's HSV. Stored locally rather than recomputed
 * from `base.hex` on every paint so a small mid-drag rounding (HSV→hex
 * lossy on grayscale) doesn't jitter the knob.
 */
const local = ref<{ h: number; s: number; v: number }>({ h: 0, s: 0, v: 1 });

// Drag state — declared before the watch so the immediate-fire path
// can safely read `dragTarget`. Module-top-level `let` values are
// hoisted but only readable after the let statement executes.
let dragTarget: 'sv' | 'hue' | null = null;
let dragEl: HTMLElement | null = null;
let dragPointerId: number | null = null;

watch(
  () => base.value?.hex,
  (hex) => {
    if (!hex) return;
    // During a drag the picker is the source of truth — ignore the
    // round-trip echo through workspace so the knob doesn't jitter
    // on near-grayscale colors (HSV→hex→HSV is lossy when s≈0).
    if (dragTarget !== null) return;
    local.value = toHSV(fromHex(hex));
  },
  { immediate: true },
);

const svBackground = computed(() => {
  // Pure hue at S=1 V=1 used as the right edge of the SV gradient.
  const pure = toHex(fromHSV(local.value.h, 1, 1));
  // CSS layer order: bottom layer (declared last) is painted first.
  // We want black-vertical to overlay the hue-horizontal — declaring
  // the vertical first puts it on top in stacking order.
  return [
    `linear-gradient(to top, #000, transparent)`,
    `linear-gradient(to right, #fff, ${pure})`,
  ].join(', ');
});

const knobStyle = computed(() => ({
  left: `${local.value.s * SV_SIZE}px`,
  top: `${(1 - local.value.v) * SV_SIZE}px`,
}));

const hueKnobStyle = computed(() => ({
  left: `${(local.value.h / 360) * SV_SIZE}px`,
}));

function pointerToSVLocal(e: PointerEvent, el: HTMLElement): { x: number; y: number } {
  const r = el.getBoundingClientRect();
  let x = ((e.clientX - r.left) / r.width) * SV_SIZE;
  let y = ((e.clientY - r.top) / r.height) * SV_SIZE;
  if (x < 0) x = 0;
  if (x > SV_SIZE) x = SV_SIZE;
  if (y < 0) y = 0;
  if (y > SV_SIZE) y = SV_SIZE;
  return { x, y };
}

function pointerToHueLocal(e: PointerEvent, el: HTMLElement): number {
  const r = el.getBoundingClientRect();
  let x = ((e.clientX - r.left) / r.width) * SV_SIZE;
  if (x < 0) x = 0;
  if (x > SV_SIZE) x = SV_SIZE;
  return (x / SV_SIZE) * 360;
}

function commit(): void {
  applyBaseHex(toHex(fromHSV(local.value.h, local.value.s, local.value.v)));
}

function onSVDown(e: PointerEvent): void {
  if (workspace.colors[0]?.locked) return;
  const el = e.currentTarget as HTMLElement;
  dragTarget = 'sv';
  dragEl = el;
  dragPointerId = e.pointerId;
  workspace.historyAdapter.pause();
  el.setPointerCapture(e.pointerId);
  const { x, y } = pointerToSVLocal(e, el);
  local.value = { ...local.value, s: x / SV_SIZE, v: 1 - y / SV_SIZE };
  commit();
  e.preventDefault();
}

function onHueDown(e: PointerEvent): void {
  if (workspace.colors[0]?.locked) return;
  const el = e.currentTarget as HTMLElement;
  dragTarget = 'hue';
  dragEl = el;
  dragPointerId = e.pointerId;
  workspace.historyAdapter.pause();
  el.setPointerCapture(e.pointerId);
  local.value = { ...local.value, h: pointerToHueLocal(e, el) };
  commit();
  e.preventDefault();
}

function onMove(e: PointerEvent): void {
  if (!dragTarget || !dragEl) return;
  if (dragTarget === 'sv') {
    const { x, y } = pointerToSVLocal(e, dragEl);
    local.value = { ...local.value, s: x / SV_SIZE, v: 1 - y / SV_SIZE };
  } else {
    local.value = { ...local.value, h: pointerToHueLocal(e, dragEl) };
  }
  commit();
}

function onUp(_e: PointerEvent): void {
  if (!dragTarget) return;
  if (dragEl && dragPointerId !== null) {
    try { dragEl.releasePointerCapture(dragPointerId); } catch { /* ignore */ }
  }
  dragTarget = null;
  dragEl = null;
  dragPointerId = null;
  workspace.historyAdapter.resume(true);
}
</script>

<template>
  <div class="picker">
    <div
      class="sv"
      :style="{
        width: SV_SIZE + 'px',
        height: SV_SIZE + 'px',
        background: svBackground,
      }"
      @pointerdown="onSVDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
    >
      <span class="knob" :style="knobStyle" :aria-label="`saturation ${Math.round(local.s * 100)}% value ${Math.round(local.v * 100)}%`" />
    </div>
    <div
      class="hue"
      :style="{ width: SV_SIZE + 'px', height: HUE_HEIGHT + 'px' }"
      @pointerdown="onHueDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
    >
      <span class="knob hue-knob" :style="hueKnobStyle" :aria-label="`hue ${Math.round(local.h)} degrees`" />
    </div>

    <div class="hsv-readout mono">
      <span>H {{ Math.round(local.h) }}°</span>
      <span>S {{ Math.round(local.s * 100) }}%</span>
      <span>V {{ Math.round(local.v * 100) }}%</span>
    </div>
  </div>
</template>

<style scoped>
.picker {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
}

.sv {
  position: relative;
  border-radius: 8px;
  box-shadow: 0 0 0 1px var(--line-soft);
  cursor: crosshair;
  touch-action: none;
}

.hue {
  position: relative;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    hsl(0 90% 55%),
    hsl(60 90% 60%),
    hsl(120 70% 50%),
    hsl(180 65% 50%),
    hsl(240 75% 60%),
    hsl(300 75% 58%),
    hsl(360 90% 55%)
  );
  cursor: ew-resize;
  touch-action: none;
}

.knob {
  position: absolute;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid #fff;
  box-shadow: 0 0 0 1px oklch(0 0 0 / 0.5);
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.hue-knob {
  top: 50%;
}

.hsv-readout {
  display: flex;
  gap: 14px;
  font-size: 11px;
  color: var(--fg-2);
  margin-top: 2px;
}
</style>
