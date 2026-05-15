<script setup lang="ts">
import { computed, ref } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import {
  fromHex,
  fromHSL,
  toHex,
  toHSL,
} from '../composables/useColor';
import { useHarmonyApply } from '../composables/useHarmonyApply';
import {
  useHandleGesture,
  type WheelGeometry,
} from '../composables/useHandleGesture';

// Geometry: SVG is fixed 320×320 at CSS-px (matches design source
// .wheel { width: 320px; height: 320px; }). Ring sits between
// rInner and rOuter; mask gives the visible donut.
const SIZE = 320;
const CENTER = SIZE / 2;
const R_OUTER = 158;
const R_INNER = 102;

// One lightness scroll-notch shifts HSL.l by this amount.
const LIGHT_STEP = 0.04;

const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const { applyBaseRGB } = useHarmonyApply();
const root = ref<HTMLElement | null>(null);

/**
 * In hue-rotation modes only slot 0 carries a handle — dragging it
 * rotates the whole palette per Kuler convention. In Custom mode
 * every unlocked slot gets its own handle. Analogous / Monochromatic
 * / Shades are count-driven (no anchor table) — treated like
 * single-handle modes since dragging non-base slots independently
 * would break the harmony invariant.
 */
const handleSlots = computed<number[]>(() => {
  if (harmony.type === 'custom') {
    return workspace.colors.map((_, i) => i);
  }
  return [0];
});

interface HandleView {
  slot: number;
  hex: string;
  cx: number;
  cy: number;
  locked: boolean;
}

/**
 * Compute pixel position for a slot's handle. Angle = HSL hue
 * (degrees, 0° at +x going clockwise). Radius = rInner + s * span
 * so the handle rides on the ring's outer edge at saturation 1 and
 * on the inner edge at saturation 0.
 */
function handlePosition(hex: string): { cx: number; cy: number } {
  const hsl = toHSL(fromHex(hex));
  const span = R_OUTER - R_INNER;
  const r = R_INNER + hsl.s * span;
  // Match the gesture composable's pointerAngle: 0° at +x, clockwise.
  // SVG Y goes down, so positive angle means cos(+x), -sin(+y).
  const rad = (hsl.h * Math.PI) / 180;
  return {
    cx: CENTER + r * Math.cos(rad),
    cy: CENTER - r * Math.sin(rad),
  };
}

const handles = computed<HandleView[]>(() =>
  handleSlots.value.map((slot) => {
    const c = workspace.colors[slot];
    const hex = c?.hex ?? '#888888';
    const locked = c?.locked ?? false;
    return { slot, hex, locked, ...handlePosition(hex) };
  }),
);

function geometryFor(): WheelGeometry {
  const el = root.value;
  if (!el) {
    return { cx: 0, cy: 0, rOuter: R_OUTER, rInner: R_INNER };
  }
  // The wheel SVG is 320 CSS-px regardless of zoom; getBoundingClientRect
  // gives us the on-screen center in viewport coords, which is what the
  // gesture composable compares pointer events against.
  const rect = el.getBoundingClientRect();
  return {
    cx: rect.left + rect.width / 2,
    cy: rect.top + rect.height / 2,
    rOuter: (R_OUTER / SIZE) * rect.width,
    rInner: (R_INNER / SIZE) * rect.width,
  };
}

function startHSL(slot: number) {
  const c = workspace.colors[slot];
  if (!c) return { h: 0, s: 0, l: 0 };
  return toHSL(fromHex(c.hex));
}

function applyDrag(
  slot: number,
  hue: number,
  sat: number,
  lightness: number,
): void {
  const rgb = fromHSL(hue, sat, lightness);
  if (slot === 0) {
    applyBaseRGB(rgb);
    return;
  }
  workspace.setHex(slot, toHex(rgb));
}

function onDrag(
  slot: number,
  d: { hue: number; saturation: number },
): void {
  const start = startHSL(slot);
  applyDrag(slot, d.hue, d.saturation, start.l);
}

function onLightnessStep(slot: number, steps: number): void {
  const c = workspace.colors[slot];
  if (!c) return;
  const hsl = toHSL(fromHex(c.hex));
  let l = hsl.l + steps * LIGHT_STEP;
  if (l < 0) l = 0;
  if (l > 1) l = 1;
  applyDrag(slot, hsl.h, hsl.s, l);
}

const {
  onPointerDown,
  onPointerMove,
  onPointerUp,
  onWheel,
  isDragging,
} = useHandleGesture({
  getGeometry: geometryFor,
  isLocked: (slot) => workspace.colors[slot]?.locked ?? false,
  getStartHSL: startHSL,
  onDrag,
  onLightnessStep,
  history: workspace.historyAdapter,
});

