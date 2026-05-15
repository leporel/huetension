<script setup lang="ts">
import { computed, ref } from 'vue';
import { watchDebounced } from '@vueuse/core';
import VChart from 'vue-echarts';
import { gradient } from '../api';
import type { ColorJSON } from '../api/types';
import type { GradientEasing, GradientSpace } from '../api/gradient';
import { useWorkspaceStore } from '../stores/workspace';
import { fromHex, luminance, toHex, toOkLab, type RGB } from '../composables/useColor';
import { useChartTheme } from '../composables/useChartTheme';

/**
 * Multi-stop gradient studio. Stops are evenly distributed — that's the
 * only spacing `gradient.MultiStop` honours on the backend, so an
 * arbitrary-position editor would desync the preview from the discrete
 * output. The CSS bar is the live visual; the discrete N-step palette
 * comes from /api/v1/gradient (debounced) so it byte-matches the CLI;
 * the ECharts line plots OkLab lightness of those steps — the "is this
 * ramp perceptually even" diagnostic.
 *
 * "Extract Gradient" reseeds the stops from the two extreme-luminance
 * colors in the workspace palette (plan §5).
 */

const workspace = useWorkspaceStore();
const chartTheme = useChartTheme();

const SPACES: GradientSpace[] = ['oklch', 'oklab', 'lab', 'hsl', 'rgb'];
const EASINGS: GradientEasing[] = ['linear', 'ease-in', 'ease-out', 'ease-in-out'];
const MAX_STEPS = 16;

/** Darkest + lightest workspace colors by WCAG luminance. */
function extremes(): string[] {
  const cs = workspace.colors;
  if (cs.length === 0) return ['#1A1A1A', '#FFFFFF'];
  let loHex = cs[0]!.hex;
  let hiHex = cs[0]!.hex;
  let loL = Infinity;
  let hiL = -Infinity;
  for (const s of cs) {
    let L: number;
    try {
      L = luminance(fromHex(s.hex));
    } catch {
      continue;
    }
    if (L < loL) {
      loL = L;
      loHex = s.hex;
    }
    if (L > hiL) {
      hiL = L;
      hiHex = s.hex;
    }
  }
  // Single-color or all-equal palette → still need 2 distinct stops.
  if (loHex === hiHex) return [loHex.toUpperCase(), '#FFFFFF'];
  return [loHex.toUpperCase(), hiHex.toUpperCase()];
}

const stops = ref<string[]>(extremes());
const selected = ref(0);
const steps = ref(7);
const space = ref<GradientSpace>('oklch');
const easing = ref<GradientEasing>('linear');

const result = ref<ColorJSON[]>([]);
const loading = ref(false);
const errorMsg = ref<string | null>(null);

// MultiStop requires steps >= stop count.
const minSteps = computed(() => stops.value.length);
const effectiveSteps = computed(() => Math.max(steps.value, minSteps.value));

const barGradient = computed(
  () => `linear-gradient(90deg, ${stops.value.join(', ')})`,
);

function stopLeft(i: number): string {
  const n = stops.value.length;
  return n > 1 ? `${(i / (n - 1)) * 100}%` : '50%';
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
  const next = [...stops.value, '#888888'];
  stops.value = next;
  if (steps.value < next.length) steps.value = next.length;
  selectStop(next.length - 1);
}

function removeStop(i: number): void {
  if (stops.value.length <= 2) return;
  stops.value = stops.value.filter((_, idx) => idx !== i);
  selectStop(Math.min(selected.value, stops.value.length - 1));
}

function extractFromWorkspace(): void {
  const next = extremes();
  stops.value = next;
  if (steps.value < next.length) steps.value = next.length;
  selectStop(0);
}

// --- backend fetch -----------------------------------------------------

async function fetchGradient(): Promise<void> {
  loading.value = true;
  errorMsg.value = null;
  try {
    const res = await gradient.generate({
      stops: stops.value.join(','),
      steps: effectiveSteps.value,
      space: space.value,
      easing: easing.value,
    });
    result.value = res.palette.colors;
  } catch (e) {
    errorMsg.value = e instanceof Error ? e.message : String(e);
    result.value = [];
  } finally {
    loading.value = false;
  }
}

watchDebounced([stops, steps, space, easing], fetchGradient, {
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
</script>

<template>
  <div class="gs">
    <div class="head">
      <button type="button" class="btn accent" @click="extractFromWorkspace">
        ⛏ Extract Gradient
      </button>
      <span class="hint mono">{{ effectiveSteps }} steps · {{ space }} · {{ easing }}</span>
    </div>

    <div class="bar" :style="{ background: barGradient }">
      <button
        v-for="(s, i) in stops"
        :key="i"
        type="button"
        class="grad-stop"
        :class="{ sel: i === selected }"
        :style="{ left: stopLeft(i), background: s }"
        :title="`stop ${i + 1}: ${s}`"
        @click="selectStop(i)"
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

.btn:hover {
  border-color: var(--accent-line);
  color: var(--fg-0);
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
  cursor: pointer;
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
</style>
