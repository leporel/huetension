<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import VChart from 'vue-echarts';
import { useWorkspaceStore } from '../stores/workspace';
import { useReviewStore } from '../stores/review';
import { fromHex, toHex, type RGB } from '../composables/useColor';
import { apca, wcag21 } from '../composables/useContrast';
import { suggestLightnessFix, type FixResult } from '../composables/useContrastFix';
import { useChartTheme } from '../composables/useChartTheme';

/**
 * Contrast checker — WCAG 2.1 + APCA, computed client-side via
 * `useContrast` (parity-pinned against the Go backend by
 * scripts/check-contrast-parity.ts).
 *
 * The palette swatches come from the shared *review palette*
 * (`useReviewStore`) — a workspace snapshot edited independently by the
 * analysis cards. A lightness slider under each foreground swatch, plus
 * applied fixes, write back into that store, so the change shows here
 * and in the Color-blindness card at once, without disturbing the
 * generator's workspace. The slider edits only OkLCH lightness — the
 * store keeps chroma + hue, so a swatch survives a trip to pure black /
 * white. `fgIndex` / `bgIndex` track which review slot fg / bg
 * mirror so an applied fix lands on the source swatch in place; typing a
 * hex by hand drops that link. "↻ Reload" re-seeds the review palette
 * from the workspace; "Sync to workspace" pushes the fg/bg pair back on
 * an explicit click (locks honoured).
 *
 * "Suggest fixes" sweeps the foreground's OkLCH lightness against
 * the target and plots the search as an ECharts histogram — pass bars
 * green, the nearest passing lightness is one click to apply.
 */

type Algo = 'wcag21' | 'apca';

const workspace = useWorkspaceStore();
const review = useReviewStore();
const chartTheme = useChartTheme();

const algo = ref<Algo>('wcag21');

// Raw editable strings — may be mid-edit / invalid. `*Rgb` is the
// validated view; contrast is computed only when both parse.
const fg = ref(review.colors[0] ?? '#1A1A1A');
const bg = ref(review.colors[1] ?? '#FFFFFF');

// Which review-palette slot fg / bg currently mirror, or null when the
// hex was typed by hand (no slot to write an applied fix back to).
const fgIndex = ref<number | null>(review.colors.length > 0 ? 0 : null);
const bgIndex = ref<number | null>(review.colors.length > 1 ? 1 : null);

function parse(hex: string): RGB | null {
  try {
    return fromHex(hex);
  } catch {
    return null;
  }
}

const fgRgb = computed(() => parse(fg.value));
const bgRgb = computed(() => parse(bg.value));
const valid = computed(() => fgRgb.value !== null && bgRgb.value !== null);

const fgCss = computed(() => (fgRgb.value ? toHex(fgRgb.value) : 'transparent'));
const bgCss = computed(() => (bgRgb.value ? toHex(bgRgb.value) : 'transparent'));

const wcag = computed(() =>
  fgRgb.value && bgRgb.value ? wcag21(fgRgb.value, bgRgb.value) : null,
);
const apcaRes = computed(() =>
  fgRgb.value && bgRgb.value ? apca(fgRgb.value, bgRgb.value) : null,
);

const score = computed(() => {
  if (algo.value === 'wcag21') {
    return wcag.value ? `${wcag.value.ratio.toFixed(2)} : 1` : '—';
  }
  return apcaRes.value ? `Lc ${apcaRes.value.lc.toFixed(1)}` : '—';
});

interface Badge {
  label: string;
  note: string;
  pass: boolean;
}

