<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import CardFrame from '../components/CardFrame.vue';
import PaletteLibrary from '../components/PaletteLibrary.vue';
import SaveToLibrary from '../components/SaveToLibrary.vue';

// `/library/:id` deep-links a palette; `/library` leaves it unselected.
const route = useRoute();
const selectedId = computed(() => {
  const id = route.params.id;
  return typeof id === 'string' && id.length > 0 ? id : null;
});
</script>

<template>
  <div class="grid">
    <CardFrame title="Save to library" sub="add the current palette" class="span-full">
      <SaveToLibrary />
    </CardFrame>
    <CardFrame title="Palette library" sub="curated catalogue" class="span-full">
      <PaletteLibrary :selected-id="selectedId" />
    </CardFrame>
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 14px;
}

.span-full {
  grid-column: 1 / -1;
}
</style>
