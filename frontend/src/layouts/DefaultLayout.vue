<template>
  <AppBar />
  <NavDrawer />
  <v-main class="app-main">
    <router-view />
  </v-main>
</template>

<script setup>
import AppBar from "@/components/AppBar.vue";
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
.app-main :deep(.v-main__wrap) {
  height: 100%;
  overflow-y: auto;
}
</style>