const badges = computed<Badge[]>(() => {
  if (algo.value === 'wcag21') {
    const r = wcag.value;
    if (!r) return [];
    return [
      { label: 'AA', note: 'normal · 4.5+', pass: r.aa },
      { label: 'AA Large', note: 'large · 3.0+', pass: r.aa_large },
      { label: 'AAA', note: 'normal · 7.0+', pass: r.aaa },
      { label: 'AAA Large', note: 'large · 4.5+', pass: r.aaa_large },
    ];
  }
  const a = apcaRes.value;
  if (!a) return [];
  return [
    { label: 'Body text', note: '|Lc| 75+', pass: a.body_text },
    { label: 'Content', note: '|Lc| 60+', pass: a.content },
    { label: 'Large heading', note: '|Lc| 45+', pass: a.large_heading },
    { label: 'Icon / UI', note: '|Lc| 30+', pass: a.icon },
  ];
});

function normalize(which: 'fg' | 'bg'): void {
  const target = which === 'fg' ? fg : bg;
  const rgb = parse(target.value);
  if (rgb) target.value = toHex(rgb).toUpperCase();
}

function swap(): void {
  [fg.value, bg.value] = [bg.value, fg.value];
  // Keep each slot link with its color through the swap, so a later
  // applyFix still writes back to the right review-palette swatch.
  [fgIndex.value, bgIndex.value] = [bgIndex.value, fgIndex.value];
}

/** Point the foreground at a review-palette slot. */
function pickFg(i: number): void {
  const hex = review.colors[i];
  if (hex === undefined) return;
  fgIndex.value = i;
  fg.value = hex;
}

/** Point the background at a review-palette slot. */
function pickBg(i: number): void {
  const hex = review.colors[i];
  if (hex === undefined) return;
  bgIndex.value = i;
  bg.value = hex;
}

/**
 * Sweep a review swatch's lightness from its slider. The store holds
 * each slot as an OkLCH intent and only L is touched here, so a swatch
 * dragged to pure black / white keeps its chroma + hue. The change goes
 * to the shared store (the Color-blindness card updates too) and mirrors
 * into fg / bg while the slot is the active pair.
 */
function setSlotL(i: number, L: number): void {
  review.setLightness(i, L);
  const hex = review.colors[i];
  if (hex === undefined) return;
  if (fgIndex.value === i) fg.value = hex;
  if (bgIndex.value === i) bg.value = hex;
}

function onSlideL(i: number, e: Event): void {
  setSlotL(i, Number((e.target as HTMLInputElement).value));
}

/** Re-seed the review palette from the workspace and reset the pair. */
function reloadPalette(): void {
  review.reload();
  const first = review.colors[0];
  const second = review.colors[1];
  fgIndex.value = first !== undefined ? 0 : null;
  bgIndex.value = second !== undefined ? 1 : null;
  if (first !== undefined) fg.value = first;
  if (second !== undefined) bg.value = second;
}

/** Push the validated pair back to workspace slots 0+1 in one undo step. */
function syncToWorkspace(): void {
  if (!fgRgb.value || !bgRgb.value) return;
  const next = workspace.colors.map((s) => ({ ...s }));
  if (next[0] && !next[0].locked) next[0].hex = toHex(fgRgb.value);
  if (next[1] && !next[1].locked) next[1].hex = toHex(bgRgb.value);
  workspace.setColors(next);
}

// --- Suggest fixes -----------------------------------------------------

interface TargetOpt {
  label: string;
  value: number;
}

const targetOpts = computed<TargetOpt[]>(() =>
  algo.value === 'wcag21'
    ? [
        { label: 'AA · 4.5', value: 4.5 },
        { label: 'AAA · 7.0', value: 7 },
      ]
    : [
        { label: 'Content · 60', value: 60 },
        { label: 'Body · 75', value: 75 },
      ],
);

const target = ref(4.5);
const fixResult = ref<FixResult | null>(null);

// Algorithm switch invalidates the target scale (4.5 is meaningless for
// APCA); snap to the first option for the new algorithm.
watch(algo, () => {
  target.value = targetOpts.value[0]?.value ?? 4.5;
});

// Any input the search depends on changing makes a shown result stale.
watch([fg, bg, algo, target], () => {
  fixResult.value = null;
});

