<script setup lang="ts">
import CardFrame from '../components/CardFrame.vue';
</script>

<template>
  <div class="grid">
    <CardFrame title="Contrast checker" sub="WCAG 2.1 · APCA" class="span-full">
      <div class="ctr-grid">
        <div class="ctr-controls">
          <label class="fld">Foreground</label>
          <div class="input row">
            <span class="swatch" :style="{ background: '#1A1A1A' }" />
            <span class="mono">#1A1A1A</span>
          </div>
          <label class="fld">Background</label>
          <div class="input row">
            <span class="swatch" :style="{ background: '#FFFFFF', borderColor: 'var(--line)' }" />
            <span class="mono">#FFFFFF</span>
          </div>
          <div class="empty mono">algorithm + sliders land in S6c</div>
        </div>
        <div class="ctr-preview">
          <div class="line"><span>WCAG 2.1</span><span class="ok">passes AAA</span></div>
          <div class="score">21.0</div>
          <div class="sub">Body text · Large text · UI components</div>
          <div class="line"><span>APCA</span><span class="ok">Lc ≈ 106</span></div>
        </div>
      </div>
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

.ctr-grid {
  display: grid;
  grid-template-columns: 1fr 1.1fr;
  gap: 14px;
  align-items: stretch;
}

.ctr-controls {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.fld {
  display: block;
  font-size: 11px;
  color: var(--fg-2);
  margin-bottom: 4px;
  font-weight: 500;
}

.input {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  background: var(--bg-2);
  border: 1px solid var(--line-soft);
  border-radius: 8px;
  color: var(--fg-0);
}

.input.row {
  justify-content: flex-start;
}

.input .swatch {
  width: 14px;
  height: 14px;
  border-radius: 4px;
  border: 1px solid oklch(1 0 0 / 0.1);
}

.empty {
  color: var(--fg-3);
  border: 1px dashed var(--line-soft);
  border-radius: var(--r-md);
  padding: 12px;
  text-align: center;
  font-size: 10.5px;
  margin-top: 4px;
}

.ctr-preview {
  border-radius: 12px;
  padding: 16px;
  background: #fff;
  color: #1a1a1a;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-height: 180px;
  position: relative;
  overflow: hidden;
}

.ctr-preview .score {
  font-family: 'Inter Tight', sans-serif;
  font-weight: 600;
  font-size: 38px;
  letter-spacing: -0.02em;
  text-align: center;
}

.ctr-preview .sub {
  text-align: center;
  font-size: 11.5px;
  color: oklch(0.4 0 0);
}

.ctr-preview .line {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  font-weight: 600;
}

.ok {
  color: oklch(0.55 0.14 155);
}
</style>
