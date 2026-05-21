<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';

/**
 * Reusable image-picker popup. Mounts to <body> via Teleport so it can
 * float over any parent card. The dropzone inside supports the full
 * trio of input methods:
 *
 *   - click → opens the native file browser via a hidden <input>
 *   - drop  → reads `dataTransfer.files[0]` directly
 *   - paste → consumes the first `image/*` clipboard item
 *
 * Paste is bound on `capture: true` so it fires before any page-level
 * Ctrl+V handler (e.g. the workspace-wide one in ImageExtractor.vue),
 * and propagation is stopped on consumption so the same file isn't
 * processed twice.
 */
const props = withDefaults(
  defineProps<{
    open: boolean;
    title?: string;
    accept?: string;
  }>(),
  {
    title: 'Choose image',
    accept: 'image/*',
  },
);

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void;
  (e: 'pick', file: File): void;
}>();

const fileInput = ref<HTMLInputElement | null>(null);
const dragOver = ref(false);

function close(): void {
  emit('update:open', false);
}

function pickFile(file: File): void {
  emit('pick', file);
  close();
}

function onClickPick(): void {
  fileInput.value?.click();
}

function onFileChange(e: Event): void {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (file) pickFile(file);
  // Reset so picking the same file again re-fires the event.
  target.value = '';
}

function onDragOver(e: DragEvent): void {
  e.preventDefault();
  dragOver.value = true;
}

function onDragLeave(): void {
  dragOver.value = false;
}

function onDrop(e: DragEvent): void {
  e.preventDefault();
  dragOver.value = false;
  const file = e.dataTransfer?.files?.[0];
  if (file) pickFile(file);
}

function onPaste(e: ClipboardEvent): void {
  const items = e.clipboardData?.items;
  if (!items) return;
  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const file = item.getAsFile();
      if (file) {
        e.preventDefault();
        e.stopImmediatePropagation();
        pickFile(file);
      }
      return;
    }
  }
}

function onKeyDown(e: KeyboardEvent): void {
  if (e.key === 'Escape') close();
}

// Bind global listeners only while the modal is open — keeps the
// page-level Ctrl+V handler free when no picker is on screen.
watch(
  () => props.open,
  (open) => {
    if (open) {
      document.addEventListener('paste', onPaste, true);
      document.addEventListener('keydown', onKeyDown);
    } else {
      document.removeEventListener('paste', onPaste, true);
      document.removeEventListener('keydown', onKeyDown);
      dragOver.value = false;
    }
  },
);

onBeforeUnmount(() => {
  document.removeEventListener('paste', onPaste, true);
  document.removeEventListener('keydown', onKeyDown);
});
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="picker-backdrop" @click.self="close">
      <div class="picker" role="dialog" :aria-label="title">
        <div class="picker-head">
          <span class="picker-title">{{ title }}</span>
          <button
            type="button"
            class="picker-close"
            aria-label="close"
            @click="close"
          >
            ×
          </button>
        </div>

        <div
          class="picker-zone"
          :class="{ dragover: dragOver }"
          @click="onClickPick"
          @dragover="onDragOver"
          @dragleave="onDragLeave"
          @drop="onDrop"
        >
          <input
            ref="fileInput"
            type="file"
            :accept="accept"
            hidden
            @change="onFileChange"
          />
          <svg
            viewBox="0 0 24 24"
            width="32"
            height="32"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            aria-hidden="true"
          >
            <path d="M12 16V4m0 0l-4 4m4-4l4 4M4 20h16" />
          </svg>
          <div class="picker-title-cta">
            Drop, paste, or click to upload an image
          </div>
          <div class="picker-hint mono">PNG · JPG · WebP · GIF · ⌃V to paste</div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.picker-backdrop {
  position: fixed;
  inset: 0;
  background: oklch(0 0 0 / 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 16px;
}

.picker {
  width: min(420px, 100%);
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  background: var(--bg-0);
  border: 1px solid var(--line);
  border-radius: 12px;
  box-shadow: 0 18px 40px rgba(0, 0, 0, 0.35);
}

.picker-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.picker-title {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--fg-2);
}

.picker-close {
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  line-height: 1;
  color: var(--fg-2);
  background: transparent;
  border: 1px solid var(--line-soft);
  border-radius: 6px;
  cursor: pointer;
}

.picker-close:hover {
  color: var(--fg-0);
  border-color: var(--accent-line);
}

.picker-zone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 20px;
  color: var(--fg-3);
  background: var(--bg-2);
  border: 2px dashed var(--line);
  border-radius: 10px;
  cursor: pointer;
  text-align: center;
}

.picker-zone:hover,
.picker-zone.dragover {
  color: var(--fg-0);
  border-color: var(--accent);
  background: var(--accent-soft);
}

.picker-title-cta {
  font-size: 12.5px;
  font-weight: 500;
}

.picker-hint {
  font-size: 10px;
  color: var(--fg-3);
}
</style>
