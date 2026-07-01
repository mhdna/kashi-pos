<template>
  <v-btn icon variant="text" @click="toggleTheme" :title="currentTheme">
    <v-icon
      :icon="
        currentTheme === 'dark' ? 'mdi-weather-sunny' : 'mdi-weather-night'
      "
    />
  </v-btn>
</template>

<script setup lang="ts">
import { useTheme } from "vuetify";
import { SetTheme } from "@/wailsjs/go/main/App";

const theme = useTheme();

const currentTheme = theme.global.name;

function toggleTheme() {
  const next = theme.global.name.value === "dark" ? "light" : "dark";
  theme.global.name.value = next;
  SetTheme(next);
}
</script>
