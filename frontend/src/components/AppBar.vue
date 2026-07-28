<template>
  <v-app-bar color="#4a352f" density="compact" elevation="4">
    <template #prepend>
      <div class="d-flex flex-column align-last justify-center mx-2">
        <v-app-bar-title class="text-title-medium">EV POS TERMINAL</v-app-bar-title>
        <div class="text-title-small">{{ branchName || "Branch" }}</div>
      </div>
      <div class="mx-8 me-0 text-title-small">{{ time }}</div>
      <v-tooltip v-if="attendanceUnreachable" text="Attendance machine is unreachable">
        <template #activator="{ props }"><v-badge v-bind="props" color="error" dot inline class="ms-3"><v-icon icon="mdi-lan-disconnect" color="error" size="20" /></v-badge></template>
      </v-tooltip>
      <!-- <div class="mx-8 text-title-small text-green pa-1 px-3"> -->
      <!--   <v-icon icon="mdi-cloud-check-variant" size="18" class="me-1" /> -->
      <!--   Synced -->
      <!-- </div> -->
    </template>
    <template #append>
      <ToggleTheme />
      <!-- <new-transfer-alert /> -->
      <salespersons-dialog />
    </template>
  </v-app-bar>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { GetAppSettings } from "../../wailsjs/go/main/App";
import { SyncAttendance } from "../../wailsjs/go/main/App";
import ToggleTheme from "./ToggleTheme.vue";
import NewTransferAlert from "./NewTransferAlert.vue";
import SalespersonsDialog from "./SalespersonsDialog.vue";

const time = ref("");
const branchName = ref("");
const attendanceUnreachable = ref(false);

function updateTime() {
  const now = new Date();
  const pad = (n: number) => n.toString().padStart(2, "0");
  time.value = `${pad(now.getDate())}-${pad(now.getMonth() + 1)}-${now.getFullYear()} ${pad(now.getHours())}:${pad(now.getMinutes())}`;
}

let interval: ReturnType<typeof setInterval>;
onMounted(async () => {
  updateTime();
  interval = setInterval(updateTime, 1000);
  try {
    const settings = await GetAppSettings();
    branchName.value = settings?.branchName ?? "";
  } catch {}
  const checkAttendance = async () => {
    try {
      const config = await GetAppSettings();
      if (!config?.attendance?.enabled) { attendanceUnreachable.value = false; return; }
      const result = await SyncAttendance(new Date().toISOString().slice(0, 10));
      attendanceUnreachable.value = result?.configured === true && result?.reachable !== true;
    } catch { attendanceUnreachable.value = true; }
  };
  await checkAttendance();
  clearInterval(interval);
  interval = setInterval(async () => { updateTime(); await checkAttendance(); }, 3000);
});
onUnmounted(() => clearInterval(interval));
</script>
