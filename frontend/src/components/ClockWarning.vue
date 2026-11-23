<template>
  <!-- The PC's clock or zone disagrees with central's (clock.go). -->
  <k-snackbar v-model="show" :timeout="-1" location="top" color="warning" multi-line>
    <div class="d-flex align-start">
      <k-icon icon="mdi-clock-alert-outline" class="me-3 mt-1" />
      <div>
        <div v-if="status.clockOff">
          {{ t("This computer's clock is {drift} {direction} central's, so sales and shifts get the wrong time.", { drift: driftText, direction: status.driftSeconds > 0 ? t("ahead of") : t("behind") }) }}
        </div>
        <div v-if="status.zoneMismatch">
          {{ t("This computer's time zone ({pcZone}) isn't the branch's ({zone}).", { pcZone: status.pcZone, zone: status.zone }) }}
        </div>
        <div class="text-caption mt-1">
          {{ t('Fix it in Windows: Settings › Time & language › Date & time — turn on "Set time automatically" and "Set time zone automatically", or pick the right time zone.') }}
        </div>
      </div>
    </div>
    <template #actions>
      <k-btn variant="text" @click="dismissed = true">{{ t('Later') }}</k-btn>
    </template>
  </k-snackbar>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { t } from "@/i18n";
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
  if (minutes < 60) return `${minutes} ${minutes === 1 ? t("minute") : t("minutes")}`;
  const hours = Math.round(minutes / 6) / 10;
  return `${hours} ${hours === 1 ? t("hour") : t("hours")}`;
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
