<script setup lang="ts">
import { ref } from 'vue';
import { extractFile, extractUrl, type ExtractMethod } from '../api/extract';
import { useWorkspaceStore, type WorkspaceSlot } from '../stores/workspace';
import { useExtractionStore } from '../stores/extraction';
import type { ColorJSON } from '../api/types';

const workspace = useWorkspaceStore();
const extraction = useExtractionStore();

const urlInput = ref('');
const count = ref(5);
const method = ref<ExtractMethod>('soft');

const loading = ref(false);
const errorMsg = ref<string | null>(null);

const fileInput = ref<HTMLInputElement | null>(null);
const dragOver = ref(false);

function onPickClick() {
  fileInput.value?.click();
}

function onFileChange(e: Event) {
  const t = e.target as HTMLInputElement;
  const f = t.files?.[0];
  if (f) void runFileExtract(f);
  // Reset so the same file can be re-uploaded after extraction tweaks.
  t.value = '';
}

function onDrop(e: DragEvent) {
  e.preventDefault();
  dragOver.value = false;
  const f = e.dataTransfer?.files?.[0];
  if (f) void runFileExtract(f);
}

function onDragOver(e: DragEvent) {
  e.preventDefault();
  dragOver.value = true;
}

function onDragLeave() {
  dragOver.value = false;
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
}

async function runFileExtract(file: File) {
  errorMsg.value = null;
  loading.value = true;
  try {
    const blobUrl = URL.createObjectURL(file);
    const { w, h } = await loadImageNaturalSize(blobUrl);
    const res = await extractFile(file, { count: count.value, method: method.value });
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
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : String(err);
  } finally {
    loading.value = false;
  }
}

async function runUrlExtract() {
  if (!urlInput.value.trim()) return;
  errorMsg.value = null;
  loading.value = true;
  try {
    const res = await extractUrl(urlInput.value.trim(), {
      count: count.value,
      method: method.value,
    });
    const { w, h } = await loadImageNaturalSize(urlInput.value.trim());
    extraction.setImage({
      kind: 'url',
      url: urlInput.value.trim(),
      naturalWidth: w,
      naturalHeight: h,
    });
    extraction.setMetadata(res.metadata ?? null);
    extraction.setLastPalette(res.palette.colors);
    extraction.setLastFile(null);
    applyToWorkspace(res.palette.colors);
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : String(err);
  } finally {
    loading.value = false;
  }
}

async function rerun() {
  if (extraction.lastFile) {
    await runFileExtract(extraction.lastFile);
  } else if (extraction.image?.kind === 'url') {
    urlInput.value = extraction.image.url;
    await runUrlExtract();
  }
}
</script>

<template>
  <div class="extractor">
    <div
      class="dropzone"
      :class="{ dragover: dragOver, loading }"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
      @click="onPickClick"
    >
      <input
        ref="fileInput"
        type="file"
        accept="image/*"
        hidden
        @change="onFileChange"
      />
      <div v-if="!extraction.image" class="dz-empty">
        <svg
          viewBox="0 0 24 24"
          width="22"
          height="22"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
        >
          <path d="M12 16V4m0 0l-4 4m4-4l4 4M4 20h16" />
        </svg>
        <div class="dz-title">Drop an image or click to upload</div>
        <div class="dz-sub mono">PNG · JPG · WebP · GIF</div>
      </div>
      <div v-else class="dz-preview">
        <img :src="extraction.image.url" alt="extraction source" />
        <div class="dz-overlay mono">replace</div>
      </div>
    </div>

    <div class="controls">
      <label class="fld">
        <span>Method</span>
        <select v-model="method">
          <option value="soft">soft</option>
          <option value="freq">freq</option>
          <option value="kmeans">k-means</option>
          <option value="median">median-cut</option>
        </select>
      </label>
      <label class="fld">
        <span>Count <span class="count-val mono">{{ count }}</span></span>
        <input v-model.number="count" type="range" min="2" max="12" />
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

.dropzone {
  position: relative;
  height: 160px;
  border: 1px dashed var(--line);
  border-radius: var(--r-md);
  background: var(--bg-2);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  overflow: hidden;
}

.dropzone.dragover {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.dropzone.loading {
  opacity: 0.6;
  pointer-events: none;
}

.dz-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  color: var(--fg-2);
}

.dz-title {
  font-size: 12px;
  font-weight: 500;
}

.dz-sub {
  font-size: 10.5px;
  color: var(--fg-3);
}

.dz-preview {
  width: 100%;
  height: 100%;
  position: relative;
}

.dz-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.dz-overlay {
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
}

.controls {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.fld {
  display: flex;
  flex-direction: column;
  gap: 4px;
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
