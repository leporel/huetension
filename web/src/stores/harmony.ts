/**
 * Shared harmony state read by ColorWheel + HarmonySelector. Lives in
 * Pinia so it survives tab switches alongside the workspace store, but
 * is *not* persisted to localStorage — the active harmony mode is
 * session-scoped and we want a fresh "Complementary" default on every
 * reload.
 *
 * The Selector flips the mode; the wheel writes only the
 * `independentSV` toggle and the `custom` flip on a non-base drag. The
 * wheel reads `type` + `count` to decide rotation propagation:
 *   - Custom: every drag is per-slot, no harmony propagation.
 *   - any other type: dragging the *base* handle (at `baseIndex`)
 *     regenerates the whole palette through `generateHarmony`; dragging
 *     any other handle is an individual edit (flips to Custom).
 *
 * The base color is authoritative state here, not "whichever slot":
 * `generateHarmony` places the base at slot 0 for hue-rotation harmonies
 * but at the centre for Analogous / Monochromatic, so a stored base is
 * the only thing the picker, wheel, and Count-change can all agree on.
 */

import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { harmonyBaseIndex, type HarmonyType } from '../composables/useColor';
import { useWorkspaceStore } from './workspace';

export type HarmonyMode = HarmonyType | 'custom';

export const useHarmonyStore = defineStore('harmony', () => {
  const type = ref<HarmonyMode>('complementary');
  const count = ref<number>(5);
  const step = ref<number>(30);

  // When on, dragging a non-base wheel handle tweaks only that slot's
  // saturation/value — the harmony keeps its mode (no flip to `custom`).
  // The harmony owns the hues; S/V is a per-slot tweak. On by default;
  // the value carries across harmony-mode changes.
  const independentSV = ref<boolean>(true);

  // Explicitly-set base color (hex). Null until the picker / wheel / a
  // "set as base" action sets one; `baseColor` then falls back to the
  // current base slot, so a fresh session still regenerates correctly.
  const explicitBase = ref<string | null>(null);

  // Slot index `generateHarmony` puts the base at, for the active mode.
  const baseIndex = computed<number>(() =>
    type.value === 'custom'
      ? 0
      : harmonyBaseIndex(type.value, count.value),
  );

  // The harmony's base color. In `custom` mode there is no harmony, so it
  // tracks slot 0 directly. Otherwise the explicit base wins; before one
  // is set it falls back to whatever sits at `baseIndex`.
  const baseColor = computed<string>(() => {
    const ws = useWorkspaceStore();
    if (type.value !== 'custom' && explicitBase.value !== null) {
      return explicitBase.value;
    }
    return ws.colors[baseIndex.value]?.hex ?? ws.colors[0]?.hex ?? '#000000';
  });

  function setType(t: HarmonyMode) {
    type.value = t;
  }

  function setCount(n: number) {
    if (n < 1) n = 1;
    if (n > 12) n = 12;
    count.value = n;
  }

  function setBaseColor(hex: string) {
    explicitBase.value = hex;
  }

  return {
    type,
    count,
    step,
    independentSV,
    baseIndex,
    baseColor,
    setType,
    setCount,
    setBaseColor,
  };
});
