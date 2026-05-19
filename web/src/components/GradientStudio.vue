<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';
import { watchDebounced } from '@vueuse/core';
import VChart from 'vue-echarts';
import { gradient } from '../api';
import type { ColorJSON, PaletteEnvelope } from '../api/types';
import type { GradientEasing, GradientSpace } from '../api/gradient';
import { useWorkspaceStore } from '../stores/workspace';
import { fromHex, toHex, toOkLab, type RGB } from '../composables/useColor';
import { useChartTheme } from '../composables/useChartTheme';
import {
  CSS_INTERP,
  gradientToCSS,
  gradientToGGR,
  gradientToJSON,
  gradientToSVG,
  safeIdent,
  type GradientArtifact,
} from '../composables/gradientExport';

/**
 * Multi-stop gradient studio. Each stop carries a 0..1 position; the two
 * endpoints stay pinned at 0 and 1, middle stops drag along the bar.
 * `gradient.MultiStopAt` honours those positions on the backend. The CSS
 * bar is the instant preview — it interpolates via `linear-gradient(in
 * <space> …)`, the same space the backend uses, so the colors agree; it
 * cannot show the easing curve, though (CSS gradients interpolate
 * linearly between stops). The discrete N-step palette comes from
 * /api/v1/gradient (debounced) and is authoritative — it honours both
 * positions and easing and byte-matches the CLI; the ECharts line plots
 * OkLab lightness of those steps — the "is this ramp perceptually even"
 * diagnostic.
 *
 * The stops are seeded from the whole workspace palette, so the card
 * opens with the palette already shown as an editable gradient; the
 * "Extract Gradient" button re-syncs them to the current palette.
 */

const workspace = useWorkspaceStore();
const chartTheme = useChartTheme();

const SPACES: GradientSpace[] = ['oklch', 'oklab', 'lab', 'hsl', 'rgb'];
const EASINGS: GradientEasing[] = ['linear', 'ease-in', 'ease-out', 'ease-in-out'];
const MAX_STEPS = 16;

// css/ggr/svg/json render client-side off the discrete result; png/jpeg
// need the Go image renderer, so they route through POST /export.
type ExportFormat = 'css' | 'ggr' | 'svg' | 'json' | 'png' | 'jpeg';
const EXPORT_FORMATS: { value: ExportFormat; label: string }[] = [
  { value: 'css', label: 'CSS' },
  { value: 'ggr', label: 'GIMP' },
  { value: 'svg', label: 'SVG' },
  { value: 'json', label: 'JSON' },
  { value: 'png', label: 'PNG' },
  { value: 'jpeg', label: 'JPEG' },
];

/**
 * The workspace palette as gradient stops — every color in palette
 * order, capped at MAX_STEPS. A degenerate 0/1-color palette falls back
 * to a dark→light pair so there are always at least 2 stops.
 */
function paletteStops(): string[] {
  const hexes = workspace.colors.map((c) => c.hex.toUpperCase());
  if (hexes.length === 0) return ['#1A1A1A', '#FFFFFF'];
  if (hexes.length === 1) {
    return [hexes[0]!, hexes[0] === '#FFFFFF' ? '#1A1A1A' : '#FFFFFF'];
  }
  return hexes.slice(0, MAX_STEPS);
}

/** n positions evenly spread across [0,1], endpoints pinned to 0 and 1. */
function evenPositions(n: number): number[] {
  if (n < 2) return n === 1 ? [0] : [];
  return Array.from({ length: n }, (_, i) => (i === n - 1 ? 1 : i / (n - 1)));
}

const stops = ref<string[]>(paletteStops());
/** Per-stop 0..1 position, parallel to `stops`; index 0 = 0, last = 1. */
const positions = ref<number[]>(evenPositions(stops.value.length));
const selected = ref(0);
const steps = ref(7);
const space = ref<GradientSpace>('oklch');
const easing = ref<GradientEasing>('linear');

/** Smallest gap kept between adjacent stop positions while dragging. */
const MIN_GAP = 0.01;

// Full /api/v1/gradient envelope-result kept for the JSON export; the
// discrete colors the UI renders are derived from it.
const lastResult = ref<PaletteEnvelope['result'] | null>(null);
const result = computed<ColorJSON[]>(() => lastResult.value?.palette.colors ?? []);
const loading = ref(false);
const errorMsg = ref<string | null>(null);

// MultiStop requires steps >= stop count.
const minSteps = computed(() => stops.value.length);
const effectiveSteps = computed(() => Math.max(steps.value, minSteps.value));

