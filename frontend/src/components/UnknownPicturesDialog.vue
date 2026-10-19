<template>
  <v-dialog :model-value="modelValue" max-width="720" persistent @update:model-value="emit('update:modelValue', $event)">
    <v-card class="pa-4">
      <v-card-title class="px-0 pt-0 text-h5">Pictures of the unknown items</v-card-title>

      <template v-if="state === 'starting'">
        <div class="text-center py-6">
          <v-progress-circular color="primary" indeterminate size="40" />
          <div class="text-body-2 mt-3">Opening a picture session…</div>
        </div>
      </template>

      <template v-else-if="state === 'error'">
        <div class="text-body-2 py-3">{{ errorText }}</div>
        <v-btn color="primary" variant="tonal" @click="start">Try again</v-btn>
      </template>

      <template v-else>
        <div class="d-flex ga-6 align-start">
          <div class="text-center" style="flex: none; width: 260px">
            <img
              v-if="qrDataUrl"
              alt="Upload QR code"
              :src="qrDataUrl"
              style="width: 260px; height: 260px"
            />
            <div class="text-h6 mt-3">Scan, then photograph each item.</div>
            <div class="text-h6 font-weight-regular text-medium-emphasis" dir="rtl" lang="ar">
              امسح الرمز والتقط صورة لكل قطعة غير معروف كودها طبقا للسعر
            </div>
          </div>

          <div class="flex-grow-1" style="min-width: 0">
            <v-list class="text-body-1">
              <v-list-item v-for="(slot, index) in slots" :key="slot.slotRef" :title="`#${index + 1} · ${slot.label}`">
                <template #append>
                  <v-icon v-if="slot.hasImage" color="success" icon="mdi-check-circle" />
                  <span v-else-if="slot.waived" class="text-caption text-success">allowed by a manager</span>
                  <v-icon v-else color="grey" icon="mdi-timer-sand" />
                </template>
              </v-list-item>
            </v-list>
            <div class="text-body-1 text-medium-emphasis">{{ doneCount }} of {{ slots.length }} ready</div>
          </div>
        </div>
      </template>

      <v-card-actions class="px-0 pb-0 mt-3">
        <v-btn v-if="state === 'ready'" :disabled="helpAsked" size="large" prepend-icon="mdi-bell-ring-outline" variant="tonal" @click="askForHelp">
          {{ helpAsked ? "Manager asked" : "Ask a manager" }}
        </v-btn>
        <v-spacer />
        <v-btn size="large" variant="text" @click="cancel">Cancel</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import QRCode from "qrcode";
import {
  RequestUnknownItemHelp,
  StartUnknownItemSession,
  UnknownItemSessionStatus,
} from "../../wailsjs/go/main/App";

// Held between "pay" and the payment itself when the cart has unknown items:
// opens a picture session on kashi, shows the phone's QR code, and closes
// the moment every unit has a picture -- or a kashi manager lets it through.
// Polls fast (a second) so the cashier never waits longer than the phone does.

type Unit = { label: string; unitPriceCents: number; discount: number };
type Slot = { slotRef: string; label: string; hasImage: boolean; waived: boolean; done: boolean };

const props = defineProps<{ modelValue: boolean; units: Unit[] }>();
const emit = defineEmits<{
  "update:modelValue": [open: boolean];
  ready: [];
  cancel: [];
}>();

const POLL_MS = 1000;
const state = ref<"starting" | "ready" | "error">("starting");
const slots = ref<Slot[]>([]);
const qrDataUrl = ref("");
const errorText = ref("");
const helpAsked = ref(false);
let clientRef = "";
let pollTimer: ReturnType<typeof setInterval> | null = null;
let session = 0;

const doneCount = computed(() => slots.value.filter((slot) => slot.done).length);

function stopPolling() {
  if (pollTimer !== null) clearInterval(pollTimer);
  pollTimer = null;
}

function finish() {
  stopPolling();
  session++;
  emit("ready");
  emit("update:modelValue", false);
}

async function poll(mine: number) {
  try {
    const status = await UnknownItemSessionStatus(clientRef);
    if (mine !== session) return;
    slots.value = status.slots ?? [];
    if (status.complete) finish();
  } catch {
    // A dropped poll is simply tried again a second later.
  }
}

async function start() {
  stopPolling();
  const mine = ++session;
  state.value = "starting";
  helpAsked.value = false;
  qrDataUrl.value = "";
  try {
    const info = await StartUnknownItemSession(JSON.stringify(props.units));
    if (mine !== session) return;
    clientRef = info.clientRef;
    slots.value = info.slots ?? [];
    qrDataUrl.value = await QRCode.toDataURL(info.uploadUrl, { width: 260, margin: 1 });
    if (mine !== session) return;
    state.value = "ready";
    if (info.complete) {
      finish();
      return;
    }
    pollTimer = setInterval(() => void poll(mine), POLL_MS);
  } catch (e) {
    if (mine !== session) return;
    errorText.value = e instanceof Error ? e.message : String(e);
    state.value = "error";
  }
}

async function askForHelp() {
  helpAsked.value = true;
  try {
    await RequestUnknownItemHelp(clientRef);
  } catch {
    helpAsked.value = false;
  }
}

function cancel() {
  stopPolling();
  session++;
  emit("cancel");
  emit("update:modelValue", false);
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) void start();
    else {
      stopPolling();
      session++;
    }
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  stopPolling();
  session++;
});
</script>
