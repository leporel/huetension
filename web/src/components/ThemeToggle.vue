<script setup lang="ts">
import { computed } from 'vue';
import { Sun, Moon, Monitor } from 'lucide-vue-next';
import { useThemeStore } from '../stores/theme';

const theme = useThemeStore();

const icons = { system: Monitor, light: Sun, dark: Moon } as const;
const labels = {
  system: 'Theme: system (matches OS)',
  light: 'Theme: light',
  dark: 'Theme: dark',
} as const;

const currentIcon = computed(() => icons[theme.mode]);
const currentLabel = computed(() => labels[theme.mode]);

function onClick() {
  theme.cycle();
}
</script>

<template>
  <button
    type="button"
    class="theme-toggle"
    :title="currentLabel"
    :aria-label="currentLabel"
    @click="onClick"
  >
    <component :is="currentIcon" :size="14" />
  </button>
  <span class="sr-only" aria-live="polite">{{ currentLabel }}</span>
  <span class="mono mode-pill" aria-hidden="true">
    {{ theme.mode }}<span v-if="theme.mode === 'system'" class="mode-pill-suffix">
      · {{ theme.resolved }}
    </span>
  </span>
</template>

<style scoped>
.theme-toggle {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--line-soft);
  background: var(--bg-1);
  color: var(--fg-1);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.theme-toggle:hover {
  color: var(--fg-0);
  border-color: var(--line);
}

.mode-pill {
  color: var(--fg-3);
  padding: 3px 8px;
  border-radius: 999px;
  border: 1px solid var(--line-soft);
  background: var(--bg-1);
}

.mode-pill-suffix {
  margin-left: 4px;
  color: var(--fg-2);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