const barGradient = computed(() => {
  const parts = stops.value.map(
    (s, i) => `${s} ${((positions.value[i] ?? 0) * 100).toFixed(2)}%`,
  );
  return `linear-gradient(in ${CSS_INTERP[space.value]} 90deg, ${parts.join(', ')})`;
});

function stopLeft(i: number): string {
  return `${(positions.value[i] ?? 0) * 100}%`;
}

function parse(hex: string): RGB | null {
  try {
    return fromHex(hex);
  } catch {
    return null;
  }
}

// --- stop editing ------------------------------------------------------

const editHex = ref('');

function selectStop(i: number): void {
  selected.value = i;
  editHex.value = stops.value[i] ?? '';
}
selectStop(0);

function commitEdit(): void {
  const rgb = parse(editHex.value);
  if (!rgb) return;
  const hex = toHex(rgb).toUpperCase();
  stops.value = stops.value.map((s, i) => (i === selected.value ? hex : s));
  editHex.value = hex;
}

function setSelectedTo(hex: string): void {
  const norm = hex.toUpperCase();
  stops.value = stops.value.map((s, i) => (i === selected.value ? norm : s));
  editHex.value = norm;
}

function addStop(): void {
  // Drop the new stop into the widest gap so it lands somewhere useful.
  let gapIdx = 0;
  let gapSize = -1;
  for (let i = 0; i < positions.value.length - 1; i++) {
    const g = (positions.value[i + 1] ?? 1) - (positions.value[i] ?? 0);
    if (g > gapSize) {
      gapSize = g;
      gapIdx = i;
    }
  }
  const mid =
    ((positions.value[gapIdx] ?? 0) + (positions.value[gapIdx + 1] ?? 1)) / 2;
  const nextStops = [...stops.value];
  const nextPositions = [...positions.value];
  nextStops.splice(gapIdx + 1, 0, '#888888');
  nextPositions.splice(gapIdx + 1, 0, mid);
  stops.value = nextStops;
  positions.value = nextPositions;
  if (steps.value < nextStops.length) steps.value = nextStops.length;
  selectStop(gapIdx + 1);
}

function removeStop(i: number): void {
  if (stops.value.length <= 2) return;
  stops.value = stops.value.filter((_, idx) => idx !== i);
  const nextPositions = positions.value.filter((_, idx) => idx !== i);
  // Removing an endpoint moves the 0/1 pin onto its surviving neighbour.
  nextPositions[0] = 0;
  nextPositions[nextPositions.length - 1] = 1;
  positions.value = nextPositions;
  selectStop(Math.min(selected.value, stops.value.length - 1));
}

function extractFromWorkspace(): void {
  const next = paletteStops();
  stops.value = next;
  positions.value = evenPositions(next.length);
  if (steps.value < next.length) steps.value = next.length;
  selectStop(0);
}

// --- stop dragging -----------------------------------------------------

const barEl = ref<HTMLElement | null>(null);
// -1 when idle; otherwise the index of the stop being dragged.
let dragIndex = -1;

