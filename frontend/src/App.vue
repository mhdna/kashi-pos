<template>
  <v-app :class="{ 'kashi-compact': isCompactBuild, 'animations-disabled': !animationsEnabled }">
    <v-main>
      <v-alert v-if="startupError" type="error" prominent class="ma-4">
        {{ startupError }}
      </v-alert>
      <router-view />
    </v-main>
  </v-app>
</template>

<script setup>
import { onMounted, ref } from "vue";
import { GetStartupError, GetTheme } from "../wailsjs/go/main/App";
import { useTheme } from "vuetify";
import { animationsEnabled, isCompactBuild } from "@/lib/display";

const theme = useTheme();
const startupError = ref("");

onMounted(async () => {
  try {
    startupError.value = await GetStartupError();
    if (startupError.value) return;

    const saved = await GetTheme();
    if (saved && saved !== "system") theme.change(saved);
  } catch (error) {
    startupError.value = error instanceof Error ? error.message : String(error);
  }
});
</script>
