<template>
  <v-app>
    <AppBar />
    <NavDrawer />
    <v-main>
      <v-container class="pa-0 h-100" fluid style="overflow: hidden">
        <router-view />
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup>
import AppBar from "@/components/AppBar.vue";
import NavDrawer from "@/components/NavDrawer.vue";
</script>
