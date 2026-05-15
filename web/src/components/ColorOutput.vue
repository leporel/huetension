<script setup lang="ts">
import { computed, ref } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import {
  fromHex,
  hslString,
  hsvString,
  oklabString,
  oklchString,
  rgbString,
} from '../composables/useColor';

const props = defineProps<{
  /** Slot index to read from. Defaults to slot 0 (the base). */
  slot?: number;
}>();

const workspace = useWorkspaceStore();

const activeSlot = computed(() => props.slot ?? 0);

interface Row {
  label: string;
  value: string;
}

const rows = computed<Row[]>(() => {
  const c = workspace.colors[activeSlot.value];
  if (!c) return [];
  const rgb = fromHex(c.hex);
  return [
    { label: 'Hex', value: c.hex.toUpperCase() },
    { label: 'RGB', value: rgbString(rgb) },
    { label: 'HSL', value: hslString(rgb) },
    { label: 'HSV', value: hsvString(rgb) },
    { label: 'OkLab', value: oklabString(rgb) },
    { label: 'OkLCH', value: oklchString(rgb) },
  ];
});

// Track the last-copied row so the user gets a brief "copied" hint
// without a global toast. The hint clears after 1.2s.
const copiedLabel = ref<string | null>(null);
let resetTimer: number | null = null;

async function copy(row: Row): Promise<void> {
  try {
    await navigator.clipboard.writeText(row.value);
    copiedLabel.value = row.label;
    if (resetTimer !== null) window.clearTimeout(resetTimer);
    resetTimer = window.setTimeout(() => {
      copiedLabel.value = null;
      resetTimer = null;
    }, 1200);
  } catch {
    // Clipboard API can fail under HTTP / iframe / permission denial.
    // Falling back to execCommand is the historical workaround but
    // it's deprecated; we surface the failure quietly rather than
    // silently rebuild the implementation.
    copiedLabel.value = `${row.label} (copy failed)`;
  }
}
</script>

<template>
  <div class="output">
    <div v-for="r in rows" :key="r.label" class="val-row">
      <span class="lbl">{{ r.label }}</span>
      <span class="v mono">{{ r.value }}</span>
      <button
        type="button"
        class="copy"
        :title="`Copy ${r.label}`"
        :aria-label="`Copy ${r.label}`"
        @click="copy(r)"
      >
        <svg
          viewBox="0 0 24 24"
          width="12"
          height="12"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <rect x="9" y="9" width="12" height="12" rx="2" />
          <path d="M5 15V5a2 2 0 0 1 2-2h10" />
        </svg>
      </button>
    </div>
    <div v-if="copiedLabel" class="copied mono" aria-live="polite">
      copied · {{ copiedLabel }}
    </div>
  </div>
</template>

<style scoped>
.output {
  display: flex;
  flex-direction: column;
}

.val-row {
  display: grid;
  grid-template-columns: 56px 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 8px 4px;
  border-bottom: 1px dashed var(--line-soft);
}

.val-row:last-of-type {
  border-bottom: 0;
}

.val-row .lbl {
  font-size: 10.5px;
  color: var(--fg-3);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.val-row .v {
  font-size: 11px;
  color: var(--fg-0);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.copy {
  border: 0;
  background: transparent;
  color: var(--fg-3);
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 4px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.copy:hover {
  color: var(--fg-0);
  background: var(--bg-2);
}

.copied {
  margin-top: 6px;
  padding: 4px 8px;
  font-size: 10.5px;
  color: var(--accent);
  text-align: center;
  background: var(--accent-soft);
  border-radius: 6px;
}
</style>