/**
 * Snap to a hex on a slot's handle without a gesture. Exposed for
 * the HarmonySelector to drive the wheel after a mode change, and
 * for keyboard nudges in a later slice. One undo entry per call.
 */
defineExpose({
  snap(slot: number, hex: string) {
    workspace.setHex(slot, hex);
  },
});

// Optional: clicking inside the inner core resets slot 0 to neutral
// gray — handy for "I dragged off the wheel by accident" recovery.
// Documented gesture in slice plan §5; kept here even though it's a
// tiny UX nicety so the wheel feels finished, not stub-ish.
function onCoreClick() {
  if (workspace.colors[0]?.locked) return;
  const cur = workspace.colors[0];
  if (!cur) return;
  const hsl = toHSL(fromHex(cur.hex));
  applyDrag(0, hsl.h, 0, hsl.l);
}

// Touch tap-and-hold fallback (lightness rail) is deferred to S5b
// per the slice plan — mouse + trackpad covers desktop fully, and
// the rail needs design polish best done together with the picker.
</script>

<template>
  <div ref="root" class="wheel" :style="{ width: SIZE + 'px', height: SIZE + 'px' }">
    <div class="ring" />
    <div class="core" @click="onCoreClick" />
    <svg class="overlay" :width="SIZE" :height="SIZE" :viewBox="`0 0 ${SIZE} ${SIZE}`">
      <line
        v-for="h in handles"
        :key="`l-${h.slot}`"
        :x1="CENTER"
        :y1="CENTER"
        :x2="h.cx"
        :y2="h.cy"
        class="spoke"
      />
    </svg>
    <button
      v-for="h in handles"
      :key="h.slot"
      type="button"
      class="node"
      :class="{ locked: h.locked, primary: h.slot === 0, dragging: isDragging }"
      :style="{
        left: h.cx + 'px',
        top: h.cy + 'px',
        background: h.hex,
      }"
      :aria-label="`slot ${h.slot} ${h.hex}`"
      @pointerdown="onPointerDown($event, h.slot)"
      @pointermove="onPointerMove($event)"
      @pointerup="onPointerUp($event)"
      @pointercancel="onPointerUp($event)"
      @wheel.prevent="onWheel($event, h.slot)"
    />
  </div>
</template>

<style scoped>
.wheel {
  position: relative;
  margin: 6px auto 0;
}

.ring {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: conic-gradient(
    hsl(0 80% 55%),
    hsl(30 85% 58%),
    hsl(60 85% 60%),
    hsl(90 70% 55%),
    hsl(120 65% 50%),
    hsl(150 60% 50%),
    hsl(180 60% 50%),
    hsl(210 65% 55%),
    hsl(240 70% 60%),
    hsl(270 65% 58%),
    hsl(300 70% 58%),
    hsl(330 75% 58%),
    hsl(360 80% 55%)
  );
  -webkit-mask: radial-gradient(circle, transparent 100px, #000 102px, #000 158px, transparent 160px);
  mask: radial-gradient(circle, transparent 100px, #000 102px, #000 158px, transparent 160px);
}

.core {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 200px;
  height: 200px;
  border-radius: 50%;
  background: radial-gradient(
    circle at 50% 50%,
    oklch(0.30 0.01 50) 0,
    oklch(0.22 0.008 50) 70%
  );
  border: 1px solid var(--line-soft);
  cursor: pointer;
}

.overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.spoke {
  stroke: oklch(1 0 0 / 0.18);
  stroke-width: 1;
  stroke-dasharray: 2 3;
}

.node {
  position: absolute;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  transform: translate(-50%, -50%);
  border: 0;
  padding: 0;
  cursor: grab;
  box-shadow:
    0 0 0 2px oklch(0.18 0.008 50),
    0 0 0 3px oklch(1 0 0 / 0.4);
  touch-action: none;
}

.node.primary {
  width: 26px;
  height: 26px;
  box-shadow:
    0 0 0 2px oklch(0.18 0.008 50),
    0 0 0 4px var(--accent-line);
}

.node.locked {
  cursor: not-allowed;
  box-shadow:
    0 0 0 2px oklch(0.18 0.008 50),
    0 0 0 3px var(--accent),
    inset 0 0 0 2px oklch(0 0 0 / 0.35);
}

.node.dragging {
  cursor: grabbing;
}

.node:hover:not(.locked) {
  box-shadow:
    0 0 0 2px oklch(0.18 0.008 50),
    0 0 0 4px oklch(1 0 0 / 0.65);
}
</style>
