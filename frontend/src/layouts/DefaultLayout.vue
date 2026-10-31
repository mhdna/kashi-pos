<template>
  <NavDrawer />
  <k-main class="app-main">
    <router-view />
  </k-main>

</template>

<script setup lang="ts">
import NavDrawer from "@/components/NavDrawer.vue";
</script>

<style>
html,
body {
  overflow: hidden;
}
.app-main {
  height: 100vh;
}
.app-main :deep(.k-main__wrap) {
  height: 100%;
  overflow-y: auto;
}
</style>
