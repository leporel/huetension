<script setup lang="ts">
import { ref } from 'vue';
import CardFrame from '../components/CardFrame.vue';

const tabs = ['All', 'Popular', 'UI / UX', 'Brand', 'Nature', 'Pastel', 'Vibrant', 'Dark', 'Light', 'Vintage', 'Mono'];
const active = ref('All');
</script>

<template>
  <div class="grid">
    <CardFrame title="Library" sub="curated catalogue" class="col-span-full">
      <div class="lib-tabs">
        <button
          v-for="t in tabs"
          :key="t"
          class="tab"
          :class="{ active: active === t }"
          type="button"
          @click="active = t"
        >
          {{ t }}
        </button>
      </div>

      <div class="lib-grid">
        <div v-for="n in 8" :key="n" class="pal-card">
          <div class="top">
            <div>
              <div class="name">Palette {{ n }}</div>
              <div class="meta">5 colors</div>
            </div>
            <span class="tag">UI</span>
          </div>
          <div class="strip">
            <span :style="{ background: '#6D5AFE' }" />
            <span :style="{ background: '#FFA94D' }" />
            <span :style="{ background: '#FFD43B' }" />
            <span :style="{ background: '#51CF66' }" />
            <span :style="{ background: '#22D3EE' }" />
            <span :style="{ background: '#A78BFA' }" />
          </div>
        </div>
      </div>
    </CardFrame>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 14px;
}

.col-span-full {
  grid-column: 1 / -1;
}

.lib-tabs {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.tab {
  padding: 5px 11px;
  border-radius: 999px;
  font-size: 11.5px;
  color: var(--fg-2);
  border: 1px solid var(--line-soft);
  background: var(--bg-1);
  cursor: pointer;
  font-family: inherit;
}

.tab.active {
  background: var(--bg-3);
  color: var(--fg-0);
}

.lib-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 10px;
}

.pal-card {
  border: 1px solid var(--line-soft);
  border-radius: 11px;
  padding: 10px 10px 8px;
  background: var(--bg-2);
}

.pal-card .top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pal-card .name {
  font-size: 12px;
  font-weight: 600;
  color: var(--fg-0);
}

.pal-card .meta {
  font-size: 10.5px;
  color: var(--fg-3);
}

.pal-card .tag {
  font-size: 9.5px;
  color: var(--fg-2);
  border: 1px solid var(--line);
  padding: 1px 6px;
  border-radius: 999px;
}

.pal-card .strip {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  height: 36px;
  border-radius: 6px;
  overflow: hidden;
  margin-top: 8px;
}
</style>
