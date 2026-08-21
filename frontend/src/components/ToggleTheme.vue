<template>
  <v-btn
    icon
    variant="text"
    :loading="saving"
    :title="`Switch to ${currentTheme === 'dark' ? 'light' : 'dark'} theme`"
    @click="toggleTheme"
  >
    <v-icon :icon="currentTheme === 'dark' ? 'mdi-weather-sunny' : 'mdi-weather-night'" />
  </v-btn>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useTheme } from "vuetify";
import { SetTheme } from "../../wailsjs/go/main/App";

const theme = useTheme();

const currentTheme = theme.name;
const saving = ref(false);

async function toggleTheme() {
  if (saving.value) return;
  const previous = theme.name.value;
  const next = theme.name.value === "dark" ? "light" : "dark";
  theme.change(next);
  saving.value = true;
  try {
    await SetTheme(next);
  } catch {
    theme.change(previous);
  } finally {
    saving.value = false;
  }
}
</script>
