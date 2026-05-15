<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useWorkspaceStore } from '../stores/workspace';

// Inline-SVG icon names match the design source exactly
// (.prompts/exampleUI/example.html lines 473–528). Stays decoupled
// from lucide version pinning, which has shifted naming several
// times across releases.
type IconKey =
  | 'target'
  | 'grid'
  | 'image'
  | 'gradient'
  | 'shuffle'
  | 'picker'
  | 'contrast'
  | 'convert'
  | 'eye'
  | 'book'
  | 'list'
  | 'heart';

interface SideItem {
  key: string;
  label: string;
  icon: IconKey;
  to: string;
  count?: number;
}

interface SideGroup {
  label: string;
  items: SideItem[];
}

const route = useRoute();
const workspace = useWorkspaceStore();

const groups = computed<SideGroup[]>(() => [
  {
    label: 'Palette Generator',
    items: [
      { key: 'harmonies', label: 'Harmonies', icon: 'target', to: '/', count: workspace.size },
      { key: 'custom', label: 'Custom mix', icon: 'grid', to: '/' },
      { key: 'image', label: 'From image', icon: 'image', to: '/' },
      { key: 'gradient', label: 'From gradient', icon: 'gradient', to: '/tools' },
      { key: 'random', label: 'Random seed', icon: 'shuffle', to: '/' },
    ],
  },
  {
    label: 'Tools',
    items: [
      { key: 'picker', label: 'Color picker', icon: 'picker', to: '/' },
      { key: 'contrast', label: 'Contrast checker', icon: 'contrast', to: '/contrast' },
      { key: 'converter', label: 'Color converter', icon: 'convert', to: '/tools' },
      { key: 'blindness', label: 'Color blindness', icon: 'eye', to: '/tools' },
    ],
  },
  {
    label: 'Library',
    items: [
      { key: 'browse', label: 'Browse palettes', icon: 'book', to: '/library' },
      { key: 'categories', label: 'Categories', icon: 'list', to: '/library' },
    ],
  },
]);

function isActive(item: SideItem): boolean {
  const tabMatch = route.path === item.to;
  if (!tabMatch) return false;
  const m = (route.query.section as string | undefined) ?? '';
  return m === '' || m === item.key;
}
</script>

<template>
  <aside class="sidebar">
    <template v-for="g in groups" :key="g.label">
      <div class="side-label">{{ g.label }}</div>
      <RouterLink
        v-for="item in g.items"
        :key="item.key"
        :to="{ path: item.to, query: { section: item.key } }"
        class="side-item"
        :class="{ active: isActive(item) }"
      >
        <svg class="ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <template v-if="item.icon === 'target'">
            <circle cx="12" cy="12" r="9" />
            <circle cx="12" cy="12" r="3" />
          </template>
          <template v-else-if="item.icon === 'grid'">
            <rect x="3" y="3" width="7" height="7" />
            <rect x="14" y="3" width="7" height="7" />
            <rect x="3" y="14" width="7" height="7" />
            <rect x="14" y="14" width="7" height="7" />
          </template>
          <template v-else-if="item.icon === 'image'">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <circle cx="9" cy="9" r="2" />
            <path d="M21 15l-5-5L5 21" />
          </template>
          <template v-else-if="item.icon === 'gradient'">
            <path d="M3 12h18M3 6h18M3 18h18" />
          </template>
          <template v-else-if="item.icon === 'shuffle'">
            <path d="M5 5l14 14M19 5L5 19" />
          </template>
          <template v-else-if="item.icon === 'picker'">
            <path d="M2 22l5-1 13-13-4-4L3 17l-1 5z" />
          </template>
          <template v-else-if="item.icon === 'contrast'">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 3v18M3 12a9 9 0 0 1 9-9" />
          </template>
          <template v-else-if="item.icon === 'convert'">
            <path d="M4 7h16M4 12h16M4 17h10" />
          </template>
          <template v-else-if="item.icon === 'eye'">
            <path d="M12 2v6M12 22v-6M2 12h6M22 12h-6" />
          </template>
          <template v-else-if="item.icon === 'book'">
            <path d="M4 4h7v16H4zM13 4h7v10h-7z" />
          </template>
          <template v-else-if="item.icon === 'list'">
            <path d="M3 6h18M5 12h14M8 18h8" />
          </template>
          <template v-else-if="item.icon === 'heart'">
            <path d="M12 21s-7-4.5-7-11a4 4 0 0 1 7-2.7A4 4 0 0 1 19 10c0 6.5-7 11-7 11z" />
          </template>
        </svg>
        <span class="lbl">{{ item.label }}</span>
        <span v-if="item.count !== undefined" class="count">{{ item.count }}</span>
      </RouterLink>
    </template>

    <div class="side-foot">
      <div class="foot-name">huetension web</div>
      <div class="foot-sub">Phase 3 · S4b shell</div>
      <div class="track">
        <span class="fill" />
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  border-right: 1px solid var(--line-soft);
  padding: 16px 12px;
  background: var(--sidebar-bg);
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.side-label {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--fg-3);
  padding: 6px 10px;
  margin-top: 14px;
}

.side-label:first-child {
  margin-top: 0;
}

.side-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  border-radius: 8px;
  color: var(--fg-1);
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  text-decoration: none;
}

.side-item .ic {
  width: 14px;
  height: 14px;
  color: var(--fg-3);
  flex: 0 0 14px;
}

.side-item:hover {
  background: var(--bg-1);
  color: var(--fg-0);
}

.side-item.active {
  background: var(--accent-soft);
  color: var(--fg-0);
  box-shadow: inset 0 0 0 1px var(--accent-line);
}

.side-item.active .ic {
  color: var(--accent);
}

.side-item .lbl {
  flex: 1 1 auto;
  min-width: 0;
}

.side-item .count {
  margin-left: auto;
  font-size: 10.5px;
  color: var(--fg-3);
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--bg-2);
}

.side-foot {
  margin-top: 22px;
  padding: 12px;
  border: 1px dashed var(--line);
  border-radius: 12px;
  background: var(--bg-1);
}

.foot-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--fg-0);
}

.foot-sub {
  font-size: 10.5px;
  color: var(--fg-3);
  margin-top: 2px;
  margin-bottom: 8px;
}

.track {
  height: 4px;
  border-radius: 999px;
  background: var(--bg-3);
  position: relative;
  overflow: hidden;
}

.track .fill {
  position: absolute;
  inset: 0;
  width: 28%;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--accent), oklch(0.70 0.18 320));
}
</style>
