<template>
  <!-- The PC's clock or zone disagrees with kashi's (clock.go). -->
  <v-snackbar v-model="show" :timeout="-1" location="top" color="warning" multi-line>
    <div class="d-flex align-start">
      <v-icon icon="mdi-clock-alert-outline" class="me-3 mt-1" />
      <div>
        <div v-if="status.clockOff">
          This computer's clock is {{ driftText }} {{ status.driftSeconds > 0 ? "ahead of" : "behind" }} kashi's,
          so sales and shifts get the wrong time.
        </div>
        <div v-if="status.zoneMismatch">
          This computer's time zone ({{ status.pcZone }}) isn't the branch's ({{ status.zone }}).
        </div>
        <div class="text-caption mt-1">
          Fix it in Windows: Settings › Time &amp; language › Date &amp; time — turn on "Set time automatically"
          and "Set time zone automatically", or pick the right time zone.
        </div>
      </div>
    </div>
    <template #actions>
      <v-btn variant="text" @click="dismissed = true">Later</v-btn>
    </template>
  </v-snackbar>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { GetClockStatus } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";

const status = ref<main.ClockStatus>({} as main.ClockStatus);
const dismissed = ref(false);
const show = computed({
  get: () => !dismissed.value && (status.value.clockOff || status.value.zoneMismatch),
  set: (v) => { if (!v) dismissed.value = true; },
});

const driftText = computed(() => {
  const minutes = Math.round(Math.abs(status.value.driftSeconds) / 60);
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? "" : "s"}`;
  const hours = Math.round(minutes / 6) / 10;
  return `${hours} hour${hours === 1 ? "" : "s"}`;
});

let off: (() => void) | undefined;

onMounted(async () => {
  off = EventsOn("clock_status_changed", (s: main.ClockStatus) => {
    // A new problem brings a dismissed warning back.
    if (s.clockOff !== status.value.clockOff || s.zoneMismatch !== status.value.zoneMismatch) dismissed.value = false;
    status.value = s;
  });
  status.value = await GetClockStatus();
});

onUnmounted(() => off?.());
</script>
