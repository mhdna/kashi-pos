<template>
  <v-app :class="{ 'kashi-compact': isCompactBuild, 'animations-disabled': !animationsEnabled }">
    <div v-if="startupLoading" class="startup-screen" role="status" aria-label="Loading">
      <div class="startup-loader-content">
        <div class="startup-spinner" aria-hidden="true"></div>
        <span>Loading…</span>
      </div>
    </div>
    <div v-else-if="databaseMissing" class="database-setup">
      <v-card class="database-setup-card" max-width="520">
        <v-card-title>Choose a database</v-card-title>
        <v-card-text>
          No Kashi database was found. Choose an existing database, or create a new one in a selected folder.
          <div v-if="startupError" class="text-caption text-medium-emphasis mt-3">{{ startupError }}</div>
        </v-card-text>
        <v-card-actions class="px-4 pb-4">
          <v-btn variant="outlined" :loading="databaseActionLoading" @click="chooseDatabase">
            Choose existing database
          </v-btn>
          <v-spacer />
          <v-btn color="primary" variant="flat" :loading="databaseActionLoading" @click="createDatabase">
            Create database
          </v-btn>
        </v-card-actions>
        <v-alert v-if="databaseActionError" type="error" density="compact" class="mx-4 mb-4">
          {{ databaseActionError }}
        </v-alert>
      </v-card>
    </div>
    <v-alert v-else-if="startupError" type="error" prominent class="ma-4">
      {{ startupError }}
    </v-alert>
    <router-view v-else />
  </v-app>
</template>

<script setup>
import { onMounted, ref } from "vue";
import { CreateDatabase, ChooseDatabase, WaitUntilReady } from "../wailsjs/go/main/App";
import { useTheme } from "vuetify";
import { animationsEnabled, isCompactBuild } from "@/lib/display";
import { EventsOn } from "../wailsjs/runtime/runtime";

const theme = useTheme();
const startupLoading = ref(true);
const startupError = ref("");
const databaseMissing = ref(false);
const databaseActionLoading = ref(false);
const databaseActionError = ref("");

async function reloadAfterDatabaseChange(action) {
  databaseActionLoading.value = true;
  databaseActionError.value = "";
  try {
    const path = await action();
    if (path) window.location.reload();
  } catch (error) {
    databaseActionError.value = error instanceof Error ? error.message : String(error);
  } finally {
    databaseActionLoading.value = false;
  }
}

function chooseDatabase() {
  return reloadAfterDatabaseChange(ChooseDatabase);
}

function createDatabase() {
  return reloadAfterDatabaseChange(CreateDatabase);
}

onMounted(async () => {
	EventsOn("admin:reload", () => window.location.reload());
	try {
    const result = await WaitUntilReady();
    startupError.value = result.error;
    databaseMissing.value = Boolean(result.databaseMissing);
    if (startupError.value) return;

    theme.change(result.theme === "dark" ? "dark" : "light");
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

.database-setup {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(var(--v-theme-background));
}

.database-setup-card {
  width: 100%;
}

@keyframes startup-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
