<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import {
  clamp01,
  fromHex,
  fromHSV,
  toHex,
  toHSV,
} from '../composables/useColor';
import {
  COLOR_FORMATS,
  FORMAT_FIELDS,
  colorToFields,
  fieldsToColor,
  type ColorFormat,
} from '../composables/colorIO';
import { useHarmonyApply } from '../composables/useHarmonyApply';

// Geometry — matches the design source's .picker-wheel (sat square
// fits a 232×232 socket). Kept square so we can use integer pixels
// for the knob math; the hue bar lives below.
const SV_SIZE = 220;
const HUE_HEIGHT = 14;

// HSV triangle geometry. All coords are logical/CSS px inside an
// SV_SIZE box; the apex is pure hue, bottom-left white, bottom-right
// black. A point's barycentric weights map to HSV as:
//   wHue = S·V   wWhite = (1−S)·V   wBlack = 1−V      (they sum to 1)
const TRI_H = (SV_SIZE * Math.sqrt(3)) / 2;
const TRI_TOP = (SV_SIZE - TRI_H) / 2;
const TRI_BOT = TRI_TOP + TRI_H;
const C_HUE = { x: SV_SIZE / 2, y: TRI_TOP };
const C_WHITE = { x: 0, y: TRI_BOT };
const C_BLACK = { x: SV_SIZE, y: TRI_BOT };

// Barycentric solve constants. The corners are fixed, so the two edge
// vectors out of C_HUE and the Cramer denominator are computed once.
const TV0X = C_WHITE.x - C_HUE.x;
const TV0Y = C_WHITE.y - C_HUE.y;
const TV1X = C_BLACK.x - C_HUE.x;
const TV1Y = C_BLACK.y - C_HUE.y;
const TD00 = TV0X * TV0X + TV0Y * TV0Y;
const TD01 = TV0X * TV1X + TV0Y * TV1Y;
const TD11 = TV1X * TV1X + TV1Y * TV1Y;
const TDENOM = TD00 * TD11 - TD01 * TD01;

const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const { applyBaseHex } = useHarmonyApply();

/** Picker geometry — SV square (CSS gradients) or HSV triangle (canvas). */
const mode = ref<'square' | 'triangle'>('square');

/**
 * Picker state, kept as HSV. It tracks `editColor` — the harmony base
 * color, or in `custom` mode the slot selected on the wheel. Held
 * locally rather than recomputed from `editColor` on every paint so a
 * small mid-drag rounding (HSV→hex lossy on grayscale) doesn't jitter
 * the knob.
 */
const local = ref<{ h: number; s: number; v: number }>({ h: 0, s: 0, v: 1 });

// Drag state — declared before the watch so the immediate-fire path
// can safely read `dragTarget`. Module-top-level `let` values are
// hoisted but only readable after the let statement executes.
let dragTarget: 'sv' | 'hue' | null = null;
let dragEl: HTMLElement | null = null;
let dragPointerId: number | null = null;

/**
 * Which palette slot the picker edits:
 *   - a harmony mode → the harmony base (an edit regenerates the palette);
 *   - `custom` mode  → the slot selected on the wheel, a per-slot edit
 *     with no harmony to anchor.
 */
const editIndex = computed(() =>
  harmony.type === 'custom' ? workspace.selectedSlot : harmony.baseIndex,
);

/** Hex the picker currently shows and edits. */
const editColor = computed(() =>
  harmony.type === 'custom'
    ? workspace.colors[workspace.selectedSlot]?.hex ?? '#000000'
    : harmony.baseColor,
);

/** True when the edited slot is locked — gestures and typed input no-op. */
const editLocked = computed(
  () => workspace.colors[editIndex.value]?.locked ?? false,
);

/** Route a hex through the right apply path for the active mode. */
function applyEdit(hex: string): void {
  if (harmony.type === 'custom') {
    workspace.setHex(workspace.selectedSlot, hex);
  } else {
    applyBaseHex(hex);
  }
}

