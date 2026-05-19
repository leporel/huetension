/**
 * Wheel-handle gesture state machine. Maps PointerEvent /
 * WheelEvent into a small set of domain callbacks so the ColorWheel
 * component owns no DOM glue.
 *
 * Contract:
 *   - One undoable snapshot per gesture. `history.pause()` on
 *     pointerdown, `history.resume(true)` on pointerup. Wheel-scroll
 *     ticks are individual snapshots (each wheel notch = one undo step).
 *   - Modifier keys are *live reads* off the current event, never
 *     state-machine states. Shift = hue-only, Alt = sat-only,
 *     Ctrl/Cmd = "unlock from harmony angle" (the consumer chooses
 *     what to do with the flag — gesture composable just forwards
 *     it).
 *   - Locked slots short-circuit before pointerdown is even captured.
 */

import { ref, type Ref } from 'vue';
import { modAngle, rgbHueToRybHue } from './useColor';

// A pointerdown closer to the centre than this fraction of the disc
// radius has an ill-defined angle — drive hue from the absolute pointer
// angle for that gesture instead of a (meaningless) relative delta.
const CENTER_ABSOLUTE_FRAC = 0.15;

export interface WheelGeometry {
  /** Center in component-local CSS pixels. */
  cx: number;
  cy: number;
  /** Disc radius. Saturation maps linearly 0 (centre) → 1 (rim). */
  rOuter: number;
}

export interface GestureModifiers {
  /** Shift held — hue-only, lock saturation. */
  shift: boolean;
  /** Alt held — saturation-only, lock hue. */
  alt: boolean;
  /** Ctrl or Meta held — caller-defined (Custom-mode unlock in
   *  HarmonySelector). */
  ctrlOrMeta: boolean;
}

export interface DragDelta {
  /** New hue on the RYB artist wheel, degrees [0, 360). The consumer
   *  maps it back to an HSV hue. */
  hue: number;
  /** Hue offset from gesture start, in RYB-wheel degrees, signed. */
  hueDelta: number;
  /** New HSV saturation [0, 1]. */
  saturation: number;
  /** True if Shift was active for the *whole* gesture path so far —
   *  if the user picked up Shift mid-drag we don't retro-apply it. */
  modifiers: GestureModifiers;
}

export interface UseHandleGestureCallbacks {
  /** Returns the wheel's pixel geometry. Called once on pointerdown. */
  getGeometry: () => WheelGeometry;
  /** Locked slots short-circuit: pointerdown is ignored, no
   *  pause/resume on history. */
  isLocked: (slot: number) => boolean;
  /** The slot's HSV at the moment of pointerdown. */
  getStartHSV: (slot: number) => { h: number; s: number; v: number };
  /** Called per pointermove with the resolved drag delta. */
  onDrag: (slot: number, delta: DragDelta) => void;
  /** Called per wheel notch (positive = up = brighter). */
  onValueStep: (slot: number, steps: number, mods: GestureModifiers) => void;
  /** History pause/resume adapter from `useWorkspaceStore`. */
  history: {
    pause: () => void;
    resume: (commit?: boolean) => void;
  };
}

interface DragState {
  slot: number;
  startHue: number;
  startSat: number;
  startAngle: number; // angle from center to pointerdown position
  // True when the pointerdown landed near the centre: hue then follows
  // the absolute pointer angle, since a relative delta from an
  // ill-defined start angle would steer the handle the wrong way.
  absolute: boolean;
  geometry: WheelGeometry;
}

export interface UseHandleGestureBindings {
  /** Bind to each wheel-handle element. */
  onPointerDown: (e: PointerEvent, slot: number) => void;
  /** Bound at the wheel root — receives events post-capture. */
  onPointerMove: (e: PointerEvent) => void;
  onPointerUp: (e: PointerEvent) => void;
  /** Wheel scroll for value step. Bind on each handle. */
  onWheel: (e: WheelEvent, slot: number) => void;
  /** Public for the ColorWheel cursor styling. */
  isDragging: Ref<boolean>;
}

/**
 * pointerAngle returns the angle in degrees from (cx, cy) to (px, py),
 * with 0° at the 3 o'clock position. This is the RYB artist-wheel
 * angle the disc is painted in — the consumer (ColorWheel) maps it
 * back to an HSV hue via `rybHueToRgbHue`. Reads off screen
 * coordinates so Y points down; negating dy matches handlePosition's
 * angle direction.
 */
function pointerAngle(px: number, py: number, cx: number, cy: number): number {
  const dx = px - cx;
  const dy = py - cy;
  // atan2(y, x) returns radians counter-clockwise from +x in math
  // coords; we want clockwise from +x. Negating dy flips the axis.
  return modAngle((Math.atan2(-dy, dx) * 180) / Math.PI);
}

