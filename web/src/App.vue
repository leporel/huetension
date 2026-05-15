<script setup lang="ts">
import TopBar from './components/TopBar.vue';
import SideBar from './components/SideBar.vue';
</script>

<template>
  <div class="app-shell">
    <TopBar />
    <div class="shell">
      <SideBar />
      <main class="main">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.shell {
  display: grid;
  grid-template-columns: 232px 1fr;
  flex: 1;
  min-height: 0;
}

.main {
  padding: 16px;
  min-width: 0;
  min-height: 0;
}
</style>
