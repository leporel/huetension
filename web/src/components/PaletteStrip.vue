<script setup lang="ts">
import { ref } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import { useHarmonyApply } from '../composables/useHarmonyApply';

const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const { applyBaseHex } = useHarmonyApply();

const dragFrom = ref<number | null>(null);
const dragOver = ref<number | null>(null);

function onDragStart(e: DragEvent, i: number): void {
  if (workspace.colors[i]?.locked) {
    e.preventDefault();
    return;
  }
  dragFrom.value = i;
  // setData is required for Firefox; the actual content is unused.
  e.dataTransfer?.setData('text/plain', String(i));
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
}

function onDragOver(e: DragEvent, i: number): void {
  if (dragFrom.value === null) return;
  e.preventDefault();
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
  dragOver.value = i;
}

function onDrop(e: DragEvent, i: number): void {
  e.preventDefault();
  const from = dragFrom.value;
  dragFrom.value = null;
  dragOver.value = null;
  if (from === null || from === i) return;
  workspace.reorder(from, i);
}

function onDragEnd(): void {
  dragFrom.value = null;
  dragOver.value = null;
}

function onLockClick(i: number): void {
  workspace.toggleLock(i);
}

// Promote a color to the harmony base. applyBaseHex stores it as the
// base and regenerates — the color then lands at `harmony.baseIndex`
// (centre for Analogous/Mono, slot 0 otherwise). One undo step.
function setAsBase(i: number): void {
  if (harmony.baseIndex === i) return;
  const hex = workspace.colors[i]?.hex;
  if (hex) applyBaseHex(hex);
}
</script>

<template>
  <div class="strip">
    <div
      v-for="(slot, i) in workspace.colors"
      :key="i"
      class="cell"
      :class="{
        locked: slot.locked,
        selected: workspace.selectedSlot === i,
        'drag-source': dragFrom === i,
        'drag-target': dragOver === i && dragFrom !== i,
      }"
      :draggable="!slot.locked"
      @click="workspace.selectSlot(i)"
      @dragstart="onDragStart($event, i)"
      @dragover="onDragOver($event, i)"
      @drop="onDrop($event, i)"
      @dragend="onDragEnd"
    >
      <div class="sw" :style="{ background: slot.hex }">
        <button
          v-if="harmony.type !== 'custom'"
          type="button"
          class="base-btn"
          :class="{ active: harmony.baseIndex === i }"
          :title="harmony.baseIndex === i ? 'Base color' : 'Set as base'"
          :aria-label="
            harmony.baseIndex === i
              ? `Slot ${i + 1} is the base color`
              : `Set slot ${i + 1} as base color`
          "
          @click.stop="setAsBase(i)"
        >
          <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2.2">
            <circle cx="12" cy="12" r="8" />
            <circle cx="12" cy="12" r="2.6" fill="currentColor" stroke="none" />
          </svg>
        </button>
        <button
          type="button"
          class="lock"
          :title="slot.locked ? 'Unlock' : 'Lock'"
          :aria-label="slot.locked ? `Unlock slot ${i + 1}` : `Lock slot ${i + 1}`"
          @click.stop="onLockClick(i)"
        >
          <svg
            v-if="slot.locked"
            viewBox="0 0 24 24"
            width="11"
            height="11"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="5" y="11" width="14" height="9" rx="2" />
            <path d="M8 11V7a4 4 0 0 1 8 0v4" />
          </svg>
          <svg
            v-else
            viewBox="0 0 24 24"
            width="11"
            height="11"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="5" y="11" width="14" height="9" rx="2" />
            <path d="M8 11V7a4 4 0 0 1 7.4-2" />
          </svg>
        </button>
        <span class="hex mono">{{ slot.hex.toUpperCase() }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(80px, 1fr));
  gap: 6px;
}

.cell {
  cursor: grab;
}

.cell.locked {
  cursor: not-allowed;
}

.cell.drag-source {
  opacity: 0.4;
}

/* Selected slot — shared with the wheel handle. Declared before
   .drag-target so an in-flight drop outline takes precedence. */
.cell.selected .sw {
  outline: 2px solid var(--fg-0);
  outline-offset: 2px;
}

.cell.drag-target .sw {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.sw {
  position: relative;
  height: 78px;
  border-radius: 10px;
  overflow: hidden;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.06);
}

.cell.locked .sw {
  box-shadow:
    inset 0 0 0 1px oklch(1 0 0 / 0.06),
    0 0 0 2px var(--accent-line);
}

.hex {
  position: absolute;
  left: 8px;
  bottom: 6px;
  font-size: 10.5px;
  color: oklch(1 0 0 / 0.92);
  text-shadow: 0 1px 2px oklch(0 0 0 / 0.55);
  letter-spacing: 0.02em;
}

.lock {
  position: absolute;
  right: 6px;
  top: 6px;
  width: 20px;
  height: 20px;
  border-radius: 5px;
  background: oklch(0 0 0 / 0.35);
  border: 0;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: oklch(1 0 0 / 0.85);
  cursor: pointer;
}

.lock:hover {
  background: oklch(0 0 0 / 0.55);
  color: oklch(1 0 0 / 1);
}

.cell.locked .lock {
  background: var(--accent);
  color: #fff;
}

/* "Set as base" — top-left, mirrors .lock. The active state marks the
   cell that is currently the harmony base. */
.base-btn {
  position: absolute;
  left: 6px;
  top: 6px;
  width: 20px;
  height: 20px;
  border-radius: 5px;
  background: oklch(0 0 0 / 0.35);
  border: 0;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: oklch(1 0 0 / 0.85);
  cursor: pointer;
}

.base-btn:hover {
  background: oklch(0 0 0 / 0.55);
  color: oklch(1 0 0 / 1);
}

.base-btn.active {
  background: var(--accent);
  color: #fff;
  cursor: default;
}
</style>
