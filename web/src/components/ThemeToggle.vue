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
  <div class="theme-toggle-wrap">
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
    <!-- Decorative caption under the button. Absolutely positioned so its
         varying length ("dark" vs "system · dark") never shifts the
         button left or right. -->
    <span class="mono mode-caption" aria-hidden="true">
      {{ theme.mode }}<span v-if="theme.mode === 'system'" class="mode-caption-suffix">
        · {{ theme.resolved }}
      </span>
    </span>
  </div>
</template>

<style scoped>
.theme-toggle-wrap {
  position: relative;
  display: inline-flex;
  flex-direction: column;
  align-items: flex-end;
  /* 32px button + 2px gap + caption — fixed so the topbar centres the
     whole control (caption included) instead of clipping it. */
  height: 44px;
}

.theme-toggle {
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
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

.mode-caption {
  position: absolute;
  top: 34px; /* 32px button + 2px gap */
  right: 0; /* right-anchored: the caption grows leftward, never past the topbar edge */
  font-size: 9.5px;
  line-height: 1;
  letter-spacing: 0.02em;
  color: var(--fg-3);
  white-space: nowrap;
  pointer-events: none;
}

.mode-caption-suffix {
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
