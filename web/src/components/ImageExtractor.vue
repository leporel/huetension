<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
  extractFile,
  extractUrl,
  type ExtractMethod,
  type ExtractOptions,
  type SoftPreset,
} from '../api/extract';
import { analyzeFile, analyzeUrl } from '../api/analyze';
import { useWorkspaceStore, type WorkspaceSlot } from '../stores/workspace';
import { useExtractionStore } from '../stores/extraction';
import { useHarmonyStore } from '../stores/harmony';
import type { ColorJSON } from '../api/types';
import ImagePickerModal from './ImagePickerModal.vue';

const workspace = useWorkspaceStore();
const extraction = useExtractionStore();
const harmony = useHarmonyStore();

const urlInput = ref('');
const count = ref(5);
const method = ref<ExtractMethod>('soft');
const preset = ref<SoftPreset>('default');

// All ten internal/extract methods, with display labels.
const METHODS: { value: ExtractMethod; label: string }[] = [
  { value: 'soft', label: 'soft' },
  { value: 'softk', label: 'soft-k' },
  { value: 'kmeans', label: 'k-means' },
  { value: 'okkmeans', label: 'ok-k-means' },
  { value: 'wkmeans', label: 'weighted k-means' },
  { value: 'mediancut', label: 'median-cut' },
  { value: 'octree', label: 'octree' },
  { value: 'wu', label: 'wu' },
  { value: 'popularity', label: 'popularity' },
  { value: 'dbscan', label: 'dbscan' },
];
const PRESETS: SoftPreset[] = [
  'default',
  'colorful',
  'bright',
  'muted',
  'deep',
  'dark',
];

// The soft preset applies only to the soft pipeline — the backend 400s
// if it is sent with any other method, so it is gated both in the UI
// (the select is hidden) and on the wire (omitted from currentOpts).
const isSoftMethod = computed(
  () => method.value === 'soft' || method.value === 'softk',
);

function currentOpts(): ExtractOptions {
  return {
    size: count.value,
    method: method.value,
    soft_preset: isSoftMethod.value ? preset.value : undefined,
  };
}

const loading = ref(false);
const errorMsg = ref<string | null>(null);
const pickerOpen = ref(false);

function onPickerPicked(file: File): void {
  void runFileExtract(file);
}

async function loadImageNaturalSize(url: string): Promise<{ w: number; h: number }> {
  // Cheap one-shot dimension read; the pixel sampler will load again
  // for getImageData. Two `Image()` instances per upload is fine — the
  // browser cache deduplicates the decode.
  const img = new Image();
  img.src = url;
  await img.decode().catch(() => {
    /* still resolves the size via onload below */
  });
  return { w: img.naturalWidth, h: img.naturalHeight };
}

function paletteToSlots(palette: ColorJSON[]): WorkspaceSlot[] {
  return palette.map((c) => {
    const slot: WorkspaceSlot = { hex: c.hex, locked: false };
    if (c.source) slot.source = { x: c.source.x, y: c.source.y };
    return slot;
  });
}

/** Replace the workspace with the extracted palette, but preserve any
 *  locked slots (both hex and source) at their original indices —
 *  matching the lock contract used by harmony regeneration. */
function applyToWorkspace(palette: ColorJSON[]): void {
  const cur = workspace.colors;
  const next: WorkspaceSlot[] = paletteToSlots(palette);
  for (let i = 0; i < next.length; i++) {
    const c = cur[i];
    if (c?.locked) {
      const slot: WorkspaceSlot = { hex: c.hex, locked: true };
      if (c.source) slot.source = { x: c.source.x, y: c.source.y };
      next[i] = slot;
    }
  }
  workspace.setColors(next);
  // An extracted palette is a wholesale replace — sync the harmony
  // store so the Count slider matches the new length and a later Count
  // change resizes the palette instead of regenerating a stale harmony
  // from slot 0. (Harmony state is session-only, not undone — the
  // extraction stays one undo snapshot.)
  harmony.setType('custom');
  harmony.setCount(next.length);
}

// runAnalyzeForFile / runAnalyzeForUrl populate the strip row under the
// image card. They run only on a *new* image (file pick, drop, URL fetch,
// paste) — not on the `[method, preset]` re-extract watcher. The strips
// depend on pixels alone, so re-running them per palette algorithm would
// just burn server cycles for an identical result.
function currentAnalyzeOpts() {
  return {
    space: extraction.analyzeSpace,
    distance_target: extraction.analyzeDistanceTarget,
  };
}

async function runAnalyzeForFile(file: File) {
  try {
    const res = await analyzeFile(file, currentAnalyzeOpts());
    extraction.setStrips(res.strips);
  } catch (err) {
    // Analysis is auxiliary — a failure here must not break extraction.
    // Surface as a console warning so debugging is possible, but leave
    // the (already-cleared) strip row empty.
    console.warn('analyze failed:', err);
  }
}

