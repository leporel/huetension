/**
 * Wheel-handle gesture state machine. Maps PointerEvent /
 * WheelEvent into a small set of domain callbacks so the ColorWheel
 * component owns no DOM glue.
 *
 * Contract (S5a):
 *   - One undoable snapshot per gesture. `history.pause()` on
 *     pointerdown, `history.resume(true)` on pointerup. Wheel-scroll
 *     ticks are individual snapshots (each wheel notch = one undo
 *     step) — per slice plan §5.
 *   - Modifier keys are *live reads* off the current event, never
 *     state-machine states. Shift = hue-only, Alt = sat-only,
 *     Ctrl/Cmd = "unlock from harmony angle" (the consumer chooses
 *     what to do with the flag — gesture composable just forwards
 *     it).
 *   - Locked slots short-circuit before pointerdown is even captured.
 */

import { ref, type Ref } from 'vue';
import { modAngle } from './useColor';

export interface WheelGeometry {
  /** Center in component-local CSS pixels. */
  cx: number;
  cy: number;
  /** Outer ring radius (handles ride here at sat=1). */
  rOuter: number;
  /** Inner ring radius (handles ride here at sat=0). */
  rInner: number;
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
  /** New HSL hue in degrees [0, 360). */
  hue: number;
  /** Hue offset from gesture start, in degrees signed [-360, 360]. */
  hueDelta: number;
  /** New HSL saturation [0, 1]. */
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
  /** The slot's HSL at the moment of pointerdown. */
  getStartHSL: (slot: number) => { h: number; s: number; l: number };
  /** Called per pointermove with the resolved drag delta. */
  onDrag: (slot: number, delta: DragDelta) => void;
  /** Called per wheel notch (positive = up = lighter). */
  onLightnessStep: (slot: number, steps: number, mods: GestureModifiers) => void;
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
  geometry: WheelGeometry;
}

export interface UseHandleGestureBindings {
  /** Bind to each wheel-handle element. */
  onPointerDown: (e: PointerEvent, slot: number) => void;
  /** Bound at the wheel root — receives events post-capture. */
  onPointerMove: (e: PointerEvent) => void;
  onPointerUp: (e: PointerEvent) => void;
  /** Wheel scroll for lightness step. Bind on each handle. */
  onWheel: (e: WheelEvent, slot: number) => void;
  /** Public for the ColorWheel cursor styling. */
  isDragging: Ref<boolean>;
}

/**
 * pointerAngle returns the angle in degrees from (cx, cy) to (px, py),
 * with 0° at the 3 o'clock position and growing clockwise — matching
 * the HSL hue convention used by the rest of huetension (red at 0°,
 * cyan at 180°). Reads off the SVG's screen coordinates so Y points
 * down; the negate-Y trick keeps HSL hue going clockwise.
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
    const start = cb.getStartHSL(slot);
    state = {
      slot,
      startHue: start.h,
      startSat: start.s,
      startAngle: pointerAngle(e.clientX, e.clientY, geometry.cx, geometry.cy),
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
    const { cx, cy, rOuter, rInner } = state.geometry;
    const ang = pointerAngle(e.clientX, e.clientY, cx, cy);
    const rad = pointerRadius(e.clientX, e.clientY, cx, cy);

    // hueDelta is signed in [-180, 180] so a 1° drag near 360→0
    // wraparound produces a 1° delta, not a 359° one.
    let hueDelta = ang - state.startAngle;
    if (hueDelta > 180) hueDelta -= 360;
    if (hueDelta < -180) hueDelta += 360;

    // Map radius into [0, 1] saturation. The visible ring sits between
    // rInner and rOuter; saturation 1 at rOuter, 0 at rInner. Outside
    // the ring is clamped so the handle doesn't "fall off".
    const span = Math.max(1e-6, rOuter - rInner);
    let sat = (rad - rInner) / span;
    if (sat < 0) sat = 0;
    else if (sat > 1) sat = 1;

    // Modifier locks: Alt → hue-locked (only sat moves), Shift →
    // sat-locked (only hue moves). Both can co-exist (no-op).
    const newHue = mods.alt ? state.startHue : modAngle(state.startHue + hueDelta);
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
    cb.onLightnessStep(slot, steps, mods);
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
