<script setup lang="ts">
// Empty card frame used as a placeholder until each feature card is
// implemented in later slices (S5 onward). Provides the visual chrome
// (border, padding, header, optional subtitle) so views can already
// communicate the final layout.

interface Props {
  title: string;
  sub?: string;
}

defineProps<Props>();
</script>

<template>
  <section class="card">
    <div class="card-h">
      <h3>{{ title }}</h3>
      <span v-if="sub" class="sub">{{ sub }}</span>
      <div class="right">
        <slot name="header-right" />
      </div>
    </div>
    <div class="card-body">
      <slot>
        <div class="empty mono">awaiting feature wiring</div>
      </slot>
    </div>
  </section>
</template>

<style scoped>
.card {
  background: var(--bg-1);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: 14px 14px 16px;
  box-shadow: var(--shadow-card);
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.card-h {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.card-h h3 {
  font-size: 12.5px;
  font-weight: 600;
  margin: 0;
  letter-spacing: -0.005em;
  color: var(--fg-0);
}

.card-h .sub {
  font-size: 11px;
  color: var(--fg-3);
}

.card-h .right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 6px;
}

.card-body {
  flex: 1 1 auto;
  min-height: 80px;
}

.empty {
  color: var(--fg-3);
  border: 1px dashed var(--line-soft);
  border-radius: var(--r-md);
  padding: 18px;
  text-align: center;
}
</style>
