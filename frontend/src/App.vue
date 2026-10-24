<template>
  <v-app :class="{ 'kashi-compact': isCompactBuild, 'animations-disabled': !animationsEnabled }">
    <!-- While starting up, index.html's #startup-loader covers the window;
         it is removed below once startup is done. -->
    <template v-if="startupLoading" />
    <div v-else-if="databaseMissing" class="database-setup">
      <v-card v-if="knownBranches.length === 0 || showAddBranchForm" class="database-setup-card" max-width="480">
        <v-card-title>Set up this branch</v-card-title>
        <v-card-text>
          No branch has been set up on this machine yet. Give it a name and code (e.g. "Main", CN)
          and, if you have one, its central API key.
          <div v-if="startupError" class="text-caption text-medium-emphasis mt-3">{{ startupError }}</div>
        </v-card-text>
        <v-card-text>
          <v-form @submit.prevent="submitCreateBranch">
            <v-text-field v-model="branchForm.name" label="Branch name" autofocus required />
            <v-text-field
              v-model="branchForm.code"
              label="Branch code"
              hint="Letters and numbers only, e.g. CN"
              persistent-hint
              required
            />
            <v-text-field v-model="branchForm.apiKey" label="Central API key" class="mt-3" />
            <v-text-field v-model="branchForm.centralApiUrl" label="Central server URL" class="mt-3" />
            <v-alert v-if="branchActionError" type="error" density="compact" class="mt-3">
              {{ branchActionError }}
            </v-alert>
            <v-card-actions class="px-0 pt-4">
              <v-btn v-if="knownBranches.length > 0" variant="text" @click="showAddBranchForm = false">
                Back
              </v-btn>
              <v-spacer />
              <v-btn type="submit" color="primary" variant="flat" :loading="branchActionLoading">
                Create branch
              </v-btn>
            </v-card-actions>
          </v-form>
        </v-card-text>
      </v-card>

      <v-card v-else class="database-setup-card" max-width="480">
        <v-card-title>Choose a branch</v-card-title>
        <v-card-text>
          This machine has more than one branch set up, but none is currently open. Pick one to
          continue.
          <div v-if="startupError" class="text-caption text-medium-emphasis mt-3">{{ startupError }}</div>
        </v-card-text>
        <v-list>
          <v-list-item
            v-for="branch in knownBranches"
            :key="branch.code"
            :title="branch.name"
            :subtitle="branch.code"
            @click="submitSwitchBranch(branch.code)"
          />
        </v-list>
        <v-alert v-if="branchActionError" type="error" density="compact" class="mx-4 mb-4">
          {{ branchActionError }}
        </v-alert>
        <v-card-actions class="px-4 pb-4">
          <v-btn variant="text" :loading="branchActionLoading" @click="showAddBranchForm = true">
            Set up another branch instead
          </v-btn>
        </v-card-actions>
      </v-card>
    </div>
    <v-alert v-else-if="startupError" type="error" prominent class="ma-4">
      {{ startupError }}
    </v-alert>
    <router-view v-else />
    <AttendanceToast />
    <WhatsAppToast />
    <AppUpdateBanner />
    <ClockWarning />
  </v-app>
</template>

<script setup>
import { onMounted, reactive, ref } from "vue";
import { CreateBranch, ListBranches, SwitchBranch, WaitUntilReady } from "../wailsjs/go/main/App";
import { useTheme } from "vuetify";
import { animationsEnabled, isCompactBuild, startDisplayPrefsSync } from "@/lib/display";
import { EventsOn, WindowReloadApp } from "../wailsjs/runtime/runtime";
import AttendanceToast from "@/components/AttendanceToast.vue";
import WhatsAppToast from "@/components/WhatsAppToast.vue";
import AppUpdateBanner from "@/components/AppUpdateBanner.vue";
import ClockWarning from "@/components/ClockWarning.vue";

const theme = useTheme();
const startupLoading = ref(true);
const startupError = ref("");
const databaseMissing = ref(false);
const knownBranches = ref([]);
const showAddBranchForm = ref(false);
const branchActionLoading = ref(false);
const branchActionError = ref("");
const branchForm = reactive({ name: "", code: "", apiKey: "", centralApiUrl: "" });

async function submitCreateBranch() {
  branchActionLoading.value = true;
  branchActionError.value = "";
  try {
    await CreateBranch(branchForm.name, branchForm.code, branchForm.apiKey, branchForm.centralApiUrl);
    WindowReloadApp();
  } catch (error) {
    branchActionError.value = error instanceof Error ? error.message : String(error);
  } finally {
    branchActionLoading.value = false;
  }
}

async function submitSwitchBranch(code) {
  branchActionLoading.value = true;
  branchActionError.value = "";
  try {
    await SwitchBranch(code);
    WindowReloadApp();
  } catch (error) {
    branchActionError.value = error instanceof Error ? error.message : String(error);
  } finally {
    branchActionLoading.value = false;
  }
}

onMounted(async () => {
	EventsOn("admin:reload", () => window.location.reload());
	try {
    const result = await WaitUntilReady();
    startupError.value = result.error;
    databaseMissing.value = Boolean(result.databaseMissing);
    if (databaseMissing.value) {
      try {
        const branches = await ListBranches();
        knownBranches.value = Array.isArray(branches?.branches) ? branches.branches : [];
      } catch {
        knownBranches.value = [];
      }
    }
    if (startupError.value && !databaseMissing.value) return;

    theme.change(result.theme === "dark" ? "dark" : "light");
    void startDisplayPrefsSync((next) => theme.change(next));
  } catch (error) {
    startupError.value = error instanceof Error ? error.message : String(error);
  } finally {
    startupLoading.value = false;
    hideStartupLoader();
  }
});

// Fades out and removes the loader index.html shows from launch.
function hideStartupLoader() {
  const loader = document.getElementById("startup-loader");
  if (!loader) return;
  loader.classList.add("startup-loader-done");
  setTimeout(() => loader.remove(), 200);
}
</script>

<style scoped>
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
</style>
