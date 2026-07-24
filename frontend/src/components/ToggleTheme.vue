<template>
  <v-btn icon variant="text" @click="toggleTheme" :title="currentTheme">
    <v-icon :icon="currentTheme === 'dark' ? 'mdi-weather-sunny' : 'mdi-weather-night'" />
  </v-btn>
</template>

<script setup lang="ts">
import { useTheme } from "vuetify";
import { SetTheme } from "../../wailsjs/go/main/App";

const theme = useTheme();

const currentTheme = theme.name;

function toggleTheme() {
  const next = theme.name.value === "dark" ? "light" : "dark";
  theme.change(next);
  SetTheme(next);
}
</script>
