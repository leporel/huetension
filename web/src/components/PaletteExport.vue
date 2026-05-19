<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';
import { watchDebounced } from '@vueuse/core';
import { exportPalette } from '../api';
import type { ExportFormat } from '../api/exportPalette';
import { useWorkspaceStore } from '../stores/workspace';

// Export the workspace palette in any exporter format via POST /export.
// Text formats render into a <pre>; binary formats (PNG / JPEG) come back
// base64-encoded and are shown as a thumbnail.
const FORMATS: { value: ExportFormat; label: string }[] = [
  { value: 'json', label: 'JSON' },
  { value: 'css', label: 'CSS' },
  { value: 'scss', label: 'SCSS' },
  { value: 'less', label: 'Less' },
  { value: 'tailwind', label: 'Tailwind' },
  { value: 'txt', label: 'Plain' },
  { value: 'gpl', label: 'GIMP' },
  { value: 'ggr', label: 'GGR' },
  { value: 'svg', label: 'SVG' },
  { value: 'png', label: 'PNG' },
  { value: 'jpeg', label: 'JPEG' },
  { value: 'ase', label: 'ASE' },
  { value: 'aco', label: 'ACO' },
];

const workspace = useWorkspaceStore();

const format = ref<ExportFormat>('css');
const name = ref('palette');
// Shade-scale count for Tailwind: 0 = flat shape; >0 expands each color.
const shades = ref(0);

const content = ref('');
const filename = ref('');
// Object URL of the decoded binary output; null for text formats. The
// preview <img> and the Download link both read it.
const blobUrl = ref<string | null>(null);
const loading = ref(false);
const errorMsg = ref<string | null>(null);
const copied = ref(false);

const isBinary = computed(() => blobUrl.value !== null);
// Image binaries (png/jpeg) get a thumbnail preview; opaque binaries
// (Adobe ase/aco) only offer a download.
const isImage = computed(
  () => isBinary.value && (format.value === 'png' || format.value === 'jpeg'),
);
const hasOutput = computed(() => content.value !== '' || blobUrl.value !== null);

/** Swap in a new binary preview URL, revoking the previous one. */
function setBlobUrl(blob: Blob | null): void {
  if (blobUrl.value) URL.revokeObjectURL(blobUrl.value);
  blobUrl.value = blob ? URL.createObjectURL(blob) : null;
}

async function generate(): Promise<void> {
  const fmt = format.value;
  const colors = workspace.colors.map((c) => c.hex);
  if (colors.length === 0) return;
  const palName = name.value.trim() || 'palette';
  errorMsg.value = null;
  loading.value = true;
  try {
    const res = await exportPalette.general({
      format: fmt,
      colors,
      name: palName,
      // shades only refines Tailwind; the endpoint ignores it elsewhere.
      shades: fmt === 'tailwind' && shades.value > 0 ? shades.value : undefined,
    });
    filename.value = res.filename;
    if (res.encoding === 'base64') {
      content.value = '';
      setBlobUrl(exportPalette.decodeBinary(res));
    } else {
      content.value = res.content;
      setBlobUrl(null);
    }
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : String(err);
    content.value = '';
    filename.value = '';
    setBlobUrl(null);
  } finally {
    loading.value = false;
  }
}

// Auto-regenerate on palette / format / name / shades change — debounced
// so a wheel drag or fast typing doesn't spam the endpoint.
watchDebounced(
  [format, name, shades, () => workspace.colors],
  generate,
  { debounce: 300, immediate: true, deep: true },
);

onBeforeUnmount(() => setBlobUrl(null));

async function copy(): Promise<void> {
  if (!content.value) return;
  try {
    await navigator.clipboard.writeText(content.value);
    copied.value = true;
    window.setTimeout(() => {
      copied.value = false;
    }, 1200);
  } catch {
    errorMsg.value = 'clipboard unavailable';
  }
}