/** Begin a drag (and select the stop). Endpoints stay pinned, not draggable. */
function startDrag(i: number, e: PointerEvent): void {
  selectStop(i);
  if (i === 0 || i === stops.value.length - 1) return;
  dragIndex = i;
  (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
}

/** Move the dragged stop, clamped between its neighbours. */
function onDrag(e: PointerEvent): void {
  if (dragIndex < 0 || !barEl.value) return;
  const rect = barEl.value.getBoundingClientRect();
  if (rect.width === 0) return;
  const raw = (e.clientX - rect.left) / rect.width;
  const lo = (positions.value[dragIndex - 1] ?? 0) + MIN_GAP;
  const hi = (positions.value[dragIndex + 1] ?? 1) - MIN_GAP;
  const clamped = Math.min(hi, Math.max(lo, raw));
  positions.value = positions.value.map((v, idx) =>
    idx === dragIndex ? clamped : v,
  );
}

function endDrag(): void {
  dragIndex = -1;
}

// --- backend fetch -----------------------------------------------------

async function fetchGradient(): Promise<void> {
  loading.value = true;
  errorMsg.value = null;
  try {
    lastResult.value = await gradient.generate({
      stops: stops.value.join(','),
      positions: positions.value.map((p) => p.toFixed(4)).join(','),
      steps: effectiveSteps.value,
      space: space.value,
      easing: easing.value,
    });
  } catch (e) {
    errorMsg.value = e instanceof Error ? e.message : String(e);
    lastResult.value = null;
  } finally {
    loading.value = false;
  }
}

watchDebounced([stops, positions, steps, space, easing], fetchGradient, {
  debounce: 220,
  deep: true,
  immediate: true,
});

// --- chart -------------------------------------------------------------

const chartOption = computed(() => {
  const t = chartTheme.value;
  const pts = result.value;
  const n = pts.length;
  const data = pts.map((c, i) => {
    const rgb: RGB = { r: c.rgb[0]!, g: c.rgb[1]!, b: c.rgb[2]! };
    const x = n > 1 ? i / (n - 1) : 0;
    return [x, toOkLab(rgb).L];
  });
  return {
    animation: false,
    grid: { left: 36, right: 12, top: 12, bottom: 22 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: t.surface,
      borderColor: t.axisLine,
      textStyle: { color: t.text, fontSize: 11 },
      valueFormatter: (v: number) => v.toFixed(3),
    },
    xAxis: {
      type: 'value',
      min: 0,
      max: 1,
      name: 'position',
      nameTextStyle: { color: t.text, fontSize: 9 },
      nameGap: 16,
      axisLine: { lineStyle: { color: t.axisLine } },
      axisTick: { show: false },
      axisLabel: { color: t.text, fontSize: 9, formatter: (v: number) => v.toFixed(1) },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 1,
      name: 'OkL',
      nameTextStyle: { color: t.text, fontSize: 9 },
      axisLine: { lineStyle: { color: t.axisLine } },
      axisLabel: { color: t.text, fontSize: 9 },
      splitLine: { lineStyle: { color: t.splitLine, type: 'dashed' } },
    },
    series: [
      {
        type: 'line',
        data,
        smooth: true,
        showSymbol: false,
        lineStyle: { color: t.accent, width: 2 },
        areaStyle: { color: t.accent, opacity: 0.12 },
      },
    ],
  };
});

// --- export ------------------------------------------------------------

const exportFormat = ref<ExportFormat>('css');
const exportName = ref('gradient');
const copied = ref(false);

/** Request params echoed into the JSON export envelope. */
const exportParams = computed<Record<string, unknown>>(() => ({
  stops: stops.value.join(','),
  positions: positions.value.map((p) => p.toFixed(4)).join(','),
  steps: effectiveSteps.value,
  space: space.value,
  easing: easing.value,
}));

/**
 * Copy / download artifact for the chosen format. CSS builds from the
 * stops directly; GIMP / SVG / JSON need the discrete result and stay
 * null until the first /api/v1/gradient response lands.
 */
const exportArtifact = computed<GradientArtifact | null>(() => {
  const name = exportName.value;
  switch (exportFormat.value) {
    case 'css':
      return gradientToCSS(stops.value, positions.value, space.value, name);
    case 'ggr':
      return result.value.length > 1 ? gradientToGGR(result.value, name) : null;
    case 'svg':
      return result.value.length > 1 ? gradientToSVG(result.value, name) : null;
    case 'json':
      return lastResult.value
        ? gradientToJSON(lastResult.value, exportParams.value, name)
        : null;
    default:
      return null;
  }
});

// png / jpeg: a true gradient image, rasterised on a <canvas>. The Go
// exporter's PNG/JPEG draws a discrete swatch strip — right for a
// palette, wrong for a gradient — so the binary export is done
// client-side here instead of through POST /export.
const isBinaryExport = computed(
  () => exportFormat.value === 'png' || exportFormat.value === 'jpeg',
);
const binaryUrl = ref<string | null>(null);

const canExport = computed(() =>
  isBinaryExport.value ? binaryUrl.value !== null : exportArtifact.value !== null,
);

// A fine sample count so the rasterised gradient looks smooth and
// honours the chosen space / easing / positions; the canvas tweens
// sRGB between these already-correct samples, imperceptibly at this
// density. The UI's MAX_STEPS cap is a slider concern, not a wire one.
const GRADIENT_IMAGE_STEPS = 256;
const GRADIENT_IMAGE_W = 720;
const GRADIENT_IMAGE_H = 120;

/** Swap in a new binary preview URL, revoking the previous one. */
function setBinaryUrl(blob: Blob | null): void {
  if (binaryUrl.value) URL.revokeObjectURL(binaryUrl.value);
  binaryUrl.value = blob ? URL.createObjectURL(blob) : null;
}

/** Paint `hexes` as a horizontal linear gradient and encode it to `mime`. */
function renderGradientImage(hexes: string[], mime: string): Promise<Blob> {
  const canvas = document.createElement('canvas');
  canvas.width = GRADIENT_IMAGE_W;
  canvas.height = GRADIENT_IMAGE_H;
  const ctx = canvas.getContext('2d');
  if (!ctx) return Promise.reject(new Error('canvas 2d context unavailable'));
  const grad = ctx.createLinearGradient(0, 0, GRADIENT_IMAGE_W, 0);
  hexes.forEach((hex, i) =>
    grad.addColorStop(hexes.length > 1 ? i / (hexes.length - 1) : 0, hex),
  );
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, GRADIENT_IMAGE_W, GRADIENT_IMAGE_H);
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (b) => (b ? resolve(b) : reject(new Error('canvas encode failed'))),
      mime,
      0.95,
    );
  });
}

