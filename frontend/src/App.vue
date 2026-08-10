<template>
  <v-app :class="{ 'kashi-compact': isCompactBuild, 'animations-disabled': !animationsEnabled }">
    <div v-if="startupLoading" class="startup-screen" role="status" aria-label="Loading">
      <div class="startup-loader-content">
        <div class="startup-spinner" aria-hidden="true"></div>
        <span>Loading…</span>
      </div>
    </div>
    <v-alert v-else-if="startupError" type="error" prominent class="ma-4">
      {{ startupError }}
    </v-alert>
    <router-view v-else />
  </v-app>
</template>

<script setup>
import { onMounted, ref } from "vue";
import { GetStartupError } from "../wailsjs/go/main/App";
import { useTheme } from "vuetify";
import { animationsEnabled, isCompactBuild } from "@/lib/display";
import { EventsOn } from "../wailsjs/runtime/runtime";

const theme = useTheme();
const startupLoading = ref(true);
const startupError = ref("");

onMounted(async () => {
	EventsOn("admin:reload", () => window.location.reload());
	try {
    startupError.value = await GetStartupError();
    if (startupError.value) return;

    theme.change("light");
  } catch (error) {
    startupError.value = error instanceof Error ? error.message : String(error);
  } finally {
    startupLoading.value = false;
  }
});
</script>

<style scoped>
.startup-screen {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background: rgb(var(--v-theme-background));
  color: rgb(var(--v-theme-primary));
}

.startup-loader-content {
  display: grid;
  justify-items: center;
  gap: 16px;
}

.startup-spinner {
  width: 36px;
  height: 36px;
  border: 4px solid rgba(var(--v-theme-primary), 0.2);
  border-top-color: currentColor;
  border-radius: 50%;
  animation: startup-spin 0.8s linear infinite !important;
}

@keyframes startup-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
