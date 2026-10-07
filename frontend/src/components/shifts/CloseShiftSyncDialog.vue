<template>
  <v-dialog v-model="open" max-width="460" persistent>
    <v-card>
      <v-card-title class="d-flex align-center ga-2 pt-4">
        <v-icon :icon="headerIcon" :color="headerColor" />
        {{ headerTitle }}
      </v-card-title>

      <v-card-text>
        <template v-if="state === 'uploading' || state === 'done'">
          <div class="text-body-2 mb-3">
            The shift closes and its receipt prints once everything has reached the office.
          </div>
          <v-progress-linear
            :model-value="percent"
            :indeterminate="state === 'uploading' && !progress"
            :color="state === 'done' ? 'success' : 'primary'"
            height="10"
            rounded
          />
          <div class="d-flex justify-space-between text-caption text-medium-emphasis mt-2">
            <span v-if="progress">{{ progress.done }} of {{ progress.total }} uploaded</span>
            <span v-else>Connecting…</span>
            <span v-if="state === 'uploading'">{{ etaText }}</span>
            <span v-else-if="progress">in {{ formatDuration(progress.elapsedSeconds) }}</span>
          </div>
        </template>

        <template v-else-if="state === 'error'">
          <v-alert type="error" variant="tonal" density="compact" class="mb-3">
            {{ errorMessage }}
          </v-alert>
          <div class="text-body-2">
            {{ remaining }} item{{ remaining === 1 ? "" : "s" }} still waiting to upload. Nothing
            is lost — once the internet is back, press <b>Try again</b>.
          </div>
        </template>

        <div v-if="breakdown.length && state !== 'done'" class="d-flex flex-wrap ga-1 mt-3">
          <v-chip v-for="row in breakdown" :key="row.entityType" size="small" variant="tonal">
            {{ entityLabel(row.entityType) }}: {{ row.count }}
          </v-chip>
        </div>

        <v-alert
          v-if="rejected > 0"
          type="warning"
          variant="tonal"
          density="compact"
          class="mt-3"
        >
          {{ rejected }} item{{ rejected === 1 ? " was" : "s were" }} rejected by the office and
          won't upload on retry. They don't block closing; check Settings › Central sync.
        </v-alert>

        <div v-if="showBypass" class="d-flex flex-column ga-3 mt-4">
          <v-text-field
            v-model="adminPassword"
            autofocus
            density="compact"
            hide-details="auto"
            label="Admin password"
            type="password"
            variant="outlined"
            @update:model-value="bypassError = ''"
          />
          <v-text-field
            v-model="bypassReason"
            density="compact"
            hide-details="auto"
            label="Reason (optional)"
            variant="outlined"
          />
          <div v-if="bypassError" class="text-caption text-error">{{ bypassError }}</div>
        </div>
      </v-card-text>

      <v-card-actions class="px-4 pb-4">
        <template v-if="showBypass">
          <v-btn variant="text" @click="showBypass = false">Back</v-btn>
          <v-spacer />
          <v-btn
            color="error"
            variant="elevated"
            :disabled="!adminPassword"
            :loading="bypassing"
            @click="submitBypass"
          >
            Close without uploading
          </v-btn>
        </template>
        <template v-else-if="state === 'error'">
          <v-btn variant="text" @click="finish(false)">Cancel</v-btn>
          <v-btn variant="text" size="small" @click="showBypass = true">Manager override</v-btn>
          <v-spacer />
          <v-btn color="primary" variant="elevated" prepend-icon="mdi-refresh" @click="upload">
            Try again
          </v-btn>
        </template>
        <template v-else-if="state === 'uploading'">
          <v-spacer />
          <v-btn variant="text" @click="cancelUpload">Cancel</v-btn>
        </template>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";
import {
  BypassCloseShiftSyncGate,
  CancelSyncFlush,
  FlushSyncOutbox,
  GetCloseShiftSyncGate,
} from "../../../wailsjs/go/main/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import type { main } from "../../../wailsjs/go/models";

// Gate in front of closing a shift when "upload everything before closing"
// is on: run(shiftId) resolves true once nothing is left to upload (or a
// manager overrides), false if the cashier backs out. When the mode is off,
// or nothing is pending, it resolves true without ever showing.

type State = "uploading" | "done" | "error";

const open = ref(false);
const state = ref<State>("uploading");
const progress = ref<main.SyncFlushProgress | null>(null);
const breakdown = ref<main.SyncPendingByType[]>([]);
const remaining = ref(0);
const rejected = ref(0);
const errorMessage = ref("");
const showBypass = ref(false);
const adminPassword = ref("");
const bypassReason = ref("");
const bypassError = ref("");
const bypassing = ref(false);

