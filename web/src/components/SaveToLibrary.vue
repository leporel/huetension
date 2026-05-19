<script setup lang="ts">
import { computed, ref } from 'vue';
import { useWorkspaceStore } from '../stores/workspace';
import { useLibraryStore } from '../stores/library';
import { library as libraryApi } from '../api';

/**
 * Save the current workspace palette into the on-disk library via
 * POST /library/palette. Every saved palette is auto-filed under the
 * "Saved" category by the server; the form offers one optional extra
 * category on top. A read-only server rejects the save — its error
 * message is surfaced inline rather than the form being pre-disabled,
 * since the server's posture is not known to the SPA up front.
 */
const workspace = useWorkspaceStore();
const libraryStore = useLibraryStore();

const name = ref('');
const category = ref('');
const tags = ref('');
const description = ref('');

const saving = ref(false);
const error = ref<string | null>(null);
const savedName = ref<string | null>(null);

const colors = computed(() => workspace.colors.map((c) => c.hex));
const canSave = computed(
  () => name.value.trim().length > 0 && colors.value.length > 0,
);

async function save(): Promise<void> {
  if (!canSave.value || saving.value) return;
  saving.value = true;
  error.value = null;
  savedName.value = null;
  const tagList = tags.value
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);
  try {
    const res = await libraryApi.save({
      name: name.value.trim(),
      colors: colors.value,
      categories: category.value.trim() ? [category.value.trim()] : undefined,
      tags: tagList.length > 0 ? tagList : undefined,
      description: description.value.trim() || undefined,
    });
    savedName.value = res.palette.name;
    // Pull the catalogue fresh so the new palette — and the "Saved"
    // category if it is new — appear in the browser below at once.
    await libraryStore.refresh();
    name.value = '';
    category.value = '';
    tags.value = '';
    description.value = '';
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div class="save">
    <div class="preview" aria-hidden="true">
      <span
        v-for="(hex, i) in colors"
        :key="i"
        class="sw"
        :style="{ background: hex }"
        :title="hex"
      />
    </div>
    <p class="hint">
      The current workspace palette — {{ colors.length }} colors, filed
      under the <strong>Saved</strong> category.
    </p>

    <div class="fields">
      <label class="fld name">
        <span>Name</span>
        <input
          v-model="name"
          type="text"
          placeholder="My palette"
          spellcheck="false"
          autocomplete="off"
          @keydown.enter.prevent="save"
        />
      </label>
      <label class="fld">
        <span>Extra category <em>optional</em></span>
        <input
          v-model="category"
          type="text"
          placeholder="e.g. Warm"
          spellcheck="false"
          autocomplete="off"
        />
      </label>
      <label class="fld">
        <span>Tags <em>comma-separated</em></span>
        <input
          v-model="tags"
          type="text"
          placeholder="warm, sunset"
          spellcheck="false"
          autocomplete="off"
        />
      </label>
      <label class="fld">
        <span>Description <em>optional</em></span>
        <input
          v-model="description"
          type="text"
          placeholder="one-line description"
          spellcheck="false"
          autocomplete="off"
        />
      </label>
    </div>

    <div class="foot">
      <button
        type="button"
        class="btn"
        :disabled="!canSave || saving"
        @click="save"
      >
        {{ saving ? 'Saving…' : 'Save to library' }}
      </button>
      <span v-if="savedName" class="ok mono">saved “{{ savedName }}”</span>
      <span v-if="error" class="err mono" role="alert">{{ error }}</span>
    </div>
  </div>
</template>

<style scoped>
.save {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.preview {
  display: flex;
  height: 34px;
  border-radius: var(--r-md);
  overflow: hidden;
  box-shadow: inset 0 0 0 1px var(--line-soft);
}

.sw {
  flex: 1 1 0;
  min-width: 0;
}

.hint {
  margin: 0;
  font-size: 11px;
  color: var(--fg-3);
}

.hint strong {
  color: var(--fg-1);
  font-weight: 600;
}

.fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
}

/* The name is the only required field — give it the first row alone so
   it reads as primary. */
.fld.name {
  grid-column: 1 / -1;
}

.fld {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 11px;
  font-weight: 500;
  color: var(--fg-2);
}

.fld em {
  color: var(--fg-3);
  font-weight: 400;
  font-style: normal;
}

.fld input {
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 7px;
  padding: 7px 9px;
  color: var(--fg-0);
  font-family: inherit;
  font-size: 12px;
}

.fld input:focus {
  outline: none;
  border-color: var(--accent);
}

.foot {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.btn {
  padding: 8px 14px;
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  color: var(--fg-0);
  background: var(--accent-soft);
  border: 1px solid var(--accent-line);
  border-radius: 8px;
}

.btn:hover:not(:disabled) {
  border-color: var(--accent);
}

.btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.ok {
  font-size: 11px;
  color: var(--accent);
}

.err {
  font-size: 11px;
  color: var(--bad);
}
</style>