async function fetchBinaryExport(): Promise<void> {
  if (!isBinaryExport.value || stops.value.length < 2) {
    setBinaryUrl(null);
    return;
  }
  try {
    // Sample the gradient finely — /api/v1/gradient honours space,
    // easing and stop positions; the colors come back already blended.
    const res = await gradient.generate({
      stops: stops.value.join(','),
      positions: positions.value.map((p) => p.toFixed(4)).join(','),
      steps: GRADIENT_IMAGE_STEPS,
      space: space.value,
      easing: easing.value,
    });
    const mime = exportFormat.value === 'png' ? 'image/png' : 'image/jpeg';
    setBinaryUrl(
      await renderGradientImage(res.palette.colors.map((c) => c.hex), mime),
    );
  } catch (e) {
    errorMsg.value = e instanceof Error ? e.message : String(e);
    setBinaryUrl(null);
  }
}

watchDebounced([exportFormat, stops, positions, space, easing], fetchBinaryExport, {
  debounce: 220,
  deep: true,
});
onBeforeUnmount(() => setBinaryUrl(null));

async function copyExport(): Promise<void> {
  const art = exportArtifact.value;
  if (!art) return;
  try {
    await navigator.clipboard.writeText(art.text);
    copied.value = true;
    window.setTimeout(() => {
      copied.value = false;
    }, 1200);
  } catch {
    errorMsg.value = 'clipboard unavailable';
  }
}