function download(): void {
  if (!hasOutput.value) return;
  // Binary output already has an object URL; text output gets a throwaway.
  const href = blobUrl.value
    ?? URL.createObjectURL(new Blob([content.value], { type: 'text/plain;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = href;
  a.download = filename.value || 'palette.txt';
  a.click();
  if (!blobUrl.value) URL.revokeObjectURL(href);
}
</script>

<template>
  <div class="export">
    <div class="fmt-row" role="group" aria-label="Export format">
      <button
        v-for="f in FORMATS"
        :key="f.value"
        type="button"
        class="fmt-btn"
        :class="{ active: format === f.value }"
        :aria-pressed="format === f.value"
        @click="format = f.value"
      >
        {{ f.label }}
      </button>
    </div>

    <div class="fields">
      <label class="name-fld">
        <span>Name</span>
        <input
          v-model="name"
          type="text"
          spellcheck="false"
          autocomplete="off"
          placeholder="palette"
        />
      </label>

      <!-- Shade-scale count is meaningful for Tailwind only. -->
      <label v-if="format === 'tailwind'" class="name-fld shades-fld">
        <span>Shades <em>{{ shades === 0 ? 'flat' : shades }}</em></span>
        <input
          v-model.number="shades"
          type="range"
          min="0"
          max="10"
          aria-label="tailwind shade-scale count"
        />
      </label>
    </div>

    <img
      v-if="isImage"
      class="thumb"
      :src="blobUrl ?? ''"
      alt="exported palette preview"
    />
    <div v-else-if="isBinary" class="output binary-note">
      Binary swatch file ready — {{ filename }}
    </div>
    <pre v-else class="output mono" :class="{ placeholder: !content }">{{
      content || (loading ? 'Generating…' : 'No output')
    }}</pre>

    <div v-if="errorMsg" class="error mono" role="alert">{{ errorMsg }}</div>

    <div class="actions">
      <button
        v-if="!isBinary"
        type="button"
        class="btn"
        :disabled="!content"
        @click="copy"
      >
        {{ copied ? 'Copied!' : 'Copy' }}
      </button>
      <button type="button" class="btn" :disabled="!hasOutput" @click="download">
        Download
      </button>
    </div>
  </div>
</template>

<style scoped>
.export {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.fmt-row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.fmt-btn {
  padding: 5px 9px;
  font-size: 11px;
  font-weight: 500;
  font-family: inherit;
  color: var(--fg-2);
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  cursor: pointer;
}

.fmt-btn:hover {
  color: var(--fg-0);
  border-color: var(--accent-line);
}

.fmt-btn.active {
  background: var(--accent-soft);
  color: var(--fg-0);
  border-color: var(--accent-line);
}

.fields {
  display: flex;
  gap: 10px;
}

.name-fld {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 11px;
  color: var(--fg-2);
  font-weight: 500;
  flex: 1 1 auto;
}

.name-fld em {
  color: var(--fg-3);
  font-weight: 400;
  font-style: normal;
}

.name-fld input[type='text'] {
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  padding: 6px 8px;
  color: var(--fg-0);
  font-family: inherit;
  font-size: 12px;
}

.name-fld input[type='text']:focus {
  outline: none;
  border-color: var(--accent);
}

.shades-fld {
  flex: 0 0 130px;
}

.shades-fld input[type='range'] {
  accent-color: var(--accent);
  margin-top: 5px;
}

.output {
  margin: 0;
  height: 180px;
  overflow: auto;
  padding: 10px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-md);
  font-size: 11px;
  line-height: 1.5;
  color: var(--fg-1);
  white-space: pre;
  tab-size: 2;
}

.output.placeholder {
  color: var(--fg-3);
}

.thumb {
  height: 180px;
  width: 100%;
  object-fit: contain;
  padding: 10px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-md);
  image-rendering: pixelated;
}

.thumb.placeholder {
  opacity: 0;
}

.binary-note {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--fg-2);
  font-size: 11.5px;
}

.actions {
  display: flex;
  gap: 6px;
}

.btn {
  flex: 1 1 auto;
  padding: 7px 10px;
  border-radius: 8px;
  font-size: 11.5px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  background: var(--bg-2);
  color: var(--fg-0);
  border: 1px solid var(--line);
}

.btn:hover:not(:disabled) {
  border-color: var(--accent);
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.error {
  padding: 6px 10px;
  background: oklch(0.7 0.18 25 / 0.16);
  color: var(--bad);
  border-radius: 6px;
  font-size: 10.5px;
}
</style>
