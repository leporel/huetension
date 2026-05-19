<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import { useHarmonyApply } from '../composables/useHarmonyApply';
import { fromHSV, toHex } from '../composables/useColor';

/**
 * Per-color H / S / V sliders for the workspace's selected slot.
 *
 * The component holds its own HSV while a slider is dragged, and
 * re-syncs from `workspace.effectiveHSV` between gestures. effectiveHSV
 * carries the slot's hue/sat *intent* (round-trip validated), so a
 * colour driven to V = 0 keeps its hue instead of collapsing — and
 * re-deriving from it is jitter-free, no write-echo guard needed.
 *
 * Apply paths:
 *  - slot 0 (base) → `applyBaseHex`: the edit propagates the active
 *    harmony, exactly like the wheel's base handle and ColorPicker. It
 *    does NOT flip to custom.
 *  - any other slot → `setHex` + flip to custom: a non-base per-colour
 *    edit ends the harmony.
 */
const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const { applyBaseHex } = useHarmonyApply();

const selectedHex = computed(
  () => workspace.colors[workspace.selectedSlot]?.hex,
);
const locked = computed(
  () => workspace.colors[workspace.selectedSlot]?.locked ?? false,
);

const hsv = ref({ h: 0, s: 0, v: 0 });

// Re-sync from the slot whenever the selection or its effective HSV
// changes. effectiveHSV returns this component's own intent right after
// a write, so a self-write re-sync is a harmless no-op.
watch(
  () => {
    const e = workspace.effectiveHSV(workspace.selectedSlot);
    return `${workspace.selectedSlot}:${e.h}:${e.s}:${e.v}`;
  },
  () => {
    hsv.value = workspace.effectiveHSV(workspace.selectedSlot);
  },
  { immediate: true },
);

const channels = computed(() => [
  {
    key: 'h' as const, label: 'H', min: 0, max: 360,
    value: Math.round(hsv.value.h), display: `${Math.round(hsv.value.h)}°`,
  },
  {
    key: 's' as const, label: 'S', min: 0, max: 100,
    value: Math.round(hsv.value.s * 100), display: `${Math.round(hsv.value.s * 100)}%`,
  },
  {
    key: 'v' as const, label: 'V', min: 0, max: 100,
    value: Math.round(hsv.value.v * 100), display: `${Math.round(hsv.value.v * 100)}%`,
  },
]);

function applyHsv(): void {
  if (locked.value) return;
  const slot = workspace.selectedSlot;
  const { h, s, v } = hsv.value;
  // Record the HSV intent before any hex write so effectiveHSV never
  // sees a fresh hex against a stale intent.
  workspace.recordSlotHsv(slot, { h, s, v });
  const hex = toHex(fromHSV(h, s, v));
  if (slot === 0) {
    // Base edit propagates the active harmony — does NOT flip to custom.
    applyBaseHex(hex);
    return;
  }
  // A non-base per-colour edit hand-composes the palette → end any
  // active harmony, so a later Count change resizes instead of
  // regenerating from slot 0 and discarding these edits.
  if (harmony.type !== 'custom') harmony.setType('custom');
  workspace.setHex(slot, hex);
}

function onInput(key: 'h' | 's' | 'v', e: Event): void {
  const raw = Number((e.target as HTMLInputElement).value);
  if (Number.isNaN(raw)) return;
  hsv.value[key] = key === 'h' ? raw : raw / 100;
  applyHsv();
}

// History: pause on press, commit on release → one undo step per drag.
// Keyboard arrows fire `input` without a press — each is its own
// snapshot, i.e. one undo step per key press, which is fine.
let paused = false;
function onDown(): void {
  if (locked.value || paused) return;
  paused = true;
  workspace.historyAdapter.pause();
}
function onUp(): void {
  if (!paused) return;
  paused = false;
  workspace.historyAdapter.resume(true);
}
</script>

<template>
  <div class="color-sliders">
    <div class="cs-head">
      <span class="cs-swatch" :style="{ background: selectedHex }" />
      <span class="cs-title">Slot {{ workspace.selectedSlot + 1 }}</span>
      <span v-if="locked" class="cs-lock mono">locked</span>
    </div>
    <div v-for="ch in channels" :key="ch.key" class="cs-row">
      <span class="cs-lbl">{{ ch.label }}</span>
      <input
        type="range"
        :min="ch.min"
        :max="ch.max"
        step="1"
        :value="ch.value"
        :disabled="locked"
        :aria-label="`${ch.label} for slot ${workspace.selectedSlot + 1}`"
        @pointerdown="onDown"
        @pointerup="onUp"
        @pointercancel="onUp"
        @input="onInput(ch.key, $event)"
      />
      <span class="cs-val mono">{{ ch.display }}</span>
    </div>
  </div>
</template>

<style scoped>
.color-sliders {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.cs-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cs-swatch {
  width: 16px;
  height: 16px;
  border-radius: 5px;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.12);
}

.cs-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--fg-1);
}

.cs-lock {
  margin-left: auto;
  font-size: 9.5px;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.cs-row {
  display: grid;
  grid-template-columns: 14px 1fr 40px;
  align-items: center;
  gap: 8px;
}

.cs-lbl {
  font-size: 11px;
  font-weight: 600;
  color: var(--fg-2);
}

.cs-row input[type='range'] {
  width: 100%;
  height: 6px;
  accent-color: var(--accent);
}

.cs-row input[type='range']:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.cs-val {
  font-size: 10.5px;
  color: var(--fg-1);
  text-align: right;
}
</style>