watch(
  editColor,
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

/** Pixel offset of the SV knob — branches on the active geometry. */
const knobStyle = computed(() => {
  if (mode.value === 'triangle') {
    const p = svToTrianglePixel(local.value.s, local.value.v);
    return { left: `${p.x}px`, top: `${p.y}px` };
  }
  return {
    left: `${local.value.s * SV_SIZE}px`,
    top: `${(1 - local.value.v) * SV_SIZE}px`,
  };
});

const hueKnobStyle = computed(() => ({
  left: `${(local.value.h / 360) * SV_SIZE}px`,
}));

// --- triangle geometry helpers --------------------------------------------

/** Barycentric weights of a logical-space point against the 3 corners. */
function baryOf(px: number, py: number): {
  wHue: number;
  wWhite: number;
  wBlack: number;
} {
  const v2x = px - C_HUE.x;
  const v2y = py - C_HUE.y;
  const d20 = v2x * TV0X + v2y * TV0Y;
  const d21 = v2x * TV1X + v2y * TV1Y;
  const wWhite = (TD11 * d20 - TD01 * d21) / TDENOM;
  const wBlack = (TD00 * d21 - TD01 * d20) / TDENOM;
  return { wHue: 1 - wWhite - wBlack, wWhite, wBlack };
}

/** HSV → triangle pixel (the inverse of `baryOf`'s HSV mapping). */
function svToTrianglePixel(s: number, v: number): { x: number; y: number } {
  const wHue = s * v;
  const wWhite = (1 - s) * v;
  const wBlack = 1 - v;
  return {
    x: wHue * C_HUE.x + wWhite * C_WHITE.x + wBlack * C_BLACK.x,
    y: wHue * C_HUE.y + wWhite * C_WHITE.y + wBlack * C_BLACK.y,
  };
}

/** Closest point on segment A→B to P (clamped to the segment). */
function closestOnSeg(
  px: number, py: number,
  ax: number, ay: number,
  bx: number, by: number,
): { x: number; y: number } {
  const dx = bx - ax;
  const dy = by - ay;
  const len2 = dx * dx + dy * dy;
  let t = len2 ? ((px - ax) * dx + (py - ay) * dy) / len2 : 0;
  t = t < 0 ? 0 : t > 1 ? 1 : t;
  return { x: ax + t * dx, y: ay + t * dy };
}

// --- pointer → S/V --------------------------------------------------------

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

function svFromTrianglePointer(e: PointerEvent, el: HTMLElement): { s: number; v: number } {
  const r = el.getBoundingClientRect();
  let px = ((e.clientX - r.left) / r.width) * SV_SIZE;
  let py = ((e.clientY - r.top) / r.height) * SV_SIZE;
  let b = baryOf(px, py);
  if (b.wHue < 0 || b.wWhite < 0 || b.wBlack < 0) {
    // Outside the triangle — snap to the nearest point on its border.
    const cands = [
      closestOnSeg(px, py, C_HUE.x, C_HUE.y, C_WHITE.x, C_WHITE.y),
      closestOnSeg(px, py, C_WHITE.x, C_WHITE.y, C_BLACK.x, C_BLACK.y),
      closestOnSeg(px, py, C_BLACK.x, C_BLACK.y, C_HUE.x, C_HUE.y),
    ];
    let best = Infinity;
    for (const c of cands) {
      const d = (c.x - px) ** 2 + (c.y - py) ** 2;
      if (d < best) {
        best = d;
        px = c.x;
        py = c.y;
      }
    }
    b = baryOf(px, py);
  }
  const v = clamp01(b.wHue + b.wWhite);
  const s = v > 0 ? clamp01(b.wHue / v) : 0;
  return { s, v };
}

/** Pointer → S/V for the active geometry. */
function svFromPointer(e: PointerEvent, el: HTMLElement): { s: number; v: number } {
  if (mode.value === 'triangle') return svFromTrianglePointer(e, el);
  const { x, y } = pointerToSVLocal(e, el);
  return { s: x / SV_SIZE, v: 1 - y / SV_SIZE };
}

function pointerToHueLocal(e: PointerEvent, el: HTMLElement): number {
  const r = el.getBoundingClientRect();
  let x = ((e.clientX - r.left) / r.width) * SV_SIZE;
  if (x < 0) x = 0;
  if (x > SV_SIZE) x = SV_SIZE;
  return (x / SV_SIZE) * 360;
}

// --- triangle canvas ------------------------------------------------------

const triCanvas = ref<HTMLCanvasElement | null>(null);

/**
 * Paint the HSV triangle for the current hue. The backing buffer is
 * sized to device pixels (DPR) so the triangle stays as crisp as the
 * CSS-gradient square it shares the picker with; `baryOf` runs in
 * logical coords, hence the `/dpr` on the loop indices.
 */
function renderTriangle(): void {
  const cv = triCanvas.value;
  if (!cv) return;
  const ctx = cv.getContext('2d');
  if (!ctx) return;
  const dpr = window.devicePixelRatio || 1;
  const dim = Math.round(SV_SIZE * dpr);
  if (cv.width !== dim || cv.height !== dim) {
    cv.width = dim;
    cv.height = dim;
  }
  const img = ctx.createImageData(dim, dim);
  const data = img.data;
  const h = local.value.h;
  for (let y = 0; y < dim; y++) {
    const ly = (y + 0.5) / dpr;
    for (let x = 0; x < dim; x++) {
      const b = baryOf((x + 0.5) / dpr, ly);
      if (b.wHue >= 0 && b.wWhite >= 0 && b.wBlack >= 0) {
        const v = b.wHue + b.wWhite;
        const s = v > 0 ? b.wHue / v : 0;
        const rgb = fromHSV(h, s, v);
        const idx = (y * dim + x) * 4;
        data[idx] = rgb.r;
        data[idx + 1] = rgb.g;
        data[idx + 2] = rgb.b;
        data[idx + 3] = 255;
      }
      // else: leave the pixel transparent (alpha defaults to 0).
    }
  }
  ctx.putImageData(img, 0, 0);
}

// Repaint when the hue moves or the triangle mode becomes visible. The
// `nextTick` covers the mode switch — the canvas is `v-if`'d in and is
// not in the DOM until the next tick.
watch([mode, () => local.value.h], () => {
  if (mode.value === 'triangle') void nextTick(renderTriangle);
});
onMounted(() => {
  if (mode.value === 'triangle') renderTriangle();
});

// --- gestures -------------------------------------------------------------

function commit(): void {
  applyEdit(toHex(fromHSV(local.value.h, local.value.s, local.value.v)));
}

function onSVDown(e: PointerEvent): void {
  if (editLocked.value) return;
  const el = e.currentTarget as HTMLElement;
  dragTarget = 'sv';
  dragEl = el;
  dragPointerId = e.pointerId;
  workspace.historyAdapter.pause();
  el.setPointerCapture(e.pointerId);
  local.value = { ...local.value, ...svFromPointer(e, el) };
  commit();
  e.preventDefault();
}

function onHueDown(e: PointerEvent): void {
  if (editLocked.value) return;
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
    local.value = { ...local.value, ...svFromPointer(e, dragEl) };
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

// --- typed color input ----------------------------------------------

const inputFormat = ref<ColorFormat>('hex');
// One string per editable field: length 1 for HEX, length 3 otherwise.
const inputFields = ref<string[]>([]);
const inputError = ref<string | null>(null);
const inputFocused = ref(false);

/** The base color the picker currently holds, as the chosen format's fields. */
const liveRGB = computed(() => fromHSV(local.value.h, local.value.s, local.value.v));
const liveFields = computed(() => colorToFields(inputFormat.value, liveRGB.value));

// Mirror the live color in the fields while they are not being edited —
// drags, format switches, and Count changes all flow through here.
watch(
  liveFields,
  (f) => {
    if (!inputFocused.value) inputFields.value = [...f];
  },
  { immediate: true },
);

// A burst of live edits (typing, holding an arrow key, spinning the
// wheel) collapses into a single undo entry: pause the history on the
// first change, resume once edits go idle for BATCH_IDLE_MS.
const BATCH_IDLE_MS = 500;
let batchTimer: ReturnType<typeof setTimeout> | null = null;
let batching = false;

function beginBatch(): void {
  if (batching) return;
  batching = true;
  workspace.historyAdapter.pause();
}

function commitBatch(): void {
  if (batchTimer !== null) {
    clearTimeout(batchTimer);
    batchTimer = null;
  }
  if (!batching) return;
  batching = false;
  workspace.historyAdapter.resume(true);
}

function scheduleCommit(): void {
  if (batchTimer !== null) clearTimeout(batchTimer);
  batchTimer = setTimeout(commitBatch, BATCH_IDLE_MS);
}

onBeforeUnmount(commitBatch);

/**
 * Apply the current component fields immediately (no Enter needed),
 * batched into one undo entry. Drives live typing and wheel / arrow
 * nudges. On a parse failure it shows the inline error and writes
 * nothing. Deliberately does *not* reformat the fields — the user is
 * mid-edit; a typed `0.51` must not snap to `0.5098` under the caret.
 * Canonical values are restored on blur instead.
 */
function applyLive(): void {
  let rgb;
  try {
    rgb = fieldsToColor(inputFormat.value, inputFields.value);
  } catch (e) {
    inputError.value = e instanceof Error ? e.message : 'invalid color';
    return;
  }
  inputError.value = null;
  beginBatch();
  applyEdit(toHex(rgb));
  scheduleCommit();
}

/** Apply the HEX field — one token, committed on Enter (one undo step). */
function applyHex(): void {
  let rgb;
  try {
    rgb = fieldsToColor('hex', inputFields.value);
  } catch (e) {
    inputError.value = e instanceof Error ? e.message : 'invalid color';
    return;
  }
  inputError.value = null;
  applyEdit(toHex(rgb));
  inputFields.value = colorToFields('hex', rgb);
}

/**
 * Nudge component field `i` by `steps` × its step, clamped to the
 * field's [min, max] bounds, then apply live. The clamp is what stops a
 * scroll or held arrow key from running a value past its range.
 */
function nudge(i: number, steps: number): void {
  if (editLocked.value) return;
  const field = FORMAT_FIELDS[inputFormat.value][i];
  if (!field) return;
  const cur = Number((inputFields.value[i] ?? '').trim());
  let next = (Number.isFinite(cur) ? cur : 0) + steps * field.step;
  if (next < field.min) next = field.min;
  if (next > field.max) next = field.max;
  // toFixed(6) clears binary-float dust (0.01 + 0.02 → 0.030000000004).
  inputFields.value[i] = String(Number(next.toFixed(6)));
  applyLive();
}

/**
 * Drive component field `i` from its range slider. The slider element
 * keeps the value within [min, max]; we mirror its value into the field
 * string and apply live — batched into one undo entry like typing.
 */
function onSlider(i: number, e: Event): void {
  if (editLocked.value) return;
  inputFields.value[i] = (e.target as HTMLInputElement).value;
  applyLive();
}

function onWheel(e: WheelEvent, i: number): void {
  // Focus the field so the live-edit guard holds — the `liveFields`
  // watch must not snap the value back while the wheel is driving it.
  (e.currentTarget as HTMLElement).focus();
  nudge(i, (e.deltaY < 0 ? 1 : -1) * (e.shiftKey ? 10 : 1));
}

function onArrowKey(e: KeyboardEvent, i: number): void {
  const dir = e.key === 'ArrowUp' ? 1 : e.key === 'ArrowDown' ? -1 : 0;
  if (dir === 0) return;
  e.preventDefault(); // don't also move the text caret
  nudge(i, dir * (e.shiftKey ? 10 : 1));
}

function onFormatChange(): void {
  // Reformatting on a dropdown change is intentional even mid-edit:
  // picking a format is a "show me the live color this way" action.
  inputFields.value = [...liveFields.value];
  inputError.value = null;
}

function onGroupFocusIn(): void {
  inputFocused.value = true;
}

function onGroupFocusOut(e: FocusEvent): void {
  // Tabbing between this format's own component fields is not a group
  // blur — only treat focus leaving the whole field row as a blur.
  const wrap = e.currentTarget as HTMLElement;
  if (wrap.contains(e.relatedTarget as Node | null)) return;
  inputFocused.value = false;
  commitBatch(); // finalise the live-edit undo entry now
  // Restore canonical values — the fields mirror the live color when
  // not focused; any real change was already applied live.
  inputFields.value = [...liveFields.value];
  inputError.value = null;
}
</script>

<template>
  <div class="picker">
    <div class="mode-toggle" role="group" aria-label="Picker geometry">
      <button
        type="button"
        class="mt-btn"
        :class="{ active: mode === 'square' }"
        :aria-pressed="mode === 'square'"
        @click="mode = 'square'"
      >
        <svg class="mt-ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <rect x="4" y="4" width="16" height="16" rx="2" />
        </svg>
        Square
      </button>
      <button
        type="button"
        class="mt-btn"
        :class="{ active: mode === 'triangle' }"
        :aria-pressed="mode === 'triangle'"
        @click="mode = 'triangle'"
      >
        <svg class="mt-ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round">
          <path d="M12 4 21 20 3 20 Z" />
        </svg>
        Triangle
      </button>
    </div>

    <div
      v-if="mode === 'square'"
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
      v-else
      class="tri"
      :style="{ width: SV_SIZE + 'px', height: SV_SIZE + 'px' }"
      @pointerdown="onSVDown"
      @pointermove="onMove"
      @pointerup="onUp"
      @pointercancel="onUp"
    >
      <canvas
        ref="triCanvas"
        class="tri-canvas"
        :style="{ width: SV_SIZE + 'px', height: SV_SIZE + 'px' }"
      />
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

    <div class="typed-wrap" :style="{ width: SV_SIZE + 'px' }">
      <select
        v-model="inputFormat"
        class="fmt-select"
        aria-label="Color format"
        @change="onFormatChange"
      >
        <option v-for="f in COLOR_FORMATS" :key="f.value" :value="f.value">
          {{ f.label }}
        </option>
      </select>
      <div class="typed" @focusin="onGroupFocusIn" @focusout="onGroupFocusOut">
        <template v-if="inputFormat === 'hex'">
          <input
            v-model="inputFields[0]"
            class="fmt-input mono"
            type="text"
            spellcheck="false"
            autocomplete="off"
            :disabled="editLocked"
            :aria-invalid="inputError !== null"
            :placeholder="editLocked ? 'color locked' : '#rrggbb · ↵ to apply'"
            @keydown.enter.prevent="applyHex"
          />
        </template>
        <template v-else>
          <div
            v-for="(f, i) in FORMAT_FIELDS[inputFormat]"
            :key="f.label"
            class="comp"
          >
            <span class="comp-lbl">{{ f.label }}</span>
            <input
              v-model="inputFields[i]"
              class="comp-input mono"
              type="text"
              inputmode="decimal"
              spellcheck="false"
              autocomplete="off"
              :disabled="editLocked"
              :aria-invalid="inputError !== null"
              :aria-label="f.label"
              :title="`${f.label} · scroll or ↑↓ to adjust (Shift ×10)`"
              @input="applyLive"
              @wheel.prevent="onWheel($event, i)"
              @keydown="onArrowKey($event, i)"
            />
            <input
              class="comp-slider"
              type="range"
              :min="f.min"
              :max="f.max"
              :step="f.step"
              :value="inputFields[i]"
              :disabled="editLocked"
              :aria-label="`${f.label} slider`"
              @input="onSlider(i, $event)"
            />
          </div>
        </template>
      </div>
      <div v-if="inputError" class="fmt-error mono" role="alert">
        {{ inputError }}
      </div>
    </div>

    <p
      v-if="harmony.type !== 'custom'"
      class="picker-note"
      :style="{ width: SV_SIZE + 'px' }"
    >
      The picker edits the harmony's base color. To edit other palette
      slots individually, switch the harmony to <strong>Custom</strong>.
    </p>
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

.mode-toggle {
  display: inline-flex;
  border: 1px solid var(--line-soft);
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg-2);
}

.mt-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 11px;
  font-size: 11px;
  font-weight: 500;
  font-family: inherit;
  color: var(--fg-2);
  background: transparent;
  border: none;
  cursor: pointer;
}

