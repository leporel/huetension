<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { useLibraryStore } from '../stores/library';
import { useWorkspaceStore } from '../stores/workspace';
import type { LibraryPalette } from '../api/types';

/**
 * Curated palette browser. Loads the catalogue once via the library
 * store (ETag-cached), filters client-side by category / search / tag,
 * and loads a chosen palette into the workspace.
 *
 * `selectedId` is driven by the route (`/library/:id`) so a palette
 * deep-links and round-trips through the URL; selecting a card pushes
 * the route rather than holding local selection state.
 */

const props = defineProps<{ selectedId: string | null }>();

const router = useRouter();
const library = useLibraryStore();
const workspace = useWorkspaceStore();

onMounted(() => {
  void library.load();
});

// --- filters -----------------------------------------------------------

const activeCategory = ref('All');
const search = ref('');
const activeTag = ref('');

const categoryTabs = computed(() => [
  { name: 'All', count: library.palettes.length },
  ...library.categories.map((c) => ({ name: c.name, count: c.count })),
]);

const allTags = computed(() => {
  const set = new Set<string>();
  for (const p of library.palettes) {
    for (const t of p.tags ?? []) set.add(t);
  }
  return [...set].sort();
});

const filtered = computed<LibraryPalette[]>(() => {
  const q = search.value.trim().toLowerCase();
  const cat = activeCategory.value;
  const tag = activeTag.value;
  return library.palettes.filter((p) => {
    if (cat !== 'All' && !p.categories.includes(cat)) return false;
    if (tag && !(p.tags ?? []).includes(tag)) return false;
    if (q && !`${p.name} ${p.description ?? ''}`.toLowerCase().includes(q)) {
      return false;
    }
    return true;
  });
});

function pickTag(t: string): void {
  activeTag.value = activeTag.value === t ? '' : t;
}

// --- selection ---------------------------------------------------------

const detail = computed<LibraryPalette | null>(() =>
  props.selectedId ? (library.getById(props.selectedId) ?? null) : null,
);

const notFound = computed(
  () => props.selectedId !== null && library.loaded && detail.value === null,
);

function selectPalette(id: string): void {
  router.push(`/library/${encodeURIComponent(id)}`);
}

function clearSelection(): void {
  router.push('/library');
}

// --- load into workspace ----------------------------------------------

const confirming = ref(false);

// Switching the selected palette cancels a pending confirm so it can
// never apply to the wrong palette.
watch(
  () => props.selectedId,
  () => {
    confirming.value = false;
  },
);

function applyToWorkspace(): void {
  const p = detail.value;
  if (!p) return;
  // Library load is a wholesale palette replace (the new palette may
  // even differ in length), so locks are discarded — unlike S6's image
  // re-extraction, which preserves them because it keeps the same
  // workspace shape. The "Replace?" confirm + one-keypress undo are the
  // user's safety nets. One setColors call = one undo snapshot.
  workspace.setColors(p.colors.map((c) => ({ hex: c.hex.toUpperCase(), locked: false })));
  confirming.value = false;
}

function requestLoad(): void {
  if (!detail.value) return;
  // Unsaved edits present → confirm first; undo still recovers them.
  if (workspace.canUndo) {
    confirming.value = true;
  } else {
    applyToWorkspace();
  }
}
</script>

<template>
  <div class="lib">
    <div class="controls">
      <div class="tabs">
        <button
          v-for="t in categoryTabs"
          :key="t.name"
          type="button"
          class="tab"
          :class="{ active: activeCategory === t.name }"
          @click="activeCategory = t.name"
        >
          {{ t.name }}<span class="tc">{{ t.count }}</span>
        </button>
      </div>
      <div class="filters">
        <input
          v-model="search"
          class="search"
          type="search"
          placeholder="Search palettes…"
          aria-label="search palettes"
        />
        <select v-model="activeTag" class="tag-select" aria-label="filter by tag">
          <option value="">All tags</option>
          <option v-for="tg in allTags" :key="tg" :value="tg">#{{ tg }}</option>
        </select>
      </div>
    </div>

    <div v-if="library.loading && !library.loaded" class="note mono">
      loading catalogue…
    </div>
    <div v-else-if="library.error && !library.loaded" class="note err mono">
      {{ library.error }}
    </div>

    <div v-if="notFound" class="note mono">
      palette “{{ props.selectedId }}” not found —
      <button type="button" class="link" @click="clearSelection">clear</button>
    </div>

    <div v-if="detail" class="detail">
      <div class="detail-h">
        <div class="d-id">
          <div class="d-name">{{ detail.name }}</div>
          <div v-if="detail.description" class="d-desc">{{ detail.description }}</div>
        </div>
        <button type="button" class="x" aria-label="close detail" @click="clearSelection">
          ✕
        </button>
      </div>

      <div
        class="d-strip"
        :style="{ gridTemplateColumns: `repeat(${detail.colors.length}, 1fr)` }"
      >
        <div v-for="(c, i) in detail.colors" :key="i" class="d-cell">
          <span class="d-sw" :style="{ background: c.hex }" />
          <span class="d-hex mono">{{ c.hex.toUpperCase() }}</span>
        </div>
      </div>

      <div class="d-tags">
        <span v-for="cat in detail.categories" :key="cat" class="chip cat">{{ cat }}</span>
        <button
          v-for="tg in detail.tags ?? []"
          :key="tg"
          type="button"
          class="chip tag-chip"
          :class="{ on: activeTag === tg }"
          @click="pickTag(tg)"
        >
          #{{ tg }}
        </button>
      </div>

      <div class="d-foot">
        <span class="src mono">
          <template v-if="detail.source">{{ detail.source }}</template>
          <template v-if="detail.license"> · {{ detail.license }}</template>
        </span>
        <div class="d-actions">
          <template v-if="confirming">
            <span class="confirm mono">Replace current palette?</span>
            <button type="button" class="btn" @click="confirming = false">Cancel</button>
            <button type="button" class="btn accent" @click="applyToWorkspace">
              Replace
            </button>
          </template>
          <button v-else type="button" class="btn accent" @click="requestLoad">
            Load into workspace
          </button>
        </div>
      </div>
    </div>

    <div v-if="library.loaded" class="grid">
      <button
        v-for="p in filtered"
        :key="p.id"
        type="button"
        class="pal-card"
        :class="{ sel: p.id === props.selectedId }"
        @click="selectPalette(p.id)"
      >
        <div class="top">
          <span class="name">{{ p.name }}</span>
          <span v-if="p.categories[0]" class="tag">{{ p.categories[0] }}</span>
        </div>
        <div class="meta">{{ p.colors.length }} colors</div>
        <div
          class="strip"
          :style="{ gridTemplateColumns: `repeat(${p.colors.length}, 1fr)` }"
        >
          <span v-for="(c, i) in p.colors" :key="i" :style="{ background: c.hex }" />
        </div>
      </button>
      <div v-if="filtered.length === 0" class="note mono">
        no palettes match the filters
      </div>
    </div>
  </div>
