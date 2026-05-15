<script setup lang="ts">
import { RouterLink } from 'vue-router';
import ThemeToggle from './ThemeToggle.vue';

interface TabDef {
  to: string;
  label: string;
}

const tabs: TabDef[] = [
  { to: '/', label: 'Generate' },
  { to: '/library', label: 'Library' },
  { to: '/tools', label: 'Tools' },
  { to: '/contrast', label: 'Contrast' },
];
</script>

<template>
  <header class="topbar">
    <div class="brand">
      <span class="brand-mark" aria-hidden="true" />
      <span class="brand-name">huetension</span>
      <sup>web</sup>
    </div>

    <nav class="nav" aria-label="Primary">
      <RouterLink
        v-for="t in tabs"
        :key="t.to"
        :to="t.to"
        class="nav-link"
        active-class="active"
        :exact-active-class="t.to === '/' ? 'active' : ''"
      >
        <span class="dot" aria-hidden="true" />
        {{ t.label }}
      </RouterLink>
    </nav>

    <div class="right">
      <ThemeToggle />
    </div>
  </header>
</template>

<style scoped>
.topbar {
  height: 52px;
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 0 18px;
  border-bottom: 1px solid var(--line-soft);
  background: var(--topbar-bg);
  backdrop-filter: blur(8px);
  position: sticky;
  top: 0;
  z-index: 5;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 600;
  letter-spacing: -0.02em;
  font-size: 14.5px;
}

.brand-mark {
  width: 22px;
  height: 22px;
  border-radius: 7px;
  background: conic-gradient(
    from 220deg,
    oklch(0.66 0.20 280),
    oklch(0.70 0.18 320),
    oklch(0.74 0.16 35),
    oklch(0.78 0.14 85),
    oklch(0.62 0.13 200),
    oklch(0.66 0.20 280)
  );
  box-shadow:
    0 0 0 1px oklch(1 0 0 / 0.08),
    0 4px 14px -4px oklch(0.66 0.20 280 / 0.6);
}

.brand sup {
  color: var(--fg-3);
  font-weight: 500;
  font-size: 9.5px;
  margin-left: 4px;
  padding: 2px 5px;
  border: 1px solid var(--line);
  border-radius: 4px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.nav {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px;
  background: var(--bg-1);
  border: 1px solid var(--line-soft);
  border-radius: 10px;
}

.nav-link {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  border-radius: 7px;
  color: var(--fg-1);
  font-size: 12.5px;
  font-weight: 500;
  text-decoration: none;
}

.nav-link:hover {
  color: var(--fg-0);
}

.nav-link .dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--accent);
  opacity: 0;
}

.nav-link.active {
  background: var(--bg-3);
  color: var(--fg-0);
  box-shadow: 0 1px 0 oklch(1 0 0 / 0.04) inset;
}

.nav-link.active .dot {
  opacity: 1;
}

.right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
