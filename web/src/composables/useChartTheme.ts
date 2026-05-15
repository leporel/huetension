/**
 * Resolves the theme tokens an ECharts <canvas> needs.
 *
 * ECharts paints to a canvas and cannot read CSS custom properties, so
 * the `--fg`/`--line`/`--accent` tokens are resolved to concrete color
 * strings via getComputedStyle. The watch re-resolves on every
 * light/dark flip; `flush: 'post'` guarantees the theme store has
 * already written `<html data-theme>` before we read the new values.
 */

import { ref, watch } from 'vue';
import { useThemeStore } from '../stores/theme';

export interface ChartTheme {
  text: string;
  axisLine: string;
  splitLine: string;
  accent: string;
  good: string;
  bad: string;
  surface: string;
}

function resolveTheme(): ChartTheme {
  const cs = getComputedStyle(document.documentElement);
  const v = (name: string, fallback: string): string => {
    const got = cs.getPropertyValue(name).trim();
    return got || fallback;
  };
  return {
    text: v('--fg-2', '#9b96b4'),
    axisLine: v('--line', '#3a3550'),
    splitLine: v('--line-soft', '#2e2a42'),
    accent: v('--accent', '#6d5afe'),
    good: v('--good', '#3fb950'),
    bad: v('--bad', '#f85149'),
    surface: v('--bg-2', '#26223a'),
  };
}

export function useChartTheme() {
  const theme = useThemeStore();
  const ct = ref<ChartTheme>(resolveTheme());

  watch(
    () => theme.resolved,
    () => {
      ct.value = resolveTheme();
    },
    { flush: 'post' },
  );

  return ct;
}