function runSuggest(): void {
  if (!fgRgb.value || !bgRgb.value) return;
  fixResult.value = suggestLightnessFix(
    fgRgb.value,
    bgRgb.value,
    algo.value,
    target.value,
  );
}

function applyFix(): void {
  const hex = fixResult.value?.suggested?.hex;
  if (!hex) return;
  const up = hex.toUpperCase();
  fg.value = up;
  // Write the fix onto its source swatch in place — no slot reordering,
  // and the Color-blindness card (same store) picks it up. A hand-typed
  // foreground has no slot link, so there is nothing to write back.
  if (fgIndex.value !== null) review.setColor(fgIndex.value, up);
}

function scoreText(v: number): string {
  return algo.value === 'wcag21' ? v.toFixed(2) : `Lc ${v.toFixed(0)}`;
}

const fixChartOption = computed(() => {
  const t = chartTheme.value;
  const r = fixResult.value;
  if (!r) return {};
  return {
    animation: false,
    grid: { left: 40, right: 14, top: 12, bottom: 26 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: t.surface,
      borderColor: t.axisLine,
      textStyle: { color: t.text, fontSize: 11 },
      valueFormatter: (v: number) => v.toFixed(2),
    },
    xAxis: {
      type: 'category',
      data: r.samples.map((s) => s.L.toFixed(2)),
      name: 'OkL',
      nameTextStyle: { color: t.text, fontSize: 9 },
      nameGap: 16,
      axisLine: { lineStyle: { color: t.axisLine } },
      axisTick: { show: false },
      axisLabel: { color: t.text, fontSize: 9 },
    },
    yAxis: {
      type: 'value',
      name: algo.value === 'wcag21' ? 'ratio' : '|Lc|',
      nameTextStyle: { color: t.text, fontSize: 9 },
      axisLine: { lineStyle: { color: t.axisLine } },
      axisLabel: { color: t.text, fontSize: 9 },
      splitLine: { lineStyle: { color: t.splitLine, type: 'dashed' } },
    },
    series: [
      {
        type: 'bar',
        barCategoryGap: '12%',
        data: r.samples.map((s) => ({
          value: s.score,
          itemStyle: { color: s.pass ? t.good : t.axisLine },
        })),
        markLine: {
          symbol: 'none',
          data: [
            {
              yAxis: r.target,
              lineStyle: { color: t.accent, type: 'dashed' as const },
              label: { color: t.text, fontSize: 9, formatter: 'target' },
            },
          ],
        },
      },
    ],
  };
});
</script>

