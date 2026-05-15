<script setup lang="ts">
import { computed } from 'vue';
import CardFrame from '../components/CardFrame.vue';
import ColorWheel from '../components/ColorWheel.vue';
import HarmonySelector from '../components/HarmonySelector.vue';
import PaletteStrip from '../components/PaletteStrip.vue';
import ColorPicker from '../components/ColorPicker.vue';
import ColorOutput from '../components/ColorOutput.vue';
import ImageExtractor from '../components/ImageExtractor.vue';
import ImagePalettePicker from '../components/ImagePalettePicker.vue';
import ExtractedPalette from '../components/ExtractedPalette.vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useHarmonyStore } from '../stores/harmony';
import { useExtractionStore } from '../stores/extraction';
import { fromHex, toHSL, toOkLCH } from '../composables/useColor';

const workspace = useWorkspaceStore();
const harmony = useHarmonyStore();
const extraction = useExtractionStore();

const base = computed(() => {
  const c = workspace.colors[0];
  if (!c) return null;
  const rgb = fromHex(c.hex);
  return {
    hex: c.hex.toUpperCase(),
    hsl: toHSL(rgb),
    oklch: toOkLCH(rgb),
  };
});
</script>

<template>
  <div class="grid">
    <CardFrame title="Harmony" sub="tonal rules" class="col-1">
      <HarmonySelector />
      <div v-if="base" class="base-readout">
        <div class="kv">
          <span class="k">Hex</span>
          <span class="v mono">{{ base.hex }}</span>
        </div>
        <div class="kv">
          <span class="k">HSL</span>
          <span class="v mono">
            {{ Math.round(base.hsl.h) }}°
            {{ Math.round(base.hsl.s * 100) }}%
            {{ Math.round(base.hsl.l * 100) }}%
          </span>
        </div>
        <div class="kv">
          <span class="k">OkLCH</span>
          <span class="v mono">
            {{ base.oklch.L.toFixed(3) }}
            {{ base.oklch.C.toFixed(3) }}
            {{ Math.round(base.oklch.H) }}°
          </span>
        </div>
      </div>
      <div class="controls">
        <button class="ctrl-btn" type="button" :disabled="!workspace.canUndo" @click="workspace.undo">Undo</button>
        <button class="ctrl-btn" type="button" :disabled="!workspace.canRedo" @click="workspace.redo">Redo</button>
        <button class="ctrl-btn" type="button" @click="workspace.reset">Reset</button>
      </div>
    </CardFrame>

    <CardFrame title="Color wheel" :sub="`${harmony.type} · ${workspace.size} slots`" class="col-2">
      <div class="wheel-host">
        <ColorWheel />
      </div>
      <PaletteStrip />
    </CardFrame>

    <CardFrame title="Color picker" sub="HSV" class="col-3">
      <ColorPicker />
    </CardFrame>

    <CardFrame title="Values" sub="copy any format" class="col-1 row-2">
      <ColorOutput />
    </CardFrame>

    <CardFrame
      title="From image"
      :sub="extraction.image ? 'drag pins to resample' : 'drop · URL · pin overlay'"
      class="col-2 row-2 image-card"
    >
      <div class="image-card-body">
        <ImageExtractor class="image-controls" />
        <div class="image-viewport">
          <ImagePalettePicker />
        </div>
      </div>
    </CardFrame>

    <CardFrame title="Extracted palette" sub="frequency · saliency" class="col-3 row-2">
      <ExtractedPalette />
    </CardFrame>

    <CardFrame title="Random" sub="seed · constraints · S5b" class="col-1 row-3" />
    <CardFrame title="Workspace" sub="active palette" class="col-span-2 row-3">
      <PaletteStrip />
    </CardFrame>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: 280px 1fr 320px;
  grid-auto-rows: min-content;
  gap: 14px;
}

.col-1 { grid-column: 1 / span 1; }
.col-2 { grid-column: 2 / span 1; }
.col-3 { grid-column: 3 / span 1; }
.col-span-2 { grid-column: span 2; }

.row-2 { grid-row: 2; }
.row-3 { grid-row: 3; }

.wheel-host {
  display: flex;
  justify-content: center;
  margin-bottom: 14px;
}

.base-readout {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-md);
}

.kv {
  display: grid;
  grid-template-columns: 56px 1fr;
  gap: 10px;
  font-size: 11px;
}

.kv .k {
  color: var(--fg-3);
}

.kv .v {
  color: var(--fg-0);
}

.controls {
  display: flex;
  gap: 6px;
  margin-top: 10px;
}

.ctrl-btn {
  flex: 1 1 auto;
  padding: 6px 8px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  color: var(--fg-1);
  font-size: 11px;
  font-family: inherit;
  cursor: pointer;
}

.ctrl-btn:hover:not(:disabled) {
  color: var(--fg-0);
  border-color: var(--line);
}

.ctrl-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.image-card .image-card-body {
  display: grid;
  grid-template-columns: 220px 1fr;
  gap: 14px;
  align-items: stretch;
  min-height: 280px;
}

.image-viewport {
  position: relative;
  border-radius: var(--r-md);
  overflow: hidden;
  background: var(--bg-2);
}
</style>