let shiftId = 0;
let failedAtStart = 0;
let resolveRun: ((proceed: boolean) => void) | null = null;
let cancelled = false;

const unsubscribe = EventsOn("sync_flush_progress", (p: main.SyncFlushProgress) => {
  if (!open.value) return;
  progress.value = p;
  breakdown.value = p.byType ?? [];
  remaining.value = p.remaining;
});
onBeforeUnmount(() => unsubscribe());

const percent = computed(() => {
  const p = progress.value;
  if (!p || p.total === 0) return state.value === "done" ? 100 : 0;
  return Math.round((p.done / p.total) * 100);
});

const etaText = computed(() => {
  const p = progress.value;
  if (!p || p.etaSeconds < 0) return "Estimating time left…";
  if (p.remaining === 0) return "Finishing…";
  return `~${formatDuration(p.etaSeconds)} left · ${p.perSecond.toFixed(p.perSecond < 10 ? 1 : 0)}/s`;
});

const headerTitle = computed(() =>
  state.value === "done"
    ? "Everything uploaded"
    : state.value === "error"
      ? "Couldn't finish uploading"
      : "Uploading to the office…",
);
const headerIcon = computed(() =>
  state.value === "done"
    ? "mdi-cloud-check-variant"
    : state.value === "error"
      ? "mdi-cloud-off-outline"
      : "mdi-cloud-upload",
);
const headerColor = computed(() =>
  state.value === "done" ? "success" : state.value === "error" ? "error" : "primary",
);

const entityLabels: Record<string, string> = {
  sales_invoice: "Sales",
  return_invoice: "Returns",
  invoice_settlement: "Account payments",
  shift_close: "Shift closes",
  expense: "Expenses",
  loan: "Loans",
  visitor_event: "Visitor counts",
  attendance_events: "Attendance",
  attendance_change: "Attendance edits",
  client: "Clients",
  transfer_request: "Transfers",
  transfer_confirm: "Transfer receipts",
  quick_access_mode: "Quick access",
};
function entityLabel(type: string) {
  return entityLabels[type] ?? type.replace(/_/g, " ");
}

function formatDuration(seconds: number) {
  const s = Math.max(0, Math.round(seconds));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

function errorText(e: unknown) {
  return e instanceof Error ? e.message : String(e);
}

async function run(id: number): Promise<boolean> {
  shiftId = id;
  const gate = await GetCloseShiftSyncGate();
  if (!gate.required) return true;
  failedAtStart = gate.failed;
  rejected.value = 0;
  if (gate.pending === 0) return true;

  breakdown.value = gate.byType ?? [];
  remaining.value = gate.pending;
  showBypass.value = false;
  open.value = true;
  const result = new Promise<boolean>((resolve) => (resolveRun = resolve));
  void upload();
  return result;
}

async function upload() {
  cancelled = false;
  state.value = "uploading";
  progress.value = null;
  errorMessage.value = "";
  try {
    const final = await FlushSyncOutbox();
    progress.value = final;
    rejected.value = Math.max(0, final.failed);
    state.value = "done";
    // Long enough to read "Everything uploaded", short enough not to stall.
    setTimeout(() => finish(true), rejected.value > 0 ? 2500 : 700);
  } catch (e) {
    if (cancelled) return;
    const gate = await GetCloseShiftSyncGate().catch(() => null);
    remaining.value = gate?.pending ?? remaining.value;
    breakdown.value = gate?.byType ?? breakdown.value;
    rejected.value = Math.max(0, (gate?.failed ?? failedAtStart) - failedAtStart);
    if (gate && gate.pending === 0) {
      state.value = "done";
      setTimeout(() => finish(true), 700);
      return;
    }
    errorMessage.value = errorText(e);
    state.value = "error";
  }
}

async function cancelUpload() {
  cancelled = true;
  await CancelSyncFlush();
  finish(false);
}

async function submitBypass() {
  bypassError.value = "";
  bypassing.value = true;
  try {
    await BypassCloseShiftSyncGate(shiftId, adminPassword.value, bypassReason.value);
    finish(true);
  } catch (e) {
    bypassError.value = errorText(e);
  } finally {
    bypassing.value = false;
  }
}

function finish(proceed: boolean) {
  if (!open.value) return;
  open.value = false;
  adminPassword.value = "";
  bypassReason.value = "";
  bypassError.value = "";
  resolveRun?.(proceed);
  resolveRun = null;
}

defineExpose({ run });
</script>