.mt-btn + .mt-btn {
  border-left: 1px solid var(--line-soft);
}

.mt-btn:hover {
  color: var(--fg-0);
}

.mt-btn.active {
  background: var(--accent-soft);
  color: var(--fg-0);
}

.mt-ic {
  width: 13px;
  height: 13px;
  color: var(--fg-3);
}

.mt-btn.active .mt-ic {
  color: var(--accent);
}

.sv {
  position: relative;
  border-radius: 8px;
  box-shadow: 0 0 0 1px var(--line-soft);
  cursor: crosshair;
  touch-action: none;
}

/* The triangle fills only part of its box, so it carries no square
   frame — the canvas-drawn triangle edge is its own outline. */
.tri {
  position: relative;
  cursor: crosshair;
  touch-action: none;
}

.tri-canvas {
  position: absolute;
  inset: 0;
  display: block;
  pointer-events: none;
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

.typed-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fmt-select,
.fmt-input,
.comp-input {
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  padding: 6px 8px;
  color: var(--fg-0);
  font-family: inherit;
  font-size: 12px;
}

.fmt-select {
  width: 100%;
}

.typed {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.fmt-input {
  width: 100%;
}

/* One stacked row per component: fixed label + number field, slider
   filling the rest. The number field's column is fixed-width so a long
   value (e.g. an OkLab channel) can't shove the slider around. */
.comp {
  display: grid;
  grid-template-columns: 16px 68px 1fr;
  align-items: center;
  gap: 9px;
}

.comp-lbl {
  font-size: 10px;
  color: var(--fg-3);
}

.comp-input {
  width: 100%;
  min-width: 0;
  padding: 6px;
}

.comp-slider {
  width: 100%;
  height: 6px;
  accent-color: var(--accent);
}

.comp-slider:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.fmt-input:focus,
.comp-input:focus {
  outline: none;
  border-color: var(--accent);
}

.fmt-input[aria-invalid='true'],
.comp-input[aria-invalid='true'] {
  border-color: var(--bad);
}

.fmt-input:disabled,
.comp-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.fmt-error {
  font-size: 10.5px;
  color: var(--bad);
}

/* Footnote anchored to the bottom of the card — explains the picker
   only edits the harmony base, and points to Custom for per-slot edits. */
.picker-note {
  margin: 0;
  font-size: 10.5px;
  line-height: 1.45;
  color: var(--fg-3);
  text-align: center;
}

.picker-note strong {
  color: var(--fg-1);
  font-weight: 600;
}
</style>
