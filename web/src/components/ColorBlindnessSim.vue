<script setup lang="ts">
import { computed } from 'vue';
import { useReviewStore } from '../stores/review';
import { fromHex, toHex, type RGB } from '../composables/useColor';
import {
  BLINDNESS_KINDS,
  BLINDNESS_LABELS,
  simulateBlindness,
} from '../composables/useBlindness';

/**
 * Color-blindness simulation strips: Normal plus the four Brettel–Viénot
 * variants. Reads the shared *review palette* (`useReviewStore`) so a
 * Contrast-checker lightness tweak or applied fix shows here too — the
 * matrix multiply is cheap enough to skip the /api/v1/blindness/simulate
 * roundtrip (see useBlindness).
 */

const review = useReviewStore();

function safeParse(hex: string): RGB {
  try {
    return fromHex(hex);
  } catch {
    return { r: 0, g: 0, b: 0 };
  }
}

const base = computed(() => review.colors.map((hex) => safeParse(hex)));

interface Strip {
  key: string;
  label: string;
  colors: string[];
}

const strips = computed<Strip[]>(() => {
  const normal = base.value;
  return [
    { key: 'normal', label: 'Normal', colors: normal.map(toHex) },
    ...BLINDNESS_KINDS.map((k) => ({
      key: k,
      label: BLINDNESS_LABELS[k],
      colors: normal.map((c) => toHex(simulateBlindness(c, k))),
    })),
  ];
});

const cols = computed(() => Math.max(1, base.value.length));
const empty = computed(() => base.value.length === 0);
</script>

<template>
  <div class="cvd">
    <div v-if="empty" class="empty mono">no palette</div>
    <template v-else>
      <div
        v-for="strip in strips"
        :key="strip.key"
        class="strip-row"
        :class="{ normal: strip.key === 'normal' }"
      >
        <span class="label">{{ strip.label }}</span>
        <div class="strip" :style="{ gridTemplateColumns: `repeat(${cols}, 1fr)` }">
          <i
            v-for="(c, i) in strip.colors"
            :key="i"
            :style="{ background: c }"
            :title="c"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.cvd {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.empty {
  color: var(--fg-3);
  border: 1px dashed var(--line-soft);
  border-radius: var(--r-md);
  padding: 14px;
  text-align: center;
  font-size: 10.5px;
}

.strip-row {
  display: grid;
  grid-template-columns: 96px 1fr;
  gap: 10px;
  align-items: center;
}

.label {
  font-size: 11px;
  color: var(--fg-2);
}

.strip-row.normal .label {
  color: var(--fg-0);
  font-weight: 600;
}

.strip {
  display: grid;
  height: 30px;
  border-radius: 6px;
  overflow: hidden;
  box-shadow: inset 0 0 0 1px oklch(1 0 0 / 0.06);
}

.strip i {
  display: block;
}
</style>