async function runAnalyzeForUrl(url: string) {
  try {
    const res = await analyzeUrl(url, currentAnalyzeOpts());
    extraction.setStrips(res.strips);
  } catch (err) {
    console.warn('analyze failed:', err);
  }
}

// Re-run analysis on the *current* image without re-extracting. Called
// from a watcher on the analyze controls (space, distance_target) so the
// strip row updates without disturbing the palette.
async function rerunAnalyze() {
  if (extraction.lastFile) {
    void runAnalyzeForFile(extraction.lastFile);
  } else if (extraction.image?.kind === 'url') {
    void runAnalyzeForUrl(extraction.image.url);
  }
}

// extractFileCore / extractUrlCore hold the common "send to /extract +
// update stores" logic. They never touch the strip row — the new-image
// entry points (runFileExtract / runUrlExtract) and the algorithm-rerun
// path call them with the same body, but only the entry points clear
// strips and fire /analyze. Keeping the analyze concern out of the core
// is what lets the [method, preset] watcher re-run extraction without
// re-running analysis.
async function extractFileCore(file: File) {
  const blobUrl = URL.createObjectURL(file);
  const { w, h } = await loadImageNaturalSize(blobUrl);
  const res = await extractFile(file, currentOpts());
  extraction.setImage({
    kind: 'file',
    url: blobUrl,
    naturalWidth: w,
    naturalHeight: h,
  });
  extraction.setMetadata(res.metadata ?? null);
  extraction.setLastPalette(res.palette.colors);
  extraction.setLastFile(file);
  applyToWorkspace(res.palette.colors);
}

async function extractUrlCore(url: string) {
  const res = await extractUrl(url, currentOpts());
  const { w, h } = await loadImageNaturalSize(url);
  extraction.setImage({
    kind: 'url',
    url,
    naturalWidth: w,
    naturalHeight: h,
  });
  extraction.setMetadata(res.metadata ?? null);
  extraction.setLastPalette(res.palette.colors);
  extraction.setLastFile(null);
  applyToWorkspace(res.palette.colors);
}

// Public entry — a fresh file pick / drop / paste. Clears the strip row
// and fires /analyze once the extract completes.
async function runFileExtract(file: File) {
  errorMsg.value = null;
  loading.value = true;
  extraction.clearStrips();
  try {
    await extractFileCore(file);
    void runAnalyzeForFile(file);
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : String(err);
  } finally {
    loading.value = false;
  }
}

// Public entry — a fresh URL fetch (user clicked Fetch or pressed Enter).
async function runUrlExtract() {
  const url = urlInput.value.trim();
  if (!url) return;
  errorMsg.value = null;
  loading.value = true;
  extraction.clearStrips();
  try {
    await extractUrlCore(url);
    void runAnalyzeForUrl(url);
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : String(err);
  } finally {
    loading.value = false;
  }
}

// Re-extract the currently loaded image with the latest method / preset
// selection. Skips the strip-row reset and /analyze call — the strips are
// pixel-driven and would be identical to the bands already on screen.
async function rerun() {
  errorMsg.value = null;
  loading.value = true;
  try {
    if (extraction.lastFile) {
      await extractFileCore(extraction.lastFile);
    } else if (extraction.image?.kind === 'url') {
      urlInput.value = extraction.image.url;
      await extractUrlCore(extraction.image.url);
    }
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : String(err);
  } finally {
    loading.value = false;
  }
}

// Changing the method or soft preset re-extracts the loaded image
// immediately — no separate Re-run click. A no-op when nothing is
// loaded (rerun has nothing to act on).
watch([method, preset], () => {
  void rerun();
});

// The analyse controls (space, distance target) live in ToolsView's
// strip row. Changing either re-renders the strips against the current
// image without touching the extraction palette.
watch(
  () => [extraction.analyzeSpace, extraction.analyzeDistanceTarget] as const,
  () => {
    if (extraction.image) void rerunAnalyze();
  },
);

// Ctrl+V / Cmd+V anywhere on the page extracts a clipboard image — a
// pasted screenshot goes straight into the dropzone. Text pastes are
// left untouched: only an image item in the clipboard is consumed.
function onPaste(e: ClipboardEvent): void {
  const items = e.clipboardData?.items;
  if (!items) return;
  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (item && item.kind === 'file' && item.type.startsWith('image/')) {
      const file = item.getAsFile();
      if (file) {
        e.preventDefault();
        void runFileExtract(file);
      }
      return;
    }
  }
}

onMounted(() => document.addEventListener('paste', onPaste));
onBeforeUnmount(() => document.removeEventListener('paste', onPaste));
</script>

