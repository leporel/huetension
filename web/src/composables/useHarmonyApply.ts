/**
 * Single funnel for "set slot 0 (the base) and let the active harmony
 * propagate to the rest of the palette". Consumed by ColorWheel,
 * HarmonySelector, and ColorPicker so the lock-preservation contract
 * lives in one place — the advisor caught it slipping out of one of
 * three duplicates in S5a, and three duplicates is still two too many.
 *
 * Contract:
 *   - Locked slot 0 is treated as the harmony *anchor*, not a kill
 *     switch: regeneration still happens, but slot 0's hex is held
 *     constant. This lets mode-pick and count-slider work while a
 *     brand color is pinned in slot 0.
 *   - Custom harmony with locked slot 0: full no-op (Custom only
 *     changes slot 0, so a lock means "no movement").
 *   - Custom harmony with unlocked slot 0: only slot 0's hex changes;
 *     other slots untouched.
 *   - Any other harmony: regenerate via `generateHarmony` with the
 *     current `harmony.count` + `harmony.step`. Locked slots in the
 *     resulting array preserve their existing hex.
 */

import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import {
  fromHex,
  generateHarmony,
  toHex,
  type HarmonyType,
  type RGB,
} from './useColor';

export function useHarmonyApply() {
  const workspace = useWorkspaceStore();
  const harmony = useHarmonyStore();

  function applyBaseRGB(base: RGB): void {
    const slot0 = workspace.colors[0];
    if (!slot0) return;

    if (harmony.type === 'custom') {
      if (slot0.locked) return;
      workspace.setHex(0, toHex(base));
      return;
    }

    // Locked slot 0 → use its current hex as the harmony anchor so
    // mode-pick and count-slider still propagate through the rest of
    // the palette. Unlocked → use the supplied base (a wheel/picker
    // gesture).
    const anchor = slot0.locked ? fromHex(slot0.hex) : base;

    const palette = generateHarmony(harmony.type as HarmonyType, anchor, {
      count: harmony.count,
      step: harmony.step,
    });
    workspace.setColors(
      palette.map((rgb, i) => {
        const cur = workspace.colors[i];
        if (cur?.locked) return { hex: cur.hex, locked: true };
        return { hex: toHex(rgb), locked: false };
      }),
    );
  }

  function applyBaseHex(hex: string): void {
    applyBaseRGB(fromHex(hex));
  }

  return { applyBaseRGB, applyBaseHex };
}