</template>

<style scoped>
.lib {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.controls {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.tabs {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  flex: 1 1 auto;
}

.tab {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 10px;
  border-radius: 999px;
  font-size: 11px;
  color: var(--fg-2);
  border: 1px solid var(--line-soft);
  background: var(--bg-1);
  cursor: pointer;
  font-family: inherit;
}

.tab:hover {
  color: var(--fg-0);
}

.tab.active {
  background: var(--accent-soft);
  border-color: var(--accent-line);
  color: var(--fg-0);
}

.tab .tc {
  font-size: 9px;
  color: var(--fg-3);
}

.tab.active .tc {
  color: var(--accent);
}

.filters {
  display: flex;
  gap: 6px;
}

.search,
.tag-select {
  padding: 6px 9px;
  font-size: 11px;
  color: var(--fg-0);
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 8px;
}

.search {
  min-width: 180px;
}

.search:focus,
.tag-select:focus {
  outline: none;
  border-color: var(--accent-line);
}

.tag-select {
  cursor: pointer;
}

.note {
  padding: 16px;
  text-align: center;
  border: 1px dashed var(--line-soft);
  border-radius: var(--r-md);
  color: var(--fg-3);
  font-size: 10.5px;
}

.note.err {
  color: var(--bad);
  border-color: var(--bad-line);
}

.link {
  background: none;
  border: 0;
  color: var(--accent);
  cursor: pointer;
  font: inherit;
  text-decoration: underline;
}

/* --- selected palette detail --- */

.detail {
  border: 1px solid var(--accent-line);
  background: var(--accent-soft);
  border-radius: var(--r-lg);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-h {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.d-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--fg-0);
}

.d-desc {
  font-size: 11px;
  color: var(--fg-2);
  margin-top: 2px;
}

.x {
  flex: none;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  border: 1px solid var(--line-soft);
  background: var(--bg-1);
  color: var(--fg-2);
  cursor: pointer;
  font-size: 11px;
}

.x:hover {
  color: var(--fg-0);
}

.d-strip {
  display: grid;
  gap: 6px;
}

.d-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: center;
  min-width: 0;
}

.d-sw {
  width: 100%;
  height: 40px;
  border-radius: 7px;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.1);
}

.d-hex {
  font-size: 9px;
  color: var(--fg-2);
}

.d-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.chip {
  font-size: 9.5px;
  padding: 3px 8px;
  border-radius: 999px;
  border: 1px solid var(--line);
  font-family: inherit;
}

.chip.cat {
  color: var(--fg-1);
  background: var(--bg-1);
}

.chip.tag-chip {
  color: var(--fg-2);
  background: var(--bg-2);
  cursor: pointer;
}

.chip.tag-chip:hover {
  color: var(--fg-0);
}

.chip.tag-chip.on {
  color: var(--fg-0);
  border-color: var(--accent-line);
  background: var(--bg-3);
}

.d-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.src {
  font-size: 9.5px;
  color: var(--fg-3);
}

.d-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.confirm {
  font-size: 10.5px;
  color: var(--warn);
}

.btn {
  padding: 7px 12px;
  font-size: 11px;
  font-weight: 500;
  color: var(--fg-1);
  background: var(--bg-1);
  border: 1px solid var(--line-soft);
  border-radius: 8px;
  cursor: pointer;
}

.btn:hover {
  border-color: var(--accent-line);
  color: var(--fg-0);
}

.btn.accent {
  background: var(--accent-soft);
  border-color: var(--accent-line);
  color: var(--fg-0);
  font-weight: 600;
}

/* --- grid --- */

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 10px;
}

.pal-card {
  border: 1px solid var(--line-soft);
  border-radius: 11px;
  padding: 10px;
  background: var(--bg-2);
  cursor: pointer;
  text-align: left;
  font-family: inherit;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.pal-card:hover {
  border-color: var(--accent-line);
}

.pal-card.sel {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}

.pal-card .top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 6px;
}

.pal-card .name {
  font-size: 12px;
  font-weight: 600;
  color: var(--fg-0);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pal-card .tag {
  flex: none;
  font-size: 9px;
  color: var(--fg-2);
  border: 1px solid var(--line);
  padding: 1px 6px;
  border-radius: 999px;
  white-space: nowrap;
}

.pal-card .meta {
  font-size: 10px;
  color: var(--fg-3);
}

.pal-card .strip {
  display: grid;
  height: 38px;
  border-radius: 6px;
  overflow: hidden;
}

.grid .note {
  grid-column: 1 / -1;
}
</style>
