<script setup lang="ts">
import { computed, ref } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import {
  fromHSV,
  toHex,
  rybHueToRgbHue,
  rgbHueToRybHue,
} from '../composables/useColor';
import { useHarmonyApply } from '../composables/useHarmonyApply';
import {
  useHandleGesture,
  type WheelGeometry,
} from '../composables/useHandleGesture';

// Geometry: SVG/disc is fixed 320×320 CSS-px. The disc is an RYB
// artist-wheel surface — angle = RYB hue, radius = saturation (0 at
// centre, 1 at the rim). R_OUTER is the saturation-1 radius; the disc
// div fills the 320px box (radius 160), so handles sit 2px inside the rim.
const SIZE = 320;
const CENTER = SIZE / 2;
const R_OUTER = 158;

// One value scroll-notch shifts HSV.v by this amount.
const VALUE_STEP = 0.04;

/**
 * Build the disc background from the RYB hue table — 25 stops every
 * 15°. conic `from 90deg` + reversed stop order puts RYB hue 0 (red)
 * at 3 o'clock growing counter-clockwise, matching handlePosition.
 */
function buildDiscGradient(): string {
  const stops: string[] = [];
  for (let k = 0; k <= 24; k++) {
    // -15·k walks screen angles 0, 345, 330, …, 15, 0; each stop shows
    // the RGB hue the RYB wheel maps that artist angle to.
    stops.push(`hsl(${rybHueToRgbHue(-15 * k)} 100% 50%)`);
  }
  return (
    'radial-gradient(circle closest-side at 50% 50%, ' +
    'hsl(0 0% 50%), hsl(0 0% 50% / 0)), ' +
    `conic-gradient(from 90deg, ${stops.join(', ')})`
  );
}

// Generated once — the RYB table is constant, so the disc never changes.
const DISC_GRADIENT = buildDiscGradient();

const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const { applyBaseRGB } = useHarmonyApply();
const root = ref<HTMLElement | null>(null);

// Slot whose handle is under the pointer — drives paint order only.
const hoveredSlot = ref<number | null>(null);

interface HandleView {
  slot: number;
  hex: string;
  cx: number;
  cy: number;
  locked: boolean;
}

/**
 * Pixel position for a slot's handle on the RYB disc. Angle = the
 * colour's RYB artist-wheel hue — its HSV hue mapped through
 * `rgbHueToRybHue` — (0° at +x, growing counter-clockwise); radius =
 * saturation × R_OUTER. Inverse of useHandleGesture's pointer→artist
 * angle, so a handle reads back where it was placed.
 *
 * Hue/sat come from `effectiveHSV`, not the raw hex: a colour at an
 * achromatic extreme (black / grey) has no recoverable hue, but the
 * intent does — so the handle stays put instead of snapping to centre.
 */
function handlePosition(slot: number): { cx: number; cy: number } {
  const hsv = workspace.effectiveHSV(slot);
  const r = hsv.s * R_OUTER;
  // The disc angle is the RYB artist-wheel hue, not the HSV hue.
  const rad = (rgbHueToRybHue(hsv.h) * Math.PI) / 180;
  return {
    cx: CENTER + r * Math.cos(rad),
    cy: CENTER - r * Math.sin(rad),
  };
}

/**
 * Every slot carries a draggable handle, in every harmony mode.
 * Dragging slot 0 propagates through the active harmony; dragging any
 * other slot is an individual edit that flips the palette to 'custom'
 * (see applyDrag). Unconditional on purpose — it must not read
 * harmony.type, or the flip would re-key this list mid-gesture.
 */
const handles = computed<HandleView[]>(() =>
  workspace.colors.map((c, slot) => {
    const hex = c?.hex ?? '#888888';
    return { slot, hex, locked: c?.locked ?? false, ...handlePosition(slot) };
  }),
);

/**
 * Same handles, reordered so the selected one — then the hovered one —
 * paint last (DOM order = stacking order here). Overlapping handles,
 * e.g. several near-grey slots bunched at the disc centre, stay
 * individually grabbable. Array.sort is stable, so equal-rank
 * handles keep their slot order.
 */
const orderedHandles = computed<HandleView[]>(() => {
  const rank = (slot: number): number => {
    if (slot === hoveredSlot.value) return 2;
    if (slot === workspace.selectedSlot) return 1;
    return 0;
  };
  return handles.value.slice().sort((a, b) => rank(a.slot) - rank(b.slot));
});

function geometryFor(): WheelGeometry {
  const el = root.value;
  if (!el) {
    return { cx: 0, cy: 0, rOuter: R_OUTER };
  }
  // The wheel is designed at SIZE CSS-px but may render smaller in a
  // narrow column; getBoundingClientRect gives the on-screen center and
  // width, which is what the gesture composable compares pointer events
  // against.
  const rect = el.getBoundingClientRect();
  return {
    cx: rect.left + rect.width / 2,
    cy: rect.top + rect.height / 2,
    rOuter: (R_OUTER / SIZE) * rect.width,
  };
}

function startHSV(slot: number) {
  // effectiveHSV preserves hue/sat through achromatic extremes, so a
  // gesture starting on such a handle begins from the intended angle.
  return workspace.effectiveHSV(slot);
}

