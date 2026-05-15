<script setup lang="ts">
import CardFrame from '../components/CardFrame.vue';
import ContrastChecker from '../components/ContrastChecker.vue';
</script>

<template>
  <div class="grid">
    <CardFrame title="Contrast checker" sub="WCAG 2.1 · APCA" class="span-full">
      <ContrastChecker />
    </CardFrame>

    <CardFrame title="Pairings" sub="palette × palette grid" class="span-full" />
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: 1fr;
  grid-auto-rows: min-content;
  gap: 14px;
}

.span-full {
  grid-column: 1 / -1;
}
</style>
