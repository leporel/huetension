/**
 * The "review palette" — a working copy of the workspace palette shared
 * by the analysis cards (Contrast checker, Color-blindness simulator).
 *
 * It is seeded from the workspace once, then edited independently:
 * Contrast-checker lightness tweaks and applied contrast fixes land here
 * and show up in both cards at once, but never touch the generator's
 * workspace palette. `reload()` re-syncs it from the workspace on demand
 * — the only workspace → review link, kept explicit by design.
 *
 * Each slot is stored as an OkLCH "intent" (L, C, H), not a hex string.
 * That is what lets the lightness slider sweep a swatch to pure black or
 * white and back without losing its color: the slider only ever touches
 * L, so C and H persist. Holding a hex instead would collapse chroma +
 * hue to zero at the extremes — the same root cause as wheel-handle jitter.
 *
 * Session-scoped, like the harmony store — not persisted to localStorage.
 */

import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import {
  fromHex,
  fromOkLCH,
  toHex,
  toOkLCH,
  type OkLCH,
} from '../composables/useColor';
import { useWorkspaceStore } from './workspace';

export const useReviewStore = defineStore('review', () => {
  const workspace = useWorkspaceStore();

  /** Current workspace palette as OkLCH intents. */
  function snapshotWorkspace(): OkLCH[] {
    return workspace.colors.map((c) => toOkLCH(fromHex(c.hex)));
  }

  /** Per-slot OkLCH intent — the source of truth for the review palette. */
  const intents = ref<OkLCH[]>(snapshotWorkspace());

  /** Displayed hex per slot, derived from the OkLCH intent. */
  const colors = computed<string[]>(() =>
    intents.value.map((o) => toHex(fromOkLCH(o.L, o.C, o.H)).toUpperCase()),
  );

  /** Re-seed the review palette from the current workspace palette. */
  function reload(): void {
    intents.value = snapshotWorkspace();
  }

  /** Replace a slot's color outright — e.g. an applied contrast fix. */
  function setColor(index: number, hex: string): void {
    if (index >= 0 && index < intents.value.length) {
      intents.value[index] = toOkLCH(fromHex(hex));
    }
  }

  /** Sweep only a slot's lightness, holding its chroma + hue. */
  function setLightness(index: number, L: number): void {
    const cur = intents.value[index];
    if (cur) intents.value[index] = { ...cur, L };
  }

  /** OkLCH lightness of a slot (0..1) — drives its slider. */
  function lightnessOf(index: number): number {
    return intents.value[index]?.L ?? 0;
  }

  return { colors, reload, setColor, setLightness, lightnessOf };
});