function applyDrag(
  slot: number,
  hue: number,
  sat: number,
  value: number,
): void {
  const rgb = fromHSV(hue, sat, value);
  // Record the HSV intent before the hex write — effectiveHSV must
  // never see a fresh hex against a stale intent (a one-frame flicker).
  workspace.recordSlotHsv(slot, { h: hue, s: sat, v: value });
  // The base handle is at `harmony.baseIndex` (slot 0 for hue-rotation
  // harmonies, the centre for Analogous / Monochromatic — not always 0).
  // Dragging it propagates the harmony; dragging any other handle is an
  // individual edit that ends the harmony (→ 'custom'), the same
  // contract library and image loads follow.
  const isBase = slot === harmony.baseIndex;
  // A non-base drag normally ends the harmony (→ 'custom'); the
  // Independent-S/V toggle suppresses that, keeping the harmony's hues
  // while this slot's saturation/value is tweaked in place.
  if (!isBase && harmony.type !== 'custom' && !harmony.independentSV) {
    harmony.setType('custom');
  }
  if (isBase) {
    applyBaseRGB(rgb);
    return;
  }
  workspace.setHex(slot, toHex(rgb));
}

function onDrag(
  slot: number,
  d: { hue: number; saturation: number },
): void {
  const start = startHSV(slot);
  // With "Independent S/V" on, a non-base handle is hue-locked to the
  // harmony — the drag tweaks saturation (radius) only, hue held.
  if (
    harmony.independentSV &&
    harmony.type !== 'custom' &&
    slot !== harmony.baseIndex
  ) {
    applyDrag(slot, start.h, d.saturation, start.v);
    return;
  }
  // d.hue is an RYB artist-wheel angle; map it back to an HSV hue
  // before applying.
  applyDrag(slot, rybHueToRgbHue(d.hue), d.saturation, start.v);
}

function onValueStep(slot: number, steps: number): void {
  if (!workspace.colors[slot]) return;
  // effectiveHSV so a scroll on a colour already at an extreme keeps
  // its hue/sat, consistent with the per-color sliders.
  const hsv = workspace.effectiveHSV(slot);
  let v = hsv.v + steps * VALUE_STEP;
  if (v < 0) v = 0;
  if (v > 1) v = 1;
  applyDrag(slot, hsv.h, hsv.s, v);
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
  getStartHSV: startHSV,
  onDrag,
  onValueStep,
  history: workspace.historyAdapter,
});

// Pressing a handle also selects its slot — so the wheel, palette
// strip, and (later) per-color controls all act on the same color.
// Locked slots still select (you may want to inspect one on top) even
// though the gesture itself short-circuits.
function onHandlePointerDown(e: PointerEvent, slot: number): void {
  workspace.selectSlot(slot);
  onPointerDown(e, slot);
}

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
</script>

<template>
  <div ref="root" class="wheel">
    <div class="disc" :style="{ background: DISC_GRADIENT }" />
    <svg class="overlay" width="100%" height="100%" :viewBox="`0 0 ${SIZE} ${SIZE}`">
      <line
        v-for="h in orderedHandles"
        :key="`l-${h.slot}`"
        :x1="CENTER"
        :y1="CENTER"
        :x2="h.cx"
        :y2="h.cy"
        class="spoke"
      />
    </svg>
    <button
      v-for="h in orderedHandles"
      :key="h.slot"
      type="button"
      class="node"
      :class="{
        locked: h.locked,
        primary: harmony.type !== 'custom' && h.slot === harmony.baseIndex,
        selected: h.slot === workspace.selectedSlot,
        dragging: isDragging,
      }"
      :style="{
        left: (h.cx / SIZE) * 100 + '%',
        top: (h.cy / SIZE) * 100 + '%',
        background: h.hex,
      }"
      :aria-label="`slot ${h.slot} ${h.hex}`"
      @pointerdown="onHandlePointerDown($event, h.slot)"
      @pointermove="onPointerMove($event)"
      @pointerup="onPointerUp($event)"
      @pointercancel="onPointerUp($event)"
      @pointerenter="hoveredSlot = h.slot"
      @pointerleave="hoveredSlot = null"
      @wheel.prevent="onWheel($event, h.slot)"
    />
  </div>
</template>

<style scoped>
/* Design size is SIZE px, but the wheel must stay a circle when its
   column is narrower than that: width follows the container, height
   follows width via aspect-ratio. Handles are placed in percentages and
   the SVG scales through its viewBox, so the geometry stays consistent
   at any rendered size (getGeometry derives rOuter from the live rect). */
.wheel {
  position: relative;
  width: min(100%, 320px);
  aspect-ratio: 1 / 1;
  margin: 6px auto 0;
}

/* RYB artist-wheel disc. The background is built in script
   (DISC_GRADIENT) from the RYB hue table and bound inline — `from
   90deg` + reversed stops put RYB hue 0 (red) at 3 o'clock growing
   counter-clockwise, matching handlePosition. The radial-gradient
   fades a mid-grey from the centre out: centre desaturated, rim
   full-chroma. */
.disc {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  pointer-events: none;
  box-shadow: inset 0 0 0 1px var(--line-soft);
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
    0 0 0 4px var(--accent);
}

/* Base-handle marker — a centre dot, white with a dark ring so it reads
   on any swatch colour. Makes the harmony anchor unmistakable. */
.node.primary::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  width: 7px;
  height: 7px;
  transform: translate(-50%, -50%);
  border-radius: 50%;
  background: oklch(1 0 0 / 0.95);
  box-shadow: 0 0 0 1.5px oklch(0 0 0 / 0.5);
}

/* Selection is an outline — a separate render layer from the box-shadow
   rings, so it composes cleanly on top of .primary / .locked. */
.node.selected {
  outline: 2px solid var(--fg-0);
  outline-offset: 4px;
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
