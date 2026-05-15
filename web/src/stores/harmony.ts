/**
 * Shared harmony state read by ColorWheel + HarmonySelector. Lives in
 * Pinia so it survives tab switches alongside the workspace store, but
 * is *not* persisted to localStorage — the active harmony mode is
 * session-scoped and we want a fresh "Complementary" default on every
 * reload.
 *
 * Wheel writes nothing here — only Selector flips the mode. The wheel
 * reads `type` + `count` to decide rotation propagation:
 *   - Custom: every drag is per-slot, no harmony propagation.
 *   - any other type: dragging slot 0 rotates the whole palette through
 *     `generateHarmony(type, newBase, { count })`. Dragging slot N
 *     (N > 0) only moves N (no spring-back yet — that lands when we
 *     wire HarmonySelector's snap-back rule).
 */

import { defineStore } from 'pinia';
import { ref } from 'vue';
import type { HarmonyType } from '../composables/useColor';

export type HarmonyMode = HarmonyType | 'custom';

export const useHarmonyStore = defineStore('harmony', () => {
  const type = ref<HarmonyMode>('complementary');
  const count = ref<number>(5);
  const step = ref<number>(30);

  function setType(t: HarmonyMode) {
    type.value = t;
  }

  function setCount(n: number) {
    if (n < 1) n = 1;
    if (n > 12) n = 12;
    count.value = n;
  }

  return {
    type,
    count,
    step,
    setType,
    setCount,
  };
});
