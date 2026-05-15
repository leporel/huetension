import { defineStore } from 'pinia';
import { computed, watchEffect } from 'vue';
import { usePreferredDark, useStorage } from '@vueuse/core';

export type ThemeMode = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

const STORAGE_KEY = 'huetension:theme';

/**
 * Theme store: tri-state mode (light / dark / system) persisted in
 * localStorage; resolved theme reactively follows OS preference when
 * mode is 'system'. The resolved value is mirrored onto
 * `<html data-theme>` so component CSS can read the right token set
 * without touching the store.
 */
export const useThemeStore = defineStore('theme', () => {
  const mode = useStorage<ThemeMode>(STORAGE_KEY, 'system');
  const prefersDark = usePreferredDark();

  const resolved = computed<ResolvedTheme>(() => {
    if (mode.value === 'light') return 'light';
    if (mode.value === 'dark') return 'dark';
    return prefersDark.value ? 'dark' : 'light';
  });

  watchEffect(() => {
    if (typeof document === 'undefined') return;
    document.documentElement.setAttribute('data-theme', resolved.value);
  });

  function setMode(next: ThemeMode) {
    mode.value = next;
  }

  function cycle() {
    const order: ThemeMode[] = ['system', 'light', 'dark'];
    const i = order.indexOf(mode.value);
    mode.value = order[(i + 1) % order.length] ?? 'system';
  }

  return { mode, resolved, setMode, cycle };
});