function downloadExport(): void {
  if (isBinaryExport.value) {
    if (!binaryUrl.value) return;
    const a = document.createElement('a');
    a.href = binaryUrl.value;
    a.download = `${safeIdent(exportName.value)}.${exportFormat.value === 'png' ? 'png' : 'jpg'}`;
    a.click();
    return;
  }
  const art = exportArtifact.value;
  if (!art) return;
  const blob = new Blob([art.text], { type: `${art.mime};charset=utf-8` });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${safeIdent(exportName.value)}.${art.ext}`;
  a.click();
  URL.revokeObjectURL(url);
}
</script>

<template>
  <div class="gs">
    <div class="head">
      <button type="button" class="btn accent" @click="extractFromWorkspace">
        ⛏ Extract Gradient
      </button>
      <span class="hint mono">{{ effectiveSteps }} steps · {{ space }} · {{ easing }}</span>
    </div>

    <div ref="barEl" class="bar" :style="{ background: barGradient }">
      <button
        v-for="(s, i) in stops"
        :key="i"
        type="button"
        class="grad-stop"
        :class="{ sel: i === selected, fixed: i === 0 || i === stops.length - 1 }"
        :style="{ left: stopLeft(i), background: s }"
        :title="`stop ${i + 1}: ${s} @ ${Math.round((positions[i] ?? 0) * 100)}%`"
        @pointerdown="startDrag(i, $event)"
        @pointermove="onDrag($event)"
        @pointerup="endDrag"
        @lostpointercapture="endDrag"
      />
    </div>

    <div class="stops">
      <button
        v-for="(s, i) in stops"
        :key="i"
        type="button"
        class="chip"
        :class="{ sel: i === selected }"
        @click="selectStop(i)"
      >
        <span class="chip-sw" :style="{ background: s }" />
        <span class="chip-hex mono">{{ s }}</span>
        <span
          v-if="stops.length > 2"
          class="chip-x"
          title="remove stop"
          @click.stop="removeStop(i)"
          >×</span
        >
      </button>
      <button
        type="button"
        class="chip add"
        :disabled="stops.length >= MAX_STEPS"
        title="add stop"
        @click="addStop"
      >
        +
      </button>
    </div>

    <div class="editor">
      <label class="fld">Stop {{ selected + 1 }} color</label>
      <div class="input">
        <span class="swatch" :style="{ background: parse(editHex) ? editHex : 'transparent' }" />
        <input
          v-model="editHex"
          class="mono hex"
          :class="{ bad: !parse(editHex) }"
          spellcheck="false"
          aria-label="selected stop hex"
          @change="commitEdit"
        />
      </div>
      <div class="ws-pick">
        <button
          v-for="(c, i) in workspace.colors"
          :key="i"
          type="button"
          class="ws-sw"
          :style="{ background: c.hex }"
          :title="`use ${c.hex}`"
          @click="setSelectedTo(c.hex)"
        />
      </div>
    </div>

    <div class="controls">
      <div class="ctl">
        <label class="fld">Steps</label>
        <div class="slider-row">
          <input
            v-model.number="steps"
            type="range"
            :min="minSteps"
            :max="MAX_STEPS"
            aria-label="gradient steps"
          />
          <span class="val mono">{{ effectiveSteps }}</span>
        </div>
      </div>
      <div class="ctl">
        <label class="fld">Space</label>
        <select v-model="space" class="select" aria-label="interpolation space">
          <option v-for="sp in SPACES" :key="sp" :value="sp">{{ sp }}</option>
        </select>
      </div>
      <div class="ctl">
        <label class="fld">Easing</label>
        <select v-model="easing" class="select" aria-label="easing">
          <option v-for="e in EASINGS" :key="e" :value="e">{{ e }}</option>
        </select>
      </div>
    </div>

    <div class="chart-wrap">
      <VChart v-if="result.length > 1" class="chart" :option="chartOption" autoresize />
      <div v-else class="empty mono">{{ loading ? 'computing…' : 'no gradient' }}</div>
    </div>

    <p v-if="errorMsg" class="err mono">{{ errorMsg }}</p>

    <div v-if="result.length" class="out">
      <span class="fld">Gradient stops · {{ result.length }}</span>
      <div class="out-row" :style="{ gridTemplateColumns: `repeat(${result.length}, 1fr)` }">
        <div v-for="(c, i) in result" :key="i" class="out-cell">
          <span class="out-sw" :style="{ background: c.hex }" />
          <span class="out-hex mono">{{ c.hex.toUpperCase() }}</span>
        </div>
      </div>
    </div>

    <div class="export">
      <div class="export-head">
        <span class="fld">Export</span>
        <div class="fmt-row" role="group" aria-label="Export format">
          <button
            v-for="f in EXPORT_FORMATS"
            :key="f.value"
            type="button"
            class="fmt-btn"
            :class="{ active: exportFormat === f.value }"
            :aria-pressed="exportFormat === f.value"
            @click="exportFormat = f.value"
          >
            {{ f.label }}
          </button>
        </div>
      </div>
      <input
        v-model="exportName"
        class="export-name mono"
        type="text"
        spellcheck="false"
        autocomplete="off"
        placeholder="gradient"
        aria-label="export name"
      />
      <img
        v-if="isBinaryExport"
        class="export-thumb"
        :class="{ placeholder: !binaryUrl }"
        :src="binaryUrl ?? ''"
        alt="gradient export preview"
      />
      <pre v-else class="export-out mono" :class="{ placeholder: !exportArtifact }">{{
        exportArtifact ? exportArtifact.text : loading ? 'computing…' : 'no gradient'
      }}</pre>
      <div class="export-actions">
        <button
          v-if="!isBinaryExport"
          type="button"
          class="btn"
          :disabled="!canExport"
          @click="copyExport"
        >
          {{ copied ? 'Copied!' : 'Copy' }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="!canExport"
          @click="downloadExport"
        >
          Download
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.gs {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.hint {
  font-size: 10px;
  color: var(--fg-3);
  margin-left: auto;
}

.btn {
  padding: 7px 12px;
  font-size: 11px;
  font-weight: 600;
  color: var(--fg-1);
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 8px;
  cursor: pointer;
}

.btn:hover:not(:disabled) {
  border-color: var(--accent-line);
  color: var(--fg-0);
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.btn.accent {
  background: var(--accent-soft);
  border-color: var(--accent-line);
  color: var(--fg-0);
}

.bar {
  position: relative;
  height: 64px;
  border-radius: 12px;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.06);
}

.grad-stop {
  position: absolute;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 16px;
  height: 16px;
  padding: 0;
  border-radius: 50%;
  border: 2px solid #fff;
  box-shadow: 0 0 0 1px oklch(0 0 0 / 0.5);
  cursor: grab;
  touch-action: none;
}

/* Endpoints stay pinned at 0 / 1 — clickable to select, not draggable. */
.grad-stop.fixed {
  cursor: pointer;
}

.grad-stop:not(.fixed):active {
  cursor: grabbing;
}

.grad-stop.sel {
  width: 20px;
  height: 20px;
  box-shadow:
    0 0 0 1px oklch(0 0 0 / 0.5),
    0 0 0 4px var(--accent-soft);
}

.stops {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 7px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  cursor: pointer;
}

.chip.sel {
  border-color: var(--accent-line);
  background: var(--accent-soft);
}

.chip-sw {
  width: 14px;
  height: 14px;
  border-radius: 4px;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.12);
}

.chip-hex {
  font-size: 10px;
  color: var(--fg-1);
}

.chip-x {
  font-size: 13px;
  line-height: 1;
  color: var(--fg-3);
  padding: 0 2px;
}

.chip-x:hover {
  color: var(--bad);
}

.chip.add {
  font-size: 14px;
  font-weight: 600;
  color: var(--fg-2);
  padding: 4px 10px;
}

.chip.add:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.editor {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fld {
  font-size: 11px;
  color: var(--fg-2);
  font-weight: 500;
}

.input {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 8px;
}

.input .swatch {
  width: 18px;
  height: 18px;
  border-radius: 5px;
  flex: none;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.12);
}

.hex {
  flex: 1 1 auto;
  min-width: 0;
  background: transparent;
  border: 0;
  color: var(--fg-0);
  font-size: 11.5px;
}

.hex:focus {
  outline: none;
}

.hex.bad {
  color: var(--bad);
}

.ws-pick {
  display: flex;
  gap: 4px;
}

.ws-sw {
  width: 100%;
  height: 14px;
  border-radius: 4px;
  border: 1px solid var(--line-soft);
  cursor: pointer;
  padding: 0;
}

.ws-sw:hover {
  border-color: var(--accent-line);
}

.controls {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.ctl {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.slider-row input[type='range'] {
  flex: 1 1 auto;
  min-width: 0;
  accent-color: var(--accent);
}

.val {
  font-size: 11px;
  color: var(--fg-1);
  min-width: 18px;
  text-align: right;
}

.select {
  padding: 5px 8px;
  font-size: 11px;
  color: var(--fg-0);
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  cursor: pointer;
}

.chart-wrap {
  min-height: 150px;
}

.chart {
  height: 150px;
  width: 100%;
}

.empty {
  height: 150px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--line-soft);
  border-radius: var(--r-md);
  color: var(--fg-3);
  font-size: 10.5px;
}

.err {
  font-size: 10.5px;
  color: var(--bad);
}

.out {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.out-row {
  display: grid;
  gap: 4px;
}

.out-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
  align-items: center;
  min-width: 0;
}

.out-sw {
  width: 100%;
  height: 26px;
  border-radius: 5px;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.08);
}

.out-hex {
  font-size: 8.5px;
  color: var(--fg-2);
}

.export {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.export-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.fmt-row {
  display: inline-flex;
  margin-left: auto;
  border: 1px solid var(--line-soft);
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg-2);
}

.fmt-btn {
  padding: 5px 10px;
  font-size: 10.5px;
  font-weight: 500;
  font-family: inherit;
  color: var(--fg-2);
  background: transparent;
  border: none;
  cursor: pointer;
}

.fmt-btn + .fmt-btn {
  border-left: 1px solid var(--line-soft);
}

.fmt-btn:hover {
  color: var(--fg-0);
}

.fmt-btn.active {
  background: var(--accent-soft);
  color: var(--fg-0);
}

.export-name {
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  padding: 6px 8px;
  color: var(--fg-0);
  font-size: 11.5px;
}

.export-name:focus {
  outline: none;
  border-color: var(--accent);
}

.export-out {
  margin: 0;
  height: 130px;
  overflow: auto;
  padding: 9px 10px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-md);
  font-size: 10px;
  line-height: 1.5;
  color: var(--fg-1);
  white-space: pre;
  tab-size: 2;
}

.export-out.placeholder {
  color: var(--fg-3);
}

.export-thumb {
  height: 130px;
  width: 100%;
  object-fit: contain;
  padding: 9px 10px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-md);
  image-rendering: pixelated;
}

.export-thumb.placeholder {
  opacity: 0;
}

.export-actions {
  display: flex;
  gap: 6px;
}

.export-actions .btn {
  flex: 1 1 auto;
}
</style>
