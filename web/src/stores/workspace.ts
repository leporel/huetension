import { defineStore } from 'pinia';
import { useRefHistory, useStorage } from '@vueuse/core';
import { computed } from 'vue';

/**
 * Active palette state shared across every feature card.
 *
 * - `colors` is the live palette. Wheel handles, image pins, library
 *   load, and randomise all write here.
 * - `locks[i]` mirrors `colors[i]` — when true the slot ignores wheel
 *   gestures and image-pin drags (S5/S6 read this; S4b only owns the
 *   plumbing).
 * - History captures one snapshot per *setter call*. S5 batches gestures
 *   so a pointerdown→pointermove→pointerup is one undoable step.
 * - The whole state survives reloads via `useStorage`. We clear the
 *   history right after construction so the first undo lands on the
 *   persisted state, not the default seed.
 */

/**
 * Image-space pin coordinate, 0..1 normalised, origin top-left.
 * Mirrors `color.Source` on the wire (S2). Present only for slots
 * sourced from an image extraction; harmony/random/library slots
 * have no source.
 */
export interface SlotSource {
  x: number;
  y: number;
}

export interface WorkspaceSlot {
  hex: string;
  locked: boolean;
  source?: SlotSource;
}

export interface WorkspaceState {
  colors: WorkspaceSlot[];
}

const STORAGE_KEY = 'huetension:workspace';

const DEFAULT_STATE: WorkspaceState = {
  colors: [
    { hex: '#6D5AFE', locked: false },
    { hex: '#FFA94D', locked: false },
    { hex: '#FFD43B', locked: false },
    { hex: '#51CF66', locked: false },
    { hex: '#22D3EE', locked: false },
  ],
};

function cloneSlot(c: WorkspaceSlot): WorkspaceSlot {
  const out: WorkspaceSlot = { hex: c.hex, locked: c.locked };
  if (c.source) out.source = { x: c.source.x, y: c.source.y };
  return out;
}

function cloneState(s: WorkspaceState): WorkspaceState {
  return { colors: s.colors.map(cloneSlot) };
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const state = useStorage<WorkspaceState>(STORAGE_KEY, cloneState(DEFAULT_STATE), undefined, {
    mergeDefaults: true,
  });

  // History tracks the full state object — one undo step per setter
  // call. Deep watch is intentional: colors[i].hex / .locked changes
  // both bump history.
  const history = useRefHistory(state, { deep: true, capacity: 50 });

  // Drop the snapshot that captured the default seed before storage
  // hydrated. After this, history.canUndo flips on as soon as the user
  // makes a real change.
  history.clear();

  const colors = computed(() => state.value.colors);
  const size = computed(() => state.value.colors.length);

  function setColors(next: WorkspaceSlot[]) {
    state.value = { colors: next.map(cloneSlot) };
  }

  function setHex(i: number, hex: string) {
    const slot = state.value.colors[i];
    if (!slot || slot.locked) return;
    const copy = cloneState(state.value);
    const target = copy.colors[i];
    if (!target) return;
    target.hex = hex;
    state.value = copy;
  }

  /**
   * Image-pin path: update slot's source + hex in one snapshot.
   * Locked slots ignore the call. Used by ImagePalettePicker so
   * a pin drag undoes the color and the pin position together.
   */
  function setSlotPin(i: number, hex: string, source: SlotSource) {
    const slot = state.value.colors[i];
    if (!slot || slot.locked) return;
    const copy = cloneState(state.value);
    const target = copy.colors[i];
    if (!target) return;
    target.hex = hex;
    target.source = { x: source.x, y: source.y };
    state.value = copy;
  }

  function toggleLock(i: number) {
    const slot = state.value.colors[i];
    if (!slot) return;
    const copy = cloneState(state.value);
    const target = copy.colors[i];
    if (!target) return;
    target.locked = !target.locked;
    state.value = copy;
  }

  function reorder(from: number, to: number) {
    if (from === to) return;
    const copy = cloneState(state.value);
    const moved = copy.colors.splice(from, 1)[0];
    if (!moved) return;
    copy.colors.splice(to, 0, moved);
    state.value = copy;
  }

  function reset() {
    state.value = cloneState(DEFAULT_STATE);
  }

  // History pause/resume adapter for gesture composables. Wheel pauses
  // on pointerdown, resumes-with-commit on pointerup so each gesture
  // lands as exactly one undo step regardless of pointermove count.
  const historyAdapter = {
    pause: () => history.pause(),
    resume: (commit?: boolean) => history.resume(commit),
  };

  return {
    state,
    colors,
    size,
    canUndo: history.canUndo,
    canRedo: history.canRedo,
    undo: history.undo,
    redo: history.redo,
    setColors,
    setHex,
    setSlotPin,
    toggleLock,
    reorder,
    reset,
    historyAdapter,
  };
});