function pointerRadius(px: number, py: number, cx: number, cy: number): number {
  return Math.hypot(px - cx, py - cy);
}

function readModifiers(e: PointerEvent | WheelEvent): GestureModifiers {
  return {
    shift: e.shiftKey,
    alt: e.altKey,
    ctrlOrMeta: e.ctrlKey || e.metaKey,
  };
}

export function useHandleGesture(
  cb: UseHandleGestureCallbacks,
): UseHandleGestureBindings {
  const isDragging = ref(false);
  let state: DragState | null = null;
  // The handle that captured the pointer. Stored so move/up can
  // resolve which slot the gesture belongs to without touching
  // event.target (re-targeted on capture).
  let pointerOwner: { id: number; element: Element } | null = null;

  function onPointerDown(e: PointerEvent, slot: number): void {
    if (cb.isLocked(slot)) return;
    if (state) return; // ignore multi-touch second finger for now
    const geometry = cb.getGeometry();
    const start = cb.getStartHSV(slot);
    const startRadius = pointerRadius(e.clientX, e.clientY, geometry.cx, geometry.cy);
    state = {
      slot,
      // The gesture runs in RYB artist-wheel space (screen angle =
      // artist hue), so store the slot's hue as its artist angle.
      startHue: rgbHueToRybHue(start.h),
      startSat: start.s,
      startAngle: pointerAngle(e.clientX, e.clientY, geometry.cx, geometry.cy),
      absolute: startRadius < geometry.rOuter * CENTER_ABSOLUTE_FRAC,
      geometry,
    };
    isDragging.value = true;
    cb.history.pause();
    // Capture so pointermove/up arrive even outside the handle.
    const el = e.currentTarget as Element | null;
    if (el && 'setPointerCapture' in el) {
      try {
        (el as Element & { setPointerCapture: (id: number) => void })
          .setPointerCapture(e.pointerId);
        pointerOwner = { id: e.pointerId, element: el };
      } catch {
        pointerOwner = null;
      }
    }
    e.preventDefault();
  }

  function onPointerMove(e: PointerEvent): void {
    if (!state) return;
    const mods = readModifiers(e);
    const { cx, cy, rOuter } = state.geometry;
    const ang = pointerAngle(e.clientX, e.clientY, cx, cy);
    const rad = pointerRadius(e.clientX, e.clientY, cx, cy);

    // hueDelta is signed in [-180, 180] so a 1° drag near 360→0
    // wraparound produces a 1° delta, not a 359° one.
    let hueDelta = ang - state.startAngle;
    if (hueDelta > 180) hueDelta -= 360;
    if (hueDelta < -180) hueDelta += 360;

    // Map radius into [0, 1] saturation: 0 at the disc centre, 1 at the
    // rim. A pointer dragged past the rim clamps so the handle doesn't
    // "fall off".
    let sat = rad / Math.max(1e-6, rOuter);
    if (sat > 1) sat = 1;

    // Modifier locks: Alt → hue-locked (only sat moves), Shift →
    // sat-locked (only hue moves). Both can co-exist (no-op).
    // `absolute` gestures (started near the centre) take the raw pointer
    // angle; otherwise hue advances by the relative drag delta.
    let newHue: number;
    if (mods.alt) {
      newHue = state.startHue;
    } else if (state.absolute) {
      newHue = ang;
    } else {
      newHue = modAngle(state.startHue + hueDelta);
    }
    const newSat = mods.shift ? state.startSat : sat;

    cb.onDrag(state.slot, {
      hue: newHue,
      hueDelta: mods.alt ? 0 : hueDelta,
      saturation: newSat,
      modifiers: mods,
    });
  }

  function onPointerUp(_e: PointerEvent): void {
    if (!state) return;
    if (pointerOwner) {
      try {
        (pointerOwner.element as Element & {
          releasePointerCapture: (id: number) => void;
        }).releasePointerCapture(pointerOwner.id);
      } catch {
        /* ignore */
      }
      pointerOwner = null;
    }
    state = null;
    isDragging.value = false;
    cb.history.resume(true);
  }

  function onWheel(e: WheelEvent, slot: number): void {
    if (cb.isLocked(slot)) return;
    // Normalise to discrete notches. deltaY > 0 means scroll-down /
    // pull-towards-user → traditional "darker"; we invert so up-step
    // is lighter, matching the slice-plan wording.
    const steps = e.deltaY === 0 ? 0 : -Math.sign(e.deltaY);
    if (steps === 0) return;
    const mods = readModifiers(e);
    // Each notch is its own history entry — no pause/resume needed,
    // the workspace setter creates one snapshot per call.
    cb.onValueStep(slot, steps, mods);
    e.preventDefault();
  }

  return {
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onWheel,
    isDragging,
  };
}
