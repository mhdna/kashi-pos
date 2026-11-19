<template>
  <k-app :class="{ 'kashi-compact': isCompactBuild, 'animations-disabled': !animationsEnabled }">
    <!-- While starting up, index.html's #startup-loader covers the window;
         it is removed below once startup is done. -->
    <template v-if="startupLoading" />
    <LicenseActivate v-else-if="licenseView === 'activate'" />
    <LicenseLock v-else-if="licenseView === 'locked'" :code="license.code" />
    <BranchSetup v-else-if="databaseMissing" :known-branches="knownBranches" :startup-error="startupError" />
    <k-alert v-else-if="startupError" type="error" prominent class="ma-4">
      {{ startupError }}
    </k-alert>
    <router-view v-else />
    <AttendanceToast />
    <WhatsAppToast />
    <CashMachineToast />
    <AppUpdateBanner />
    <ClockWarning />
    <LicenseWarning v-if="licenseView === 'ok'" :warn-hours="license.warnHours" />
  </k-app>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import { ListBranches, WaitUntilReady } from "../wailsjs/go/main/App";
import { useTheme } from "@/ui";
import { animationsEnabled, isCompactBuild, startDisplayPrefsSync } from "@/lib/display";
import { EventsOn } from "../wailsjs/runtime/runtime";
import AttendanceToast from "@/components/AttendanceToast.vue";
import WhatsAppToast from "@/components/WhatsAppToast.vue";
import CashMachineToast from "@/components/CashMachineToast.vue";
import AppUpdateBanner from "@/components/AppUpdateBanner.vue";
import ClockWarning from "@/components/ClockWarning.vue";
import BranchSetup from "@/components/BranchSetup.vue";
import LicenseActivate from "@/components/LicenseActivate.vue";
import LicenseLock from "@/components/LicenseLock.vue";
import LicenseWarning from "@/components/LicenseWarning.vue";
import { licenseScreen } from "@/lib/license";

const theme = useTheme();
const startupLoading = ref(true);
const startupError = ref("");
const databaseMissing = ref(false);
const knownBranches = ref([]);
// The license decides first: activation screen, lock screen, or the app.
const license = ref({});
const licenseView = computed(() => licenseScreen(license.value));
onMounted(async () => {
	EventsOn("admin:reload", () => window.location.reload());
	// Turned off (or back on) while running: the lock covers the app at once,
	// and lifting it reloads so every screen starts fresh.
	EventsOn("license:changed", (next) => {
    const wasLocked = licenseView.value !== "ok";
    license.value = next ?? {};
    if (wasLocked && licenseView.value === "ok") window.location.reload();
  });
	try {
    const result = await WaitUntilReady();
    license.value = result.license ?? {};
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
    if (licenseView.value === "activate") return;
    if (startupError.value && !databaseMissing.value && licenseView.value !== "locked") return;

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