<template>
  <div class="ctr">
    <div class="main">
      <div class="controls">
        <div class="field">
          <label class="fld">Foreground</label>
          <div class="input">
            <span class="swatch" :style="{ background: fgCss }" />
            <input
              v-model="fg"
              class="mono hex"
              :class="{ bad: !fgRgb }"
              spellcheck="false"
              aria-label="foreground color hex"
              @input="fgIndex = null"
              @change="normalize('fg')"
            />
          </div>
          <div class="ws-pick">
            <div v-for="(c, i) in review.colors" :key="'fg' + i" class="ws-cell">
              <button
                type="button"
                class="ws-sw"
                :class="{ active: fgIndex === i }"
                :style="{ background: c }"
                :title="`set foreground to ${c}`"
                @click="pickFg(i)"
              />
              <input
                type="range"
                class="l-slider"
                min="0"
                max="1"
                step="0.005"
                :value="review.lightnessOf(i)"
                :aria-label="`lightness of palette color ${i + 1}`"
                @input="onSlideL(i, $event)"
              />
            </div>
          </div>
        </div>

        <div class="field">
          <label class="fld">Background</label>
          <div class="input">
            <span class="swatch" :style="{ background: bgCss }" />
            <input
              v-model="bg"
              class="mono hex"
              :class="{ bad: !bgRgb }"
              spellcheck="false"
              aria-label="background color hex"
              @input="bgIndex = null"
              @change="normalize('bg')"
            />
          </div>
          <div class="ws-pick">
            <button
              v-for="(c, i) in review.colors"
              :key="'bg' + i"
              type="button"
              class="ws-sw"
              :class="{ active: bgIndex === i }"
              :style="{ background: c }"
              :title="`set background to ${c}`"
              @click="pickBg(i)"
            />
          </div>
        </div>

        <div class="actions">
          <button type="button" class="btn" @click="swap">⇄ Swap</button>
          <button
            type="button"
            class="btn"
            title="re-seed the review palette from the workspace"
            @click="reloadPalette"
          >
            ↻ Reload
          </button>
          <button type="button" class="btn" :disabled="!valid" @click="syncToWorkspace">
            ↑ Sync to workspace
          </button>
        </div>

        <div class="seg" role="tablist" aria-label="contrast algorithm">
          <button
            type="button"
            role="tab"
            :class="{ active: algo === 'wcag21' }"
            :aria-selected="algo === 'wcag21'"
            @click="algo = 'wcag21'"
          >
            WCAG 2.1
          </button>
          <button
            type="button"
            role="tab"
            :class="{ active: algo === 'apca' }"
            :aria-selected="algo === 'apca'"
            @click="algo = 'apca'"
          >
            APCA
          </button>
        </div>
      </div>

      <div class="preview">
        <div class="sample" :style="{ background: bgCss, color: fgCss }">
          <p class="s14">The quick brown fox · 14px regular</p>
          <p class="s14 bold">The quick brown fox · 14px bold</p>
          <p class="s18">The quick brown fox · 18.66px</p>
          <p class="s24">The quick brown fox · 24px</p>
        </div>

        <div class="verdict">
          <div class="score-row">
            <span class="algo-tag mono">{{ algo === 'wcag21' ? 'WCAG 2.1' : 'APCA' }}</span>
            <span class="score">{{ score }}</span>
          </div>
          <div class="badges">
            <span
              v-for="b in badges"
              :key="b.label"
              class="badge"
              :class="b.pass ? 'pass' : 'fail'"
            >
              <span class="b-mark">{{ b.pass ? '✓' : '✕' }}</span>
              <span class="b-label">{{ b.label }}</span>
              <span class="b-note mono">{{ b.note }}</span>
            </span>
          </div>
          <p v-if="!valid" class="warn mono">enter a valid hex for both colors</p>
        </div>
      </div>
    </div>

    <div v-if="valid" class="fix">
      <div class="fix-head">
        <span class="fld">Suggest fixes</span>
        <label class="fix-target">
          target
          <select v-model.number="target" class="select" aria-label="contrast target">
            <option v-for="o in targetOpts" :key="o.value" :value="o.value">
              {{ o.label }}
            </option>
          </select>
        </label>
        <button type="button" class="btn" @click="runSuggest">Analyse foreground</button>
      </div>

      <div v-if="fixResult" class="fix-body">
        <VChart class="fix-chart" :option="fixChartOption" autoresize />
        <div class="fix-result">
          <template v-if="fixResult.suggested">
            <span class="fld">nearest passing lightness</span>
            <div class="fix-swatches">
              <div class="fx">
                <span class="fx-sw" :style="{ background: fgCss }" />
                <span class="mono">now · {{ scoreText(fixResult.currentScore) }}</span>
              </div>
              <span class="arrow">→</span>
              <div class="fx">
                <span class="fx-sw" :style="{ background: fixResult.suggested.hex }" />
                <span class="mono">
                  {{ fixResult.suggested.hex.toUpperCase() }} ·
                  {{ scoreText(fixResult.suggested.score) }}
                </span>
              </div>
            </div>
            <button type="button" class="btn accent" @click="applyFix">
              Apply to foreground
            </button>
          </template>
          <p v-else class="warn mono">
            no OkLCH lightness reaches the target at this chroma / hue — adjust the
            background or the foreground hue
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ctr {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.main {
  display: grid;
  grid-template-columns: 1fr 1.1fr;
  gap: 16px;
  align-items: start;
}

@media (max-width: 720px) {
  .main {
    grid-template-columns: 1fr;
  }
}

.controls {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.field {
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
  padding: 2px 0;
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

.ws-cell {
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.ws-sw {
  width: 100%;
  height: 16px;
  border-radius: 4px;
  border: 1px solid var(--line-soft);
  cursor: pointer;
  padding: 0;
}

.ws-sw:hover {
  border-color: var(--accent-line);
}

/* The swatch currently mirrored by fg / bg. */
.ws-sw.active {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}

/* Per-swatch lightness slider — sweeps the review color's OkLCH L,
   holding chroma + hue. The fg and bg rows show the slider for the
   same review slot, so they track each other. */
.l-slider {
  width: 100%;
  height: 12px;
  margin: 0;
  padding: 0;
  cursor: pointer;
  accent-color: var(--accent);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.btn {
  flex: 1 1 auto;
  padding: 7px 8px;
  font-size: 11px;
  font-weight: 500;
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
  font-weight: 600;
}

.seg {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 3px;
  padding: 3px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 9px;
}

.seg button {
  padding: 6px 8px;
  font-size: 11px;
  font-weight: 500;
  color: var(--fg-2);
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
}

.seg button.active {
  background: var(--accent-soft);
  color: var(--fg-0);
  box-shadow: inset 0 0 0 1px var(--accent-line);
}

.preview {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sample {
  border-radius: 10px;
  padding: 14px 16px;
  min-height: 150px;
  display: flex;
  flex-direction: column;
  gap: 7px;
  justify-content: center;
  box-shadow: inset 0 0 0 1px oklch(0.5 0 0 / 0.15);
}

.sample p {
  margin: 0;
  letter-spacing: -0.005em;
}

.s14 {
  font-size: 14px;
}

.s14.bold {
  font-weight: 700;
}

.s18 {
  font-size: 18.66px;
}

.s24 {
  font-size: 24px;
  font-weight: 600;
}

.verdict {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.score-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.algo-tag {
  font-size: 10px;
  color: var(--fg-3);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.score {
  font-family: 'Inter Tight', sans-serif;
  font-weight: 600;
  font-size: 30px;
  letter-spacing: -0.02em;
  color: var(--fg-0);
}

.badges {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
}

.badge {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 9px;
  border-radius: 8px;
  border: 1px solid var(--line-soft);
  font-size: 11px;
}

.badge.pass {
  border-color: var(--good-line);
  background: var(--good-soft);
}

.badge.fail {
  border-color: var(--bad-line);
  background: var(--bad-soft);
}

.b-mark {
  font-weight: 700;
}

.badge.pass .b-mark {
  color: var(--good);
}

.badge.fail .b-mark {
  color: var(--bad);
}

.b-label {
  font-weight: 600;
  color: var(--fg-0);
}

.b-note {
  margin-left: auto;
  font-size: 9.5px;
  color: var(--fg-3);
}

.warn {
  font-size: 10.5px;
  color: var(--warn);
}

/* --- Suggest fixes --- */

.fix {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 14px;
  border-top: 1px dashed var(--line-soft);
}

.fix-head {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.fix-head .btn {
  flex: 0 0 auto;
}

.fix-target {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--fg-2);
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

.fix-body {
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  gap: 14px;
  align-items: center;
}

@media (max-width: 720px) {
  .fix-body {
    grid-template-columns: 1fr;
  }
}

.fix-chart {
  height: 168px;
  width: 100%;
}

.fix-result {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.fix-swatches {
  display: flex;
  align-items: center;
  gap: 10px;
}

.fx {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.fx-sw {
  width: 100%;
  min-width: 56px;
  height: 34px;
  border-radius: 7px;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.1);
}

.fx .mono {
  font-size: 9.5px;
  color: var(--fg-2);
}

.arrow {
  color: var(--fg-3);
  font-size: 14px;
}
</style>
