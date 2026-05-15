<script setup lang="ts">
import { computed } from 'vue';
import { useExtractionStore } from '../stores/extraction';
import { useWorkspaceStore } from '../stores/workspace';

/**
 * Right-side card showing each extracted color with a freq bar.
 * The swatch reads from the workspace so pin-dragged colors update
 * live; the freq stays pinned to the extraction snapshot (extraction
 * is the moment freqs are computed — they don't change as the user
 * edits hexes).
 *
 * Slots without a freq (i.e., workspace has been edited via
 * harmony/picker/library after extraction, or never extracted at
 * all) hide their bar — the strip becomes a plain swatch list.
 */

const extraction = useExtractionStore();
const workspace = useWorkspaceStore();

interface Row {
  hex: string;
  freq?: number | undefined;
}

const rows = computed<Row[]>(() => {
  const last = extraction.lastPalette;
  return workspace.colors.map((s, i) => ({
    hex: s.hex,
    freq: last[i]?.freq,
  }));
});

const stats = computed(() => extraction.metadata?.stats);
</script>

<template>
  <div class="extracted">
    <div v-if="rows.length === 0" class="empty mono">
      no extraction yet
    </div>
    <ul v-else class="rows">
      <li v-for="(r, i) in rows" :key="i" class="row">
        <span class="sw" :style="{ background: r.hex }" />
        <div class="meta">
          <span class="hex mono">{{ r.hex.toUpperCase() }}</span>
          <div v-if="r.freq !== undefined" class="bar">
            <span class="bar-fill" :style="{ width: (r.freq * 100) + '%', background: r.hex }" />
          </div>
          <span v-if="r.freq !== undefined" class="pct mono">
            {{ (r.freq * 100).toFixed(1) }}%
          </span>
        </div>
      </li>
    </ul>

    <div v-if="stats" class="stats mono">
      <span>{{ stats.valid_pixels.toLocaleString() }} px · {{ stats.duration_ms }} ms</span>
      <span v-if="extraction.metadata?.method">{{ extraction.metadata.method }}</span>
    </div>
  </div>
</template>

<style scoped>
.extracted {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.empty {
  padding: 18px 14px;
  text-align: center;
  border: 1px dashed var(--line-soft);
  border-radius: var(--r-md);
  color: var(--fg-3);
  font-size: 10.5px;
  letter-spacing: 0.03em;
}

.rows {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.row {
  display: grid;
  grid-template-columns: 28px 1fr;
  gap: 10px;
  align-items: center;
}

.sw {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.08);
}

.meta {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 4px 8px;
  align-items: center;
}

.hex {
  font-size: 10.5px;
  color: var(--fg-0);
  grid-column: 1 / span 2;
}

.bar {
  grid-column: 1;
  height: 4px;
  border-radius: 999px;
  background: var(--bg-3);
  position: relative;
  overflow: hidden;
}

.bar-fill {
  position: absolute;
  inset: 0;
  border-radius: 999px;
  opacity: 0.9;
}

.pct {
  grid-column: 2;
  font-size: 9.5px;
  color: var(--fg-2);
  justify-self: end;
}

.stats {
  display: flex;
  justify-content: space-between;
  padding: 6px 8px;
  border-top: 1px dashed var(--line-soft);
  font-size: 10px;
  color: var(--fg-3);
  letter-spacing: 0.04em;
}
</style>
