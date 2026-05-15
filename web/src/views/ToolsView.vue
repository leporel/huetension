<script setup lang="ts">
import CardFrame from '../components/CardFrame.vue';
import ColorBlindnessSim from '../components/ColorBlindnessSim.vue';
import GradientStudio from '../components/GradientStudio.vue';
</script>

<template>
  <div class="grid">
    <CardFrame title="Color converter" sub="hex · rgb · hsl · oklch" />
    <CardFrame title="Gradient" sub="multi-stop · interpolate" class="col-span-2">
      <GradientStudio />
    </CardFrame>
    <CardFrame title="Color blindness" sub="protan · deutan · tritan · achroma" class="col-span-2">
      <ColorBlindnessSim />
    </CardFrame>
    <CardFrame title="Color picker" sub="HSV wheel + hue bar" />
    <CardFrame title="Export" sub="CSS · SCSS · Less · Tailwind" />
  </div>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  grid-auto-rows: min-content;
  gap: 14px;
}

.col-span-2 {
  grid-column: span 2;
}

@media (max-width: 1100px) {
  .grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .col-span-2 {
    grid-column: span 2;
  }
}
</style>
