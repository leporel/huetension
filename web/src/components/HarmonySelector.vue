<script setup lang="ts">
import { computed } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore, type HarmonyMode } from '../stores/harmony';
import {
  naturalAnchorCount,
  type HarmonyType,
} from '../composables/useColor';
import { useHarmonyApply } from '../composables/useHarmonyApply';

interface ModeDef {
  key: HarmonyMode;
  label: string;
  sub: string;
}

const MODES: ModeDef[] = [
  { key: 'complementary', label: 'Comp', sub: '0 · 180' },
  { key: 'analogous', label: 'Analog', sub: '±30°' },
  { key: 'triadic', label: 'Triad', sub: '120°' },
  { key: 'split-complementary', label: 'Split', sub: '161 · 199' },
  { key: 'tetradic', label: 'Square', sub: '90°' },
  { key: 'double-complementary', label: 'Dbl-Comp', sub: '38 · 180 · 218' },
  { key: 'compound', label: 'Compound', sub: '30 · 150 · 180' },
  { key: 'monochromatic', label: 'Mono', sub: 'S/V ramp' },
  { key: 'shades', label: 'Shades', sub: 'L-down' },
  { key: 'custom', label: 'Custom', sub: 'free' },
];

const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const { applyBaseHex, regenerate, randomize } = useHarmonyApply();

const active = computed(() => harmony.type);

/**
 * Minimum slot count for the active mode. Hue-rotation harmonies
 * refuse counts below their natural anchor count (generateHarmony
 * throws), so the slider clamps to that floor. Non-anchor harmonies
 * (analogous / mono / shades / custom) accept any size ≥ 1.
 */
const minCount = computed(() => {
  if (active.value === 'custom') return 1;
  const n = naturalAnchorCount(active.value as HarmonyType);
  return n > 0 ? n : 1;
});

const MAX_COUNT = 12;

function pickMode(m: HarmonyMode): void {
  if (m === 'custom') {
    harmony.setType('custom');
    return;
  }

  // The base color carries over from the previous mode — capture it from
  // the OLD base slot *before* setType changes `baseIndex`. (For e.g.
  // Analogous the base sits in the centre, not slot 0.)
  const prevBase =
    workspace.colors[harmony.baseIndex]?.hex ??
    workspace.colors[0]?.hex ??
    '#000000';
  harmony.setType(m);

  // Bump count up to the new mode's natural-anchor floor if needed.
  // Switching from Complementary (min 2) to Tetradic (min 4) should
  // not throw — it should grow the palette to 4 silently.
  const required = naturalAnchorCount(m as HarmonyType);
  let count = workspace.colors.length;
  if (required > 0 && count < required) count = required;
  if (count !== harmony.count) harmony.setCount(count);

  applyBaseHex(prevBase);
}

function onCountInput(e: Event): void {
  const target = e.target as HTMLInputElement;
  const n = Number.parseInt(target.value, 10);
  if (Number.isNaN(n)) return;
  harmony.setCount(n);
  // Custom mode resizes the palette without regenerating: append
  // neutral-gray slots when growing, truncate when shrinking. Lets
  // the user explicitly compose a palette of any length.
  if (active.value === 'custom') {
    const cur = workspace.colors;
    const next = cur.slice(0, n);
    while (next.length < n) {
      next.push({ hex: '#888888', locked: false });
    }
    workspace.setColors(next);
    return;
  }
  // Count change keeps the base color — regenerate around it.
  regenerate();
}
</script>

<template>
  <div class="harmony-selector">
    <div class="harm-row">
      <button
        v-for="m in MODES"
        :key="m.key"
        type="button"
        class="harm"
        :class="{ active: active === m.key }"
        :title="m.label"
        @click="pickMode(m.key)"
      >
        <span class="mode-label">{{ m.label }}</span>
        <span class="mode-sub mono">{{ m.sub }}</span>
      </button>
    </div>

    <div class="count">
      <label class="count-label">
        <span>Count</span>
        <span class="count-val mono">{{ harmony.count }}</span>
      </label>
      <input
        type="range"
        :min="minCount"
        :max="MAX_COUNT"
        :value="harmony.count"
        @input="onCountInput"
      />
    </div>

    <!-- Randomise within the active mode — a harmony gets a fresh random
         base run through its rule, custom fills random slots. -->
    <button type="button" class="random-btn" @click="randomize">
      <svg class="rnd-ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
        <rect x="3" y="3" width="18" height="18" rx="5" />
        <circle cx="8.5" cy="8.5" r="1.3" fill="currentColor" stroke="none" />
        <circle cx="15.5" cy="8.5" r="1.3" fill="currentColor" stroke="none" />
        <circle cx="12" cy="12" r="1.3" fill="currentColor" stroke="none" />
        <circle cx="8.5" cy="15.5" r="1.3" fill="currentColor" stroke="none" />
        <circle cx="15.5" cy="15.5" r="1.3" fill="currentColor" stroke="none" />
      </svg>
      Random
    </button>
  </div>
</template>

<style scoped>
.harmony-selector {
  width: 100%;
}

.harm-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
}

.harm {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 9px 0;
  border-radius: 8px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  color: var(--fg-2);
  font-size: 11px;
  cursor: pointer;
  font-family: inherit;
}

.harm:hover {
  color: var(--fg-1);
  border-color: var(--line);
}

.harm.active {
  background: var(--accent-soft);
  color: var(--fg-0);
  border-color: var(--accent-line);
}

.mode-label {
  font-weight: 600;
  letter-spacing: -0.005em;
}

.mode-sub {
  font-size: 9.5px;
  color: var(--fg-3);
  letter-spacing: 0.02em;
}

.harm.active .mode-sub {
  color: var(--fg-2);
}

.count {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.count-label {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--fg-2);
  font-weight: 500;
}

.count-val {
  color: var(--fg-0);
  font-size: 11px;
}

.count input[type='range'] {
  width: 100%;
  accent-color: var(--accent);
  height: 6px;
}

.random-btn {
  margin-top: 14px;
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 9px 0;
  border-radius: 8px;
  background: var(--accent-soft);
  border: 1px solid var(--accent-line);
  color: var(--fg-0);
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
}

.random-btn:hover {
  border-color: var(--accent);
}

.rnd-ic {
  width: 15px;
  height: 15px;
  color: var(--accent);
}
</style>