<template>
  <div class="extractor">
    <template v-if="!extraction.image">
      <button
        type="button"
        class="pick-btn"
        :disabled="loading"
        @click="pickerOpen = true"
      >
        <svg
          viewBox="0 0 24 24"
          width="16"
          height="16"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          aria-hidden="true"
        >
          <path d="M12 16V4m0 0l-4 4m4-4l4 4M4 20h16" />
        </svg>
        <span>Load image</span>
      </button>
    </template>
    <button
      v-else
      type="button"
      class="pick-preview"
      :class="{ loading }"
      :disabled="loading"
      @click="pickerOpen = true"
    >
      <img :src="extraction.image.url" alt="extraction source" />
      <span class="pick-preview-overlay mono">replace</span>
    </button>

    <ImagePickerModal
      v-model:open="pickerOpen"
      title="From image"
      @pick="onPickerPicked"
    />

    <div class="controls">
      <label class="fld">
        <span>Method</span>
        <select v-model="method">
          <option v-for="m in METHODS" :key="m.value" :value="m.value">
            {{ m.label }}
          </option>
        </select>
      </label>
      <label class="fld">
        <span>Count <span class="count-val mono">{{ count }}</span></span>
        <input v-model.number="count" type="range" min="2" max="12" />
      </label>
      <label v-if="isSoftMethod" class="fld">
        <span>Soft preset</span>
        <select v-model="preset">
          <option v-for="p in PRESETS" :key="p" :value="p">{{ p }}</option>
        </select>
      </label>
    </div>

    <div class="url-row">
      <input
        v-model="urlInput"
        class="url-input mono"
        type="url"
        placeholder="…or paste image URL"
        @keydown.enter.prevent="runUrlExtract"
      />
      <button class="btn-primary" type="button" :disabled="loading" @click="runUrlExtract">
        Fetch
      </button>
    </div>

    <div class="actions">
      <button
        type="button"
        class="btn-ghost"
        :disabled="loading || (!extraction.lastFile && extraction.image?.kind !== 'url')"
        @click="rerun"
      >
        {{ loading ? 'Extracting…' : 'Re-run' }}
      </button>
    </div>

    <div v-if="errorMsg" class="error mono" role="alert">{{ errorMsg }}</div>
  </div>
</template>

<style scoped>
.extractor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* "From image" trigger — opens the ImagePickerModal. Two states: a
 * plain button when no image is loaded, and a preview-thumbnail button
 * (with a "replace" overlay on hover) once one is. Both share the
 * disabled-during-loading dim. */
.pick-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 10px 14px;
  font-family: inherit;
  font-size: 12px;
  font-weight: 500;
  color: var(--fg-0);
  background: var(--bg-2);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  cursor: pointer;
}

.pick-btn:hover:not(:disabled),
.pick-btn:focus-visible {
  border-color: var(--accent);
  background: var(--accent-soft);
  outline: none;
}

.pick-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pick-preview {
  position: relative;
  width: 100%;
  height: 160px;
  padding: 0;
  background: var(--bg-2);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  overflow: hidden;
  cursor: pointer;
}

.pick-preview:hover:not(:disabled),
.pick-preview:focus-visible {
  border-color: var(--accent);
  outline: none;
}

.pick-preview.loading {
  opacity: 0.6;
  pointer-events: none;
}

.pick-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.pick-preview-overlay {
  position: absolute;
  right: 8px;
  bottom: 8px;
  background: oklch(0 0 0 / 0.55);
  color: oklch(1 0 0 / 0.9);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 10.5px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  opacity: 0;
  transition: opacity 120ms ease;
}

.pick-preview:hover .pick-preview-overlay,
.pick-preview:focus-visible .pick-preview-overlay {
  opacity: 1;
}

/* Single column: the controls sit in a narrow (~220px) slot, too tight
   for two selects side by side once long method names like "weighted
   k-means" appear — stacking gives each control the full width. */
.controls {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.fld {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  font-size: 11px;
  color: var(--fg-2);
  font-weight: 500;
}

.fld select,
.url-input {
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  padding: 6px 8px;
  color: var(--fg-0);
  font-family: inherit;
  font-size: 12px;
}

/* Keep the select inside its column — a long option must not widen it. */
.fld select {
  width: 100%;
  min-width: 0;
}

.url-input {
  flex: 1 1 auto;
  min-width: 0;
}

.fld input[type='range'] {
  accent-color: var(--accent);
}

.count-val {
  margin-left: 6px;
  color: var(--fg-0);
}

.url-row {
  display: flex;
  gap: 6px;
}

.btn-primary,
.btn-ghost {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid transparent;
  font-family: inherit;
}

.btn-primary {
  background: var(--accent);
  color: #fff;
  box-shadow: 0 6px 18px -6px var(--accent);
}

.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-ghost {
  background: var(--bg-2);
  color: var(--fg-0);
  border-color: var(--line);
  width: 100%;
}

.btn-ghost:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.actions {
  display: flex;
  gap: 6px;
}

.error {
  padding: 6px 10px;
  background: oklch(0.70 0.18 25 / 0.16);
  color: var(--bad);
  border-radius: 6px;
  font-size: 10.5px;
}
</style>
