/**
 * "Set the base color, regenerate the harmony" funnel. Consumed by
 * ColorWheel, HarmonySelector, and ColorPicker so the lock-preservation
 * contract lives in one place.
 *
 * Contract:
 *   - The base color is authoritative state in the harmony store. It is
 *     placed by `generateHarmony` at slot 0 for hue-rotation harmonies
 *     and Shades, but at the centre for Analogous / Monochromatic — so a
 *     stored base is the only thing the picker, wheel, and Count-change
 *     can all agree on.
 *   - `applyBaseRGB` changes the base, then regenerates.
 *   - `regenerate` rebuilds the palette from the *current* base — used
 *     when only the mode or count changes.
 *   - Custom harmony: no propagation — `applyBaseRGB` only updates slot 0;
 *     a locked slot 0 makes it a no-op.
 *   - Locked slots keep their existing hex through a regeneration. The
 *     base color is persistent in the store, so no slot need be locked
 *     to pin the anchor.
 */

import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import {
  fromHex,
  fromHSL,
  generateHarmony,
  toHex,
  type HarmonyType,
  type RGB,
} from './useColor';

/**
 * A random, reasonably vivid color: a full-circle hue with healthy
 * saturation and mid lightness, so a randomised palette comes out
 * lively rather than muddy or washed out.
 */
function randomHex(): string {
  const h = Math.random() * 360;
  const s = 0.55 + Math.random() * 0.4; // 0.55 .. 0.95
  const l = 0.42 + Math.random() * 0.28; // 0.42 .. 0.70
  return toHex(fromHSL(h, s, l));
}

export function useHarmonyApply() {
  const workspace = useWorkspaceStore();
  const harmony = useHarmonyStore();

  function regenerate(): void {
    if (harmony.type === 'custom') return;
    const palette = generateHarmony(
      harmony.type as HarmonyType,
      fromHex(harmony.baseColor),
      { count: harmony.count, step: harmony.step },
    );
    workspace.setColors(
      palette.map((rgb, i) => {
        const cur = workspace.colors[i];
        if (cur?.locked) return { hex: cur.hex, locked: true };
        return { hex: toHex(rgb), locked: false };
      }),
    );
  }

  function applyBaseRGB(base: RGB): void {
    if (harmony.type === 'custom') {
      // Custom: the picker / base handle edits slot 0 directly.
      const slot0 = workspace.colors[0];
      if (!slot0 || slot0.locked) return;
      workspace.setHex(0, toHex(base));
      return;
    }
    harmony.setBaseColor(toHex(base));
    regenerate();
  }

  function applyBaseHex(hex: string): void {
    applyBaseRGB(fromHex(hex));
  }

  /**
   * Randomise the palette within the active mode. A harmony mode gets a
   * fresh random base run through its rule; `custom` mode fills every
   * unlocked slot with its own random colour. `harmony.type` / `count`
   * stay put — a random within the current mode is not a wholesale
   * replace, so no harmony sync is needed. Locked slots keep their hex.
   * One undo step.
   */
  function randomize(): void {
    if (harmony.type === 'custom') {
      workspace.setColors(
        workspace.colors.map((c) =>
          c.locked ? c : { hex: randomHex(), locked: false },
        ),
      );
      return;
    }
    applyBaseHex(randomHex());
  }

  return { applyBaseRGB, applyBaseHex, regenerate, randomize };
}
