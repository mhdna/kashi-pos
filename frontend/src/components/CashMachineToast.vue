<template>
  <v-snackbar v-model="visible" class="cash-machine-snackbar" :timeout="8000" location="top" variant="flat">
    <div v-if="result" class="cash-machine-toast" role="status" aria-live="polite">
      <v-icon :icon="result.failed ? 'mdi-alert-circle' : 'mdi-check-circle'" :color="result.failed ? 'warning' : 'success'" size="32" />
      <div>
        <div class="cash-machine-toast__title">
          {{ result.failed ? "Cash machine set up, with problems" : "This till is now a cash machine" }}
        </div>
        <div class="cash-machine-toast__detail">
          {{ result.done }} of {{ result.total }} steps done{{ result.failed ? " · see Settings → Cash machine" : "" }}
        </div>
      </div>
    </div>
  </v-snackbar>
</template>

<script setup lang="ts">
import { onUnmounted, ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { main } from "../../wailsjs/go/models";

// "Make this a cash machine" runs in the background for minutes
// (cash_machine.go), so its result is announced here, on whatever page is
// open when it finishes.

const visible = ref(false);
const result = ref<{ done: number; total: number; failed: number } | null>(null);

const off = EventsOn("cash_machine_setup_done", (setup: main.CashMachineSetup) => {
  const steps = setup?.steps ?? [];
  const done = steps.filter((step) => step.ok).length;
  result.value = { done, total: steps.length, failed: steps.length - done };
  visible.value = true;
});
onUnmounted(off);
</script>

<style scoped>
.cash-machine-toast {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 310px;
}
.cash-machine-toast__title {
  font-size: 1rem;
  font-weight: 600;
}
.cash-machine-toast__detail {
  font-size: 0.8rem;
  opacity: 0.75;
}
</style>

<style>
/* Solid like WhatsAppToast; unscoped because the snackbar is teleported. */
.cash-machine-snackbar .v-snackbar__wrapper {
  background: rgb(var(--v-theme-surface)) !important;
  color: rgb(var(--v-theme-on-surface)) !important;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.12);
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.25) !important;
}
</style>
